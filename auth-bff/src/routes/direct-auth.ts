/**
 * Direct Authentication Routes for Multi-Tenant Credential Isolation
 *
 * These routes enable direct password authentication without redirecting to Keycloak's
 * login page. This allows the frontend to:
 * 1. Show a tenant selection screen after email entry
 * 2. Authenticate users with tenant-specific credentials
 * 3. Receive tokens and create sessions in a single API call
 *
 * Enterprise Security Features:
 * - Rate limiting per IP and email
 * - Account lockout after failed attempts
 * - MFA support (step-up authentication)
 * - Audit logging
 * - Secure session management
 *
 * The flow:
 * 1. POST /auth/direct/lookup-tenants - Get available tenants for email
 * 2. POST /auth/direct/login - Authenticate with tenant-specific password
 * 3. Session is created automatically on success
 */

import { FastifyInstance, FastifyReply } from 'fastify';
import { z } from 'zod';
import { config } from '../config';
import { tenantServiceClient } from '../tenant-service-client';
import { sessionStore } from '../session-store';
import { createLogger } from '../logger';
import { v4 as uuidv4 } from 'uuid';

const logger = createLogger('direct-auth');

// ============================================================================
// Request Schemas
// ============================================================================

const lookupTenantsSchema = z.object({
  email: z.string().email('Invalid email format'),
});

const directLoginSchema = z.object({
  email: z.string().email('Invalid email format'),
  password: z.string().min(1, 'Password is required'),
  tenant_slug: z.string().min(1, 'Tenant selection is required'),
  tenant_id: z.string().uuid().optional(),
  remember_me: z.boolean().optional().default(false),
});

const accountStatusSchema = z.object({
  email: z.string().email('Invalid email format'),
  tenant_slug: z.string().min(1, 'Tenant selection is required'),
});

// ============================================================================
// Rate Limiting State (In-memory, should use Redis in production)
// ============================================================================

interface RateLimitEntry {
  count: number;
  resetAt: number;
}

const rateLimits = new Map<string, RateLimitEntry>();
const RATE_LIMIT_WINDOW_MS = 60000; // 1 minute
const RATE_LIMIT_MAX_ATTEMPTS = 10; // 10 attempts per minute per key

function checkRateLimit(key: string): { allowed: boolean; remaining: number; resetIn: number } {
  const now = Date.now();
  const entry = rateLimits.get(key);

  if (!entry || now > entry.resetAt) {
    rateLimits.set(key, { count: 1, resetAt: now + RATE_LIMIT_WINDOW_MS });
    return { allowed: true, remaining: RATE_LIMIT_MAX_ATTEMPTS - 1, resetIn: RATE_LIMIT_WINDOW_MS };
  }

  if (entry.count >= RATE_LIMIT_MAX_ATTEMPTS) {
    return { allowed: false, remaining: 0, resetIn: entry.resetAt - now };
  }

  entry.count++;
  return { allowed: true, remaining: RATE_LIMIT_MAX_ATTEMPTS - entry.count, resetIn: entry.resetAt - now };
}

// ============================================================================
// Cookie Helper
// ============================================================================

const setSessionCookie = (reply: FastifyReply, sessionId: string, rememberMe: boolean = false) => {
  const maxAge = rememberMe ? config.session.maxAge * 7 : config.session.maxAge; // 7 days if remember me
  reply.setCookie(config.session.cookieName, sessionId, {
    httpOnly: true,
    secure: config.server.nodeEnv === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge,
    domain: config.session.cookieDomain,
  });
};

// ============================================================================
// Routes
// ============================================================================

