import { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import { z } from 'zod';
import { config, getSessionCookieName, isAdminHost, isHomeHost, isOnboardingHost, isCustomDomainAdminHost } from '../config';
import { oidcClient } from '../oidc-client';
import { sessionStore, SessionData, WsTicketData, SessionTransferData } from '../session-store';
import { tenantServiceClient, maskEmail, maskIP } from '../tenant-service-client';
import { natsClient } from '../nats-client';
import { v4 as uuidv4 } from 'uuid';
import { createLogger } from '../logger';

const logger = createLogger('auth-routes');

/**
 * Validate returnTo URL to prevent open redirect attacks.
 * Allows relative paths and URLs on trusted domains only.
 */
function isValidReturnTo(url: string): boolean {
  // Relative paths are always safe
  if (url.startsWith('/')) return true;
  try {
    const parsed = new URL(url);
    // Only allow HTTPS (or HTTP for localhost in dev)
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return false;
    // Allow subdomains of the configured base domain
    return parsed.hostname === config.baseDomain || parsed.hostname.endsWith('.' + config.baseDomain);
  } catch {
    return false;
  }
}

function sanitizeReturnTo(url: string | undefined, fallback: string): string {
  if (!url) return fallback;
  return isValidReturnTo(url) ? url : fallback;
}

/**
 * Verify that the request includes a valid internal service key.
 * Used to protect /internal/* endpoints from external access.
 * When INTERNAL_SERVICE_KEY is not configured, requests pass through (with a warning log).
 * When configured, the X-Internal-Service-Key header must match exactly.
 */
function requireInternalServiceKey(request: FastifyRequest, reply: FastifyReply, done: () => void) {
  if (!config.internalServiceKey) {
    // No key configured — allow through but warn so operators know to set it
    logger.warn({ path: request.url }, 'INTERNAL_SERVICE_KEY not configured — internal endpoint unprotected');
    done();
    return;
  }
  const serviceKey = request.headers['x-internal-service-key'] as string | undefined;
  if (!serviceKey || serviceKey !== config.internalServiceKey) {
    logger.warn({ hasKey: !!serviceKey }, 'Unauthorized internal endpoint access attempt');
    reply.code(403).send({ error: 'forbidden', message: 'Invalid or missing internal service key' });
    return;
  }
  done();
}

// Request schemas
const loginQuerySchema = z.object({
  returnTo: z.string().optional(),
  // Standard OIDC prompt values + 'create' for Keycloak registration (converted to kc_action=register)
  prompt: z.enum(['none', 'login', 'consent', 'select_account', 'create']).optional(),
  loginHint: z.string().optional(),
  login_hint: z.string().optional(), // Alternative format for login hint
  kc_idp_hint: z.string().optional(), // Keycloak IDP hint - skips login page and goes directly to the IDP (e.g., 'google')
  kc_action: z.string().optional(), // Keycloak action - e.g., 'register' for direct registration
  // Tenant context - for multi-tenant storefront authentication
  tenant_id: z.string().optional(), // Tenant UUID
  tenant_slug: z.string().optional(), // Tenant slug (e.g., 'demo-store')
  // Registration info passed from storefront
  first_name: z.string().optional(),
  last_name: z.string().optional(),
});

const callbackQuerySchema = z.object({
  code: z.string().optional(), // Optional: missing when Keycloak returns an error response
  state: z.string(),
  error: z.string().optional(),
  error_description: z.string().optional(),
  // Keycloak-specific parameters
  iss: z.string().optional(),
  session_state: z.string().optional(),
});

const logoutQuerySchema = z.object({
  returnTo: z.string().optional(),
});

// Cookie helper with dynamic domain support for custom domains
/**
 * Determine the cookie domain based on the request host.
 * For tesserix.app domains, use .tesserix.app to enable cross-subdomain cookies.
 * For custom domains, extract the base domain (e.g., .yahvismartfarm.com) so
 * cookies work across both www and non-www variants.
 */
const getCookieDomain = (forwardedHost: string | undefined): string | undefined => {
  if (config.session.cookieDomain) {
    return config.session.cookieDomain;
  }
  if (!forwardedHost) {
    return undefined;
  }
  const hostname = forwardedHost.split(':')[0].toLowerCase();
  const baseDomain = config.baseDomain;
  if (hostname.endsWith(`.${baseDomain}`) || hostname === baseDomain) {
    return `.${baseDomain}`;
  }
  if (hostname === 'localhost' || hostname.endsWith('.localhost')) {
    return undefined;
  }
  // Custom domains: set cookie on the base domain so it works across
  // both www.example.com and example.com. Strip "www." prefix and prepend
  // with "." to enable subdomain sharing.
  const customBaseDomain = hostname.startsWith('www.') ? hostname.substring(4) : hostname;
  logger.debug({ hostname, cookieDomain: `.${customBaseDomain}` }, 'Custom domain detected, using base domain for cookie');
  return `.${customBaseDomain}`;
};

const setSessionCookie = (reply: FastifyReply, sessionId: string, forwardedHost?: string) => {
  const domain = getCookieDomain(forwardedHost);
  const cookieName = getSessionCookieName(forwardedHost);
  reply.setCookie(cookieName, sessionId, {
    httpOnly: true,
    secure: config.server.nodeEnv === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: config.session.maxAge,
    ...(domain ? { domain } : {}),
  });
};

const clearSessionCookie = (reply: FastifyReply, forwardedHost?: string) => {
  const domain = getCookieDomain(forwardedHost);
  const cookieName = getSessionCookieName(forwardedHost);
  reply.clearCookie(cookieName, {
    httpOnly: true,
    secure: config.server.nodeEnv === 'production',
    sameSite: 'lax',
    path: '/',
    ...(domain ? { domain } : {}),
  });
};

// Get session from request (uses hostname to determine which cookie to read)
const getSession = async (request: FastifyRequest): Promise<SessionData | null> => {
  // Use x-forwarded-host, handling comma-separated multi-proxy values, with fallback to request.hostname.
  // When called internally (e.g., from admin Next.js at auth-bff.marketplace.svc.cluster.local),
  // request.hostname resolves to the K8s service name — treat it as undefined so
  // getSessionCookieName() returns the default "bff_session" cookie name.
  const rawForwardedHost = request.headers['x-forwarded-host'] as string | undefined;
  const rawHostname = request.hostname;
  const isInternalHost = rawHostname.includes('.svc.cluster.local') || rawHostname.endsWith('.local');
  const forwardedHost = rawForwardedHost?.split(',')[0]?.trim() || (isInternalHost ? undefined : rawHostname);
  const cookieName = getSessionCookieName(forwardedHost);
  const sessionId = request.cookies[cookieName];

  // Fallback: accept session ID via X-Session-ID header for internal service-to-service calls.
  // In Istio mesh traffic, x-forwarded-host can be overwritten by the Envoy sidecar,
  // causing getSessionCookieName to return the wrong cookie name. The X-Session-ID header
  // bypasses this by passing the session value directly.
  const effectiveSessionId = sessionId || (request.headers['x-session-id'] as string | undefined);

  if (!effectiveSessionId) {
    logger.debug({ forwardedHost, cookieName, hasCookie: !!sessionId, hasSessionHeader: !!request.headers['x-session-id'] }, 'No session found');
    return null;
  }
  return sessionStore.getSession(effectiveSessionId);
};

export async function authRoutes(fastify: FastifyInstance) {
  // ============================================================================
  // GET /auth/login
  // Initiates the OIDC authorization flow
  // ============================================================================
  fastify.get<{
    Querystring: z.infer<typeof loginQuerySchema>;
  }>('/auth/login', async (request, reply) => {
    const query = loginQuerySchema.parse(request.query);

    // Determine client type from request (based on hostname or header)
    const clientType = determineClientType(request);

    // Get tenant context from headers (set by middleware) or query params
    // Priority: query params > headers > hostname extraction
    let tenantId = query.tenant_id || (request.headers['x-tenant-id'] as string | undefined);
    let tenantSlug = query.tenant_slug || (request.headers['x-tenant-slug'] as string | undefined);

    // Extract tenant slug from hostname if not provided
    // e.g., demo-store-admin.mark8ly.com → demo-store
    if (!tenantSlug) {
      const forwardedHost = (request.headers['x-forwarded-host'] as string || request.hostname || '').split(':')[0];
      const escapedDomain = config.baseDomain.replace(/\./g, '\\.');
      const adminMatch = forwardedHost.match(new RegExp(`^(.+)-admin\\.${escapedDomain}$`));
      const storefrontMatch = forwardedHost.match(new RegExp(`^(.+)\\.${escapedDomain}$`));
      if (adminMatch) {
        tenantSlug = adminMatch[1];
      } else if (storefrontMatch && !storefrontMatch[1].includes('devtest')) {
        tenantSlug = storefrontMatch[1];
      }
    }

    // Log tenant context for debugging
    if (tenantId || tenantSlug) {
      logger.info({ tenantId, tenantSlug }, 'Login initiated with tenant context');
    }

    // Generate PKCE values
    const state = oidcClient.generateState();
    const nonce = oidcClient.generateNonce();
    const codeVerifier = oidcClient.generateCodeVerifier();

    // Determine redirect URI based on request
    const redirectUri = getCallbackUrl(request, clientType);

    // Determine app type for role-based access control at callback time
    const fwdHostForAppType = request.headers['x-forwarded-host'] as string || request.hostname || '';
    const appType = isHomeHost(fwdHostForAppType)
      ? 'home' as const
      : isOnboardingHost(fwdHostForAppType)
        ? 'onboarding' as const
        : isAdminHost(fwdHostForAppType) || isCustomDomainAdminHost(fwdHostForAppType)
          ? 'admin' as const
          : 'storefront' as const;

    // Save auth flow state (including tenant context for session creation after callback)
    await sessionStore.saveAuthFlowState({
      state,
      nonce,
      codeVerifier,
      redirectUri,
      clientType,
      returnTo: query.returnTo,
      tenantId,
      tenantSlug,
      idpHint: query.kc_idp_hint,
      appType,
      createdAt: Date.now(),
    });

    // Support both loginHint and login_hint formats (storefront uses login_hint)
    const loginHint = query.loginHint || query.login_hint;

    // Convert prompt=create to kc_action=register for Keycloak registration
    // 'create' is not a standard OIDC prompt value, but we support it for convenience
    let kcAction = query.kc_action;
    let prompt = query.prompt;
    if (prompt === 'create') {
      kcAction = 'register';
      prompt = undefined; // Don't pass 'create' as prompt to Keycloak
    }

    const authUrl = await oidcClient.getAuthorizationUrl(clientType, {
      redirectUri,
      scope: 'openid profile email offline_access',
      state,
      nonce,
      codeVerifier,
      prompt,
      loginHint,
      kcIdpHint: query.kc_idp_hint, // Pass IDP hint to skip Keycloak login page
      kcAction, // Pass action for registration or password reset
    });

    logger.info(
      { clientType, state, kcIdpHint: query.kc_idp_hint, kcAction, tenantSlug, redirectUri },
      'Initiating Keycloak auth flow'
    );

    return reply.redirect(authUrl);
  });

  // ============================================================================
  // GET /auth/callback
  // Handles the OIDC callback after authentication
  // ============================================================================
  fastify.get<{
    Querystring: z.infer<typeof callbackQuerySchema>;
  }>('/auth/callback', async (request, reply) => {
    const query = callbackQuerySchema.parse(request.query);

    // Check for OAuth error (Keycloak sends error without code param)
    if (query.error) {
      const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;
      logger.error({ error: query.error, description: query.error_description, forwardedHost }, 'OAuth callback error from Keycloak');
      return reply.redirect(`/auth/error?error=${encodeURIComponent(query.error)}`);
    }

    // Ensure code is present (it's optional in schema to allow error responses through)
    if (!query.code) {
      logger.warn({ state: query.state }, 'Callback received without code or error');
      return reply.redirect('/auth/error?error=callback_error');
    }

    // Retrieve auth flow state
    const authState = await sessionStore.getAuthFlowState(query.state);
    if (!authState) {
      logger.warn({ state: query.state }, 'Invalid or expired auth state');
      return reply.redirect('/auth/error?error=invalid_state');
    }

    try {
      // Exchange code for tokens
      const tokens = await oidcClient.exchangeCode(
        authState.clientType,
        {
          code: query.code,
          state: query.state,
          iss: query.iss,
          session_state: query.session_state,
        },
        authState.redirectUri,
        authState.codeVerifier,
        authState.nonce
      );
      const userInfo = await oidcClient.getUserInfo(authState.clientType, tokens.accessToken);

      // For home app, enforce platform-admin role requirement.
      // The Keycloak realm_roles mapper puts roles in a top-level `roles` claim.
      if (authState.appType === 'home') {
        const keycloakRoles = (userInfo.roles as string[])
          || (userInfo.realm_access as { roles?: string[] })?.roles
          || [];
        if (!keycloakRoles.includes('platform-admin')) {
          logger.warn(
            { email: userInfo.email, roles: keycloakRoles },
            'Home app login rejected — user does not have platform-admin role'
          );
          return reply.redirect('/auth/error?error=access_denied&error_description=You+do+not+have+permission+to+access+the+platform+admin+portal');
        }
        logger.info({ email: userInfo.email }, 'Home app login authorized — platform-admin role verified');
      }

      // Determine tenant context:
      // 1. Use auth state tenant context if provided (from storefront/admin login)
      // 2. Fall back to Keycloak userinfo tenant claims (if configured)
      let tenantId = authState.tenantId || (userInfo.tenant_id as string | undefined);
      let tenantSlug = authState.tenantSlug || (userInfo.tenant_slug as string | undefined);

      if (authState.tenantId || authState.tenantSlug) {
        logger.info(
          { tenantId, tenantSlug, fromAuthState: true },
          'Using tenant context from auth state'
        );
      }

      // Enrich session with tenant role from tenant-service
      // This is critical for SSO (Google) logins where Keycloak doesn't have tenant-specific roles
      const email = userInfo.email as string | undefined;
      let userRole: string | undefined;
      let isStaff = false;

      if (email) {
        try {
          const tenantsResult = await tenantServiceClient.getUserTenants(email);
          if (tenantsResult.success && tenantsResult.data?.tenants) {
            const tenants = tenantsResult.data.tenants;

            // If we have a tenant slug, find the matching tenant
            if (tenantSlug) {
              const matchedTenant = tenants.find(t => t.slug === tenantSlug);
              if (matchedTenant) {
                tenantId = tenantId || matchedTenant.id;
                userRole = matchedTenant.role;
                // getUserTenants already filters through validateStaffAdminAccess,
                // so any tenant returned means the user is authorized staff
                isStaff = !!userRole;
                logger.info(
                  { email, tenantSlug, tenantId, role: userRole, isStaff },
                  'Resolved tenant role from tenant-service for SSO user'
                );
              }
            } else if (tenants.length === 1) {
              // Auto-select single tenant
              tenantId = tenants[0].id;
              tenantSlug = tenants[0].slug;
              userRole = tenants[0].role;
              isStaff = !!userRole;
              logger.info(
                { email, tenantSlug, tenantId, role: userRole },
                'Auto-selected single tenant for SSO user'
              );
            }
          }
        } catch (err) {
          logger.warn({ error: err, email: maskEmail(email) }, 'Failed to resolve tenant role (non-blocking)');
        }
      }

      // If staff logged in via Google IDP, upgrade auth_method from password → password_and_google
      if (isStaff && tenantId && email && authState.idpHint === 'google') {
        tenantServiceClient.updateStaffAuthMethod(email, tenantId, 'password_and_google')
          .catch(err => logger.warn({ error: err }, 'Failed to update staff auth method (non-blocking)'));
      }

      // Merge tenant-specific role into userInfo for session
      const enrichedUserInfo = {
        ...userInfo,
        ...(tenantId ? { tenant_id: tenantId } : {}),
        ...(tenantSlug ? { tenant_slug: tenantSlug } : {}),
        ...(userRole ? { role: userRole } : {}),
        ...(isStaff ? { is_staff: true } : {}),
        ...(userRole ? { realm_access: { roles: [userRole, ...(((userInfo.realm_access as any)?.roles) || [])] } } : {}),
      };

      // Create session with tenant context
      const session = await sessionStore.createSession({
        userId: userInfo.sub as string,
        tenantId,
        tenantSlug,
        clientType: authState.clientType,
        accessToken: tokens.accessToken,
        idToken: tokens.idToken,
        refreshToken: tokens.refreshToken,
        expiresAt: tokens.expiresAt,
        userInfo: enrichedUserInfo,
      });

      // Set session cookie with dynamic domain for custom domain support
      const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;
      setSessionCookie(reply, session.id, forwardedHost);

      logger.info(
        { userId: session.userId, sessionId: session.id, clientType: authState.clientType, forwardedHost, tenantSlug, idpHint: authState.idpHint },
        'Authentication successful'
      );

      // Publish login success event for notifications (non-blocking)
      // Only publish for customer logins (storefront) with tenant context
      if (tenantId && authState.clientType === 'customer') {
        const ipAddress = (request.headers['x-forwarded-for'] as string)?.split(',')[0]?.trim()
          || request.ip
          || 'unknown';
        const userAgent = request.headers['user-agent'] || 'unknown';
        const email = (userInfo.email as string) || '';

        // Publish in background - don't block the redirect
        natsClient.publishLoginSuccess(
          tenantId,
          session.userId,
          email,
          ipAddress,
          userAgent,
          'oidc'
        ).catch((err) => {
          logger.warn({ error: err }, 'Failed to publish login event (non-critical)');
        });
      }

      // Redirect to return URL or default (validated to prevent open redirects)
      const returnTo = sanitizeReturnTo(authState.returnTo, '/');
      return reply.redirect(returnTo);
    } catch (error: any) {
      logger.error({
        error,
        errorMessage: error?.message,
        errorResponse: error?.response?.body,
        statusCode: error?.response?.statusCode,
        redirectUri: authState.redirectUri,
        clientType: authState.clientType,
      }, 'Token exchange failed');
      return reply.redirect('/auth/error?error=token_exchange_failed');
    }
  });

  // ============================================================================
  // POST /auth/logout
  // Logs out the user - clears session locally without redirecting to IDP
  // ============================================================================
  fastify.post<{
    Querystring: z.infer<typeof logoutQuerySchema>;
  }>('/auth/logout', async (request, reply) => {
    const session = await getSession(request);
    const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;

    if (session) {
      // Delete session
      await sessionStore.deleteSession(session.id);

      // Clear cookie with dynamic domain for custom domain support
      clearSessionCookie(reply, forwardedHost);

      // Revoke refresh token at Keycloak
      if (session.refreshToken) {
        try {
          await oidcClient.revokeToken(session.clientType, session.refreshToken, 'refresh_token');
        } catch (error) {
          logger.warn({ error }, 'Failed to revoke refresh token');
        }
      }

      logger.info({ userId: session.userId, sessionId: session.id }, 'User logged out');
    }

    // Return JSON for programmatic callers (storefront uses fetch + client-side navigation)
    return reply.send({ success: true, logged_out: !!session });
  });

  // ============================================================================
  // GET /auth/logout (for redirect-based logout)
  // ============================================================================
  fastify.get<{
    Querystring: z.infer<typeof logoutQuerySchema>;
  }>('/auth/logout', async (request, reply) => {
    const session = await getSession(request);
    const query = logoutQuerySchema.parse(request.query);
    const returnTo = sanitizeReturnTo(query.returnTo, '/login');

    if (session) {
      await sessionStore.deleteSession(session.id);
      const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;
      clearSessionCookie(reply, forwardedHost);

      // Revoke refresh token at Keycloak
      if (session.refreshToken) {
        try {
          await oidcClient.revokeToken(session.clientType, session.refreshToken, 'refresh_token');
        } catch (error) {
          logger.warn({ error }, 'Failed to revoke refresh token');
        }
      }

      logger.info({ userId: session.userId, sessionId: session.id }, 'User logged out');

      // Redirect to app's login page
      return reply.redirect(returnTo);
    }

    return reply.redirect(returnTo);
  });

  // ============================================================================
  // GET /auth/session
  // Returns current session info (user info, not tokens)
  // ============================================================================
  fastify.get('/auth/session', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({ authenticated: false });
    }

    // Check if session is expired
    // Note: We use a long session expiry (24 hours) for direct login sessions
    // to avoid issues with token refresh when using different Keycloak endpoints.
    // The session cookie handles the actual session lifetime.
    const now = Math.floor(Date.now() / 1000);
    if (session.expiresAt < now) {
      // Session has expired - clear it
      logger.info({ sessionId: session.id }, 'Session expired');
      await sessionStore.deleteSession(session.id);
      const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;
      clearSessionCookie(reply, forwardedHost);
      return reply.code(401).send({ authenticated: false, error: 'session_expired' });
    }

    // Read roles from both Keycloak locations:
    // - `roles` (top-level): set by the realm_roles client mapper (claim.name = "roles")
    // - `realm_access.roles`: standard Keycloak format from default scope mapper
    const roles = (session.userInfo?.roles as string[])
      || (session.userInfo?.realm_access as { roles?: string[] })?.roles
      || [];

    return reply.send({
      authenticated: true,
      user: {
        id: session.userId,
        email: session.userInfo?.email,
        name: session.userInfo?.name,
        firstName: session.userInfo?.given_name,
        lastName: session.userInfo?.family_name,
        tenantId: session.tenantId,
        tenantSlug: session.tenantSlug,
        roles,
      },
      expiresAt: session.expiresAt,
      csrfToken: session.csrfToken,
    });
  });

  // ============================================================================
  // POST /auth/refresh
  // Manually refresh the session tokens
  // ============================================================================
  fastify.post('/auth/refresh', async (request, reply) => {
    const session = await getSession(request);

    if (!session || !session.refreshToken) {
      return reply.code(401).send({ error: 'no_session' });
    }

    try {
      const newTokens = await oidcClient.refreshTokens(session.clientType, session.refreshToken);

      await sessionStore.updateSession(session.id, {
        accessToken: newTokens.accessToken,
        idToken: newTokens.idToken,
        refreshToken: newTokens.refreshToken || session.refreshToken,
        expiresAt: newTokens.expiresAt,
      });

      return reply.send({
        success: true,
        expiresAt: newTokens.expiresAt,
      });
    } catch (error) {
      logger.error({ error }, 'Token refresh failed');
      await sessionStore.deleteSession(session.id);
      const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;
      clearSessionCookie(reply, forwardedHost);
      return reply.code(401).send({ error: 'refresh_failed' });
    }
  });

  // ============================================================================
  // GET /auth/csrf
  // Returns CSRF token for the current session
  // ============================================================================
  fastify.get('/auth/csrf', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({ error: 'no_session' });
    }

    return reply.send({ csrfToken: session.csrfToken });
  });

  // ============================================================================
  // POST /auth/ws-ticket
  // Creates a short-lived ticket for WebSocket authentication
  // This allows WebSocket connections without exposing tokens in URLs
  // ============================================================================
  fastify.post('/auth/ws-ticket', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({ error: 'unauthorized' });
    }

    const ticketId = uuidv4();
    const ticketData: WsTicketData = {
      userId: session.userId,
      tenantId: session.tenantId,
      tenantSlug: session.tenantSlug,
      sessionId: session.id,
      createdAt: Date.now(),
    };

    await sessionStore.saveWsTicket(ticketId, ticketData);

    logger.info({ userId: session.userId, ticketId }, 'WebSocket ticket created');

    return reply.send({
      ticket: ticketId,
      expiresIn: 30, // 30 seconds
    });
  });

  // ============================================================================
  // POST /internal/validate-ws-ticket
  // Internal endpoint for services (e.g., notification-hub) to validate WS tickets
  // This endpoint should only be accessible from internal services
  // ============================================================================
  fastify.post<{
    Body: { ticket: string };
  }>('/internal/validate-ws-ticket', { preHandler: requireInternalServiceKey }, async (request, reply) => {
    const { ticket } = request.body as { ticket: string };

    if (!ticket) {
      return reply.code(400).send({ error: 'missing_ticket' });
    }

    const ticketData = await sessionStore.consumeWsTicket(ticket);

    if (!ticketData) {
      return reply.code(401).send({ error: 'invalid_ticket' });
    }

    logger.debug({ ticketId: ticket, userId: ticketData.userId }, 'WebSocket ticket validated');

    return reply.send({
      valid: true,
      user_id: ticketData.userId,
      tenant_id: ticketData.tenantId,
      tenant_slug: ticketData.tenantSlug,
      session_id: ticketData.sessionId,
    });
  });

  // ============================================================================
  // GET /internal/get-token
  // Internal endpoint for BFF services (Next.js) to get the access token
  // This enables server-side API calls to backend services with proper auth
  // SECURITY: Should only be accessible from internal services (not exposed externally)
  // ============================================================================
  fastify.get('/internal/get-token', { preHandler: requireInternalServiceKey }, async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({ error: 'no_session' });
    }

    // Check if session is expired
    const now = Math.floor(Date.now() / 1000);
    if (session.expiresAt < now) {
      // Try to refresh if we have a refresh token
      if (session.refreshToken) {
        try {
          const newTokens = await oidcClient.refreshTokens(session.clientType, session.refreshToken);
          await sessionStore.updateSession(session.id, {
            accessToken: newTokens.accessToken,
            idToken: newTokens.idToken,
            refreshToken: newTokens.refreshToken || session.refreshToken,
            expiresAt: newTokens.expiresAt,
          });

          return reply.send({
            access_token: newTokens.accessToken,
            user_id: session.userId,
            tenant_id: session.tenantId,
            tenant_slug: session.tenantSlug,
            expires_at: newTokens.expiresAt,
          });
        } catch (error) {
          logger.error({ error, sessionId: session.id }, 'Token refresh failed during internal get-token');
          return reply.code(401).send({ error: 'session_expired' });
        }
      }
      return reply.code(401).send({ error: 'session_expired' });
    }

    logger.debug({ userId: session.userId }, 'Internal token retrieved');

    return reply.send({
      access_token: session.accessToken,
      user_id: session.userId,
      tenant_id: session.tenantId,
      tenant_slug: session.tenantSlug,
      expires_at: session.expiresAt,
    });
  });

  // ============================================================================
  // POST /auth/transfer-session
  // Creates a one-time transfer code for cross-subdomain session handoff
  // Used when redirecting from onboarding to admin dashboard
  // ============================================================================
  fastify.post('/auth/transfer-session', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({ error: 'unauthorized' });
    }

    const transferCode = uuidv4();
    const transferData: SessionTransferData = {
      sessionId: session.id,
      userId: session.userId,
      tenantId: session.tenantId,
      tenantSlug: session.tenantSlug,
      clientType: session.clientType,
      accessToken: session.accessToken,
      idToken: session.idToken,
      refreshToken: session.refreshToken,
      expiresAt: session.expiresAt,
      userInfo: session.userInfo,
      createdAt: Date.now(),
    };

    await sessionStore.saveSessionTransfer(transferCode, transferData);

    logger.info({ userId: session.userId, code: transferCode }, 'Session transfer created');

    return reply.send({
      code: transferCode,
      expiresIn: 60, // 60 seconds
    });
  });

  // ============================================================================
  // GET /auth/accept-transfer
  // Accepts a session transfer code and creates a new session on this domain
  // Used when receiving a redirect from onboarding with a transfer code
  // ============================================================================
  fastify.get<{
    Querystring: { code: string; returnTo?: string };
  }>('/auth/accept-transfer', async (request, reply) => {
    const { code, returnTo } = request.query as { code: string; returnTo?: string };

    if (!code) {
      return reply.redirect('/login?error=missing_transfer_code');
    }

    // Rate limit by IP to prevent brute-force transfer code guessing
    const clientIP = request.ip;
    const rateLimit = await sessionStore.checkRateLimit(`accept-transfer:${clientIP}`, 10, 60);
    if (!rateLimit.allowed) {
      logger.warn({ ip: maskIP(clientIP) }, 'Rate limit exceeded for session transfer');
      return reply.redirect('/login?error=rate_limited');
    }

    const transferData = await sessionStore.consumeSessionTransfer(code);

    if (!transferData) {
      logger.warn({ code }, 'Invalid or expired session transfer code');
      return reply.redirect('/login?error=invalid_transfer_code');
    }

    // Create new session with transferred data
    const session = await sessionStore.createSession({
      userId: transferData.userId,
      tenantId: transferData.tenantId,
      tenantSlug: transferData.tenantSlug,
      clientType: transferData.clientType,
      accessToken: transferData.accessToken,
      idToken: transferData.idToken,
      refreshToken: transferData.refreshToken,
      expiresAt: transferData.expiresAt,
      userInfo: transferData.userInfo,
    });

    // Set session cookie for this domain with dynamic domain support
    const forwardedHost = request.headers['x-forwarded-host'] as string || request.hostname;
    setSessionCookie(reply, session.id, forwardedHost);

    logger.info({ userId: session.userId, sessionId: session.id, forwardedHost }, 'Session transfer accepted');

    // Redirect to return URL or default dashboard (validated to prevent open redirects)
    return reply.redirect(sanitizeReturnTo(returnTo, '/'));
  });

  // ============================================================================
  // POST /auth/import-tokens
  // Creates a session and transfer code from Keycloak tokens
  // Used by onboarding to auto-login users after account creation
  // SECURITY: Internal-only endpoint, protected by service key
  // ============================================================================
  fastify.post<{
    Body: {
      access_token: string;
      refresh_token?: string;
      expires_in?: number;
      user_id: string;
      email: string;
      tenant_id: string;
      tenant_slug: string;
      first_name?: string;
      last_name?: string;
    };
  }>('/auth/import-tokens', { preHandler: requireInternalServiceKey }, async (request, reply) => {
    const {
      access_token,
      refresh_token,
      expires_in,
      user_id,
      email,
      tenant_id,
      tenant_slug,
      first_name,
      last_name,
    } = request.body;

    // Validate required fields
    if (!access_token || !user_id || !email || !tenant_id || !tenant_slug) {
      return reply.code(400).send({
        error: 'missing_required_fields',
        message: 'access_token, user_id, email, tenant_id, and tenant_slug are required',
      });
    }

    try {
      // Calculate expiration time
      const expiresAt = expires_in
        ? Math.floor(Date.now() / 1000) + expires_in
        : Math.floor(Date.now() / 1000) + 300; // Default 5 minutes

      // Create transfer code directly without creating a session first
      const transferCode = uuidv4();
      const fwdHost = request.headers['x-forwarded-host'] as string | undefined;
      const fwdAuthContext = request.headers['x-auth-context'] as string | undefined;
      const resolvedClientType: 'internal' | 'customer' =
        (fwdAuthContext === 'admin' || isAdminHost(fwdHost) || isCustomDomainAdminHost(fwdHost)) ? 'internal' : 'customer';

      logger.debug(
        { fwdHost, fwdAuthContext, resolvedClientType, tenantSlug: tenant_slug },
        'import-tokens: resolved client type for transfer session'
      );

      const transferData: SessionTransferData = {
        sessionId: uuidv4(), // Will be replaced when session is created on accept
        userId: user_id,
        tenantId: tenant_id,
        tenantSlug: tenant_slug,
        clientType: resolvedClientType,
        accessToken: access_token,
        refreshToken: refresh_token,
        expiresAt,
        userInfo: {
          email,
          firstName: first_name,
          lastName: last_name,
          name: [first_name, last_name].filter(Boolean).join(' ') || email,
        },
        createdAt: Date.now(),
      };

      await sessionStore.saveSessionTransfer(transferCode, transferData);

      logger.info(
        { userId: user_id, tenantSlug: tenant_slug, code: transferCode },
        'Token import created transfer code for auto-login'
      );

      return reply.send({
        success: true,
        transfer_code: transferCode,
        expires_in: 60, // 60 seconds
      });
    } catch (error) {
      logger.error({ error, userId: user_id }, 'Failed to import tokens');
      return reply.code(500).send({
        error: 'import_failed',
        message: 'Failed to create auto-login session',
      });
    }
  });
}