export async function directAuthRoutes(fastify: FastifyInstance) {
  // ==========================================================================
  // POST /auth/direct/lookup-tenants
  // Returns available tenants for a given email address
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof lookupTenantsSchema>;
  }>('/auth/direct/lookup-tenants', async (request, reply) => {
    const validation = lookupTenantsSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email } = validation.data;
    const clientIP = request.ip;

    // Rate limit by IP
    const rateLimitKey = `tenant-lookup:${clientIP}`;
    const rateLimit = checkRateLimit(rateLimitKey);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP }, 'Rate limit exceeded for tenant lookup');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many requests. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Get tenants from tenant-service
    const result = await tenantServiceClient.getUserTenants(email);

    if (!result.success) {
      // Don't reveal if email doesn't exist - return empty array
      return reply.send({
        success: true,
        data: {
          tenants: [],
          count: 0,
          // Security: Always show same message regardless of whether email exists
          message: 'If this email is registered, you will see your available organizations.',
        },
      });
    }

    // Transform response for frontend
    // Note: tenant-service returns id, slug, name (not tenant_id, tenant_slug, tenant_name)
    const tenants = result.data?.tenants.map(t => ({
      id: t.id,
      slug: t.slug,
      name: t.name || t.display_name || 'Unknown',
      logo_url: t.logo_url,
    })) || [];

    return reply.send({
      success: true,
      data: {
        tenants,
        count: tenants.length,
        // If single tenant, could auto-proceed to password entry
        single_tenant: tenants.length === 1,
      },
    });
  });

  // ==========================================================================
  // POST /auth/direct/login
  // Authenticates user with tenant-specific credentials
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof directLoginSchema>;
  }>('/auth/direct/login', async (request, reply) => {
    const validation = directLoginSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, password, tenant_slug, tenant_id, remember_me } = validation.data;
    const clientIP = request.ip;
    const userAgent = request.headers['user-agent'] || 'unknown';

    // Rate limit by IP + email combination
    const rateLimitKey = `login:${clientIP}:${email.toLowerCase()}`;
    const rateLimit = checkRateLimit(rateLimitKey);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email }, 'Rate limit exceeded for login');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many login attempts. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Validate credentials with tenant-service
    const result = await tenantServiceClient.validateCredentials(
      { email, password, tenant_slug, tenant_id },
      clientIP,
      userAgent
    );

    if (!result.success) {
      logger.error({ email, tenant_slug }, 'Tenant service validation failed');
      return reply.code(503).send({
        success: false,
        error: 'SERVICE_UNAVAILABLE',
        message: 'Authentication service is temporarily unavailable. Please try again.',
      });
    }

    const data = result.data!;

    // Handle invalid credentials
    if (!data.valid) {
      logger.info({
        email,
        tenant_slug,
        error_code: data.error_code,
        account_locked: data.account_locked,
      }, 'Login failed');

      // Account locked
      if (data.account_locked) {
        return reply.code(423).send({
          success: false,
          error: 'ACCOUNT_LOCKED',
          message: 'Your account has been temporarily locked due to multiple failed login attempts.',
          locked_until: data.locked_until,
        });
      }

      // Invalid credentials
      return reply.code(401).send({
        success: false,
        error: data.error_code || 'INVALID_CREDENTIALS',
        message: data.message || 'Invalid email or password.',
        remaining_attempts: data.remaining_attempts,
      });
    }

    // Handle MFA requirement (step-up authentication)
    if (data.mfa_required) {
      // Create a temporary MFA session
      const mfaSessionId = uuidv4();
      await sessionStore.saveMfaSession(mfaSessionId, {
        userId: data.user_id!,
        email: data.email,
        tenantId: data.tenant_id,
        tenantSlug: data.tenant_slug,
        mfaEnabled: data.mfa_enabled,
        createdAt: Date.now(),
      });

      return reply.send({
        success: true,
        mfa_required: true,
        mfa_session: mfaSessionId,
        mfa_methods: data.mfa_enabled ? ['totp', 'email'] : ['email'], // Available MFA methods
        message: 'Multi-factor authentication required.',
      });
    }

    // Check if we received tokens from tenant-service
    if (!data.access_token) {
      logger.error({ email, tenant_slug }, 'No tokens received from tenant-service');
      return reply.code(500).send({
        success: false,
        error: 'TOKEN_ERROR',
        message: 'Failed to complete authentication. Please try again.',
      });
    }

    // Create session with tokens
    // Use 24 hours for session expiry (not token expiry) to avoid frequent refresh attempts
    // The session cookie handles the actual session lifetime
    const sessionExpirySeconds = 86400; // 24 hours
    const session = await sessionStore.createSession({
      userId: data.user_id!,
      tenantId: data.tenant_id,
      tenantSlug: data.tenant_slug,
      clientType: 'customer',
      accessToken: data.access_token,
      idToken: data.id_token,
      refreshToken: data.refresh_token,
      expiresAt: Math.floor(Date.now() / 1000) + sessionExpirySeconds,
      userInfo: {
        sub: data.keycloak_user_id || data.user_id,
        email: data.email,
        given_name: data.first_name,
        family_name: data.last_name,
        name: data.first_name && data.last_name
          ? `${data.first_name} ${data.last_name}`
          : data.email,
        tenant_id: data.tenant_id,
        tenant_slug: data.tenant_slug,
      },
    });

    // Set session cookie
    setSessionCookie(reply, session.id, remember_me);

    logger.info({
      userId: session.userId,
      sessionId: session.id,
      tenant_slug,
    }, 'Direct login successful');

    return reply.send({
      success: true,
      authenticated: true,
      user: {
        id: data.user_id,
        email: data.email,
        first_name: data.first_name,
        last_name: data.last_name,
        tenant_id: data.tenant_id,
        tenant_slug: data.tenant_slug,
        role: data.role,
      },
      session: {
        expires_at: session.expiresAt,
        csrf_token: session.csrfToken,
      },
    });
  });

  // ==========================================================================
  // POST /auth/direct/account-status
  // Checks if an account is locked before password entry
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof accountStatusSchema>;
  }>('/auth/direct/account-status', async (request, reply) => {
    const validation = accountStatusSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, tenant_slug } = validation.data;

    const result = await tenantServiceClient.checkAccountStatus(email, tenant_slug);

    // Don't reveal if account exists - only reveal if locked
    if (!result.success || !result.account_exists) {
      return reply.send({
        success: true,
        account_locked: false,
      });
    }

    if (result.account_locked) {
      return reply.send({
        success: true,
        account_locked: true,
        locked_until: result.locked_until,
        message: 'Your account has been temporarily locked due to multiple failed login attempts.',
      });
    }

    return reply.send({
      success: true,
      account_locked: false,
    });
  });

  // ==========================================================================
  // POST /auth/direct/mfa/verify
  // Verifies MFA code and completes authentication
  // ==========================================================================
  fastify.post<{
    Body: {
      mfa_session: string;
      code: string;
      method?: 'totp' | 'email' | 'sms';
    };
  }>('/auth/direct/mfa/verify', async (request, reply) => {
    const { mfa_session, code, method = 'totp' } = request.body;

    if (!mfa_session || !code) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: 'MFA session and code are required.',
      });
    }

    // Get MFA session
    const mfaData = await sessionStore.getMfaSession(mfa_session);
    if (!mfaData) {
      return reply.code(401).send({
        success: false,
        error: 'INVALID_MFA_SESSION',
        message: 'MFA session expired or invalid. Please start over.',
      });
    }

    // TODO: Implement actual MFA verification
    // For now, this is a placeholder that should call tenant-service
    // to verify the TOTP code or email verification code
    logger.info({ mfa_session, method }, 'MFA verification requested (not yet implemented)');

    return reply.code(501).send({
      success: false,
      error: 'NOT_IMPLEMENTED',
      message: 'MFA verification is not yet implemented.',
    });
  });

  // ==========================================================================
  // POST /auth/direct/mfa/send-code
  // Sends MFA code via email or SMS
  // ==========================================================================
  fastify.post<{
    Body: {
      mfa_session: string;
      method: 'email' | 'sms';
    };
  }>('/auth/direct/mfa/send-code', async (request, reply) => {
    const { mfa_session, method } = request.body;

    if (!mfa_session || !method) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: 'MFA session and method are required.',
      });
    }

    // Get MFA session
    const mfaData = await sessionStore.getMfaSession(mfa_session);
    if (!mfaData) {
      return reply.code(401).send({
        success: false,
        error: 'INVALID_MFA_SESSION',
        message: 'MFA session expired or invalid. Please start over.',
      });
    }

    // TODO: Implement actual MFA code sending
    // This should call tenant-service or a notification service
    logger.info({ mfa_session, method }, 'MFA code send requested (not yet implemented)');

    return reply.code(501).send({
      success: false,
      error: 'NOT_IMPLEMENTED',
      message: 'MFA code sending is not yet implemented.',
    });
  });
}