// Helper functions
function determineClientType(request: FastifyRequest): 'internal' | 'customer' {
  const rawForwardedHost = request.headers['x-forwarded-host'] as string | undefined;
  const fullForwardedHost = rawForwardedHost || request.hostname || '';
  const forwardedHost = fullForwardedHost.split(':')[0];

  // Check X-Auth-Context header first (set by Next.js admin/storefront apps)
  // This handles custom domains where hostname pattern matching won't work
  const authContext = request.headers['x-auth-context'] as string | undefined;
  if (authContext === 'admin') {
    logger.debug({ forwardedHost, authContext, method: 'x-auth-context' }, 'Client type resolved to internal');
    return 'internal';
  }

  // Fall back to hostname pattern matching for direct requests
  const baseDomain = config.baseDomain;
  let matchMethod: string | undefined;
  if (forwardedHost === `admin.${baseDomain}`) {
    matchMethod = 'admin-base-domain';
  } else if (isAdminHost(forwardedHost)) {
    matchMethod = 'admin-host';
  } else if (isHomeHost(forwardedHost)) {
    matchMethod = 'home-host';
  } else if (isOnboardingHost(fullForwardedHost)) {
    matchMethod = 'onboarding-host';
  } else if (isCustomDomainAdminHost(fullForwardedHost)) {
    matchMethod = 'custom-domain-admin';
  }

  if (matchMethod) {
    logger.debug({ forwardedHost, method: matchMethod }, 'Client type resolved to internal');
    return 'internal';
  }

  logger.debug({ forwardedHost }, 'Client type resolved to customer (no admin/internal match)');
  return 'customer';
}

function getCallbackUrl(request: FastifyRequest, _clientType: 'internal' | 'customer'): string {
  const protocol = request.headers['x-forwarded-proto'] || 'https';
  const host = request.headers['x-forwarded-host'] || request.hostname;
  // tesserix-home proxies auth routes through /api/auth/*, so the callback
  // URL needs the /api prefix for the Next.js catch-all route to handle it
  const callbackPath = isHomeHost(host as string) ? '/api/auth/callback' : '/auth/callback';
  return `${protocol}://${host}${callbackPath}`;
}
