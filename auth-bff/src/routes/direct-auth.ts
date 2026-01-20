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
import { tenantServiceClient, maskEmail } from '../tenant-service-client';
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

const directRegisterSchema = z.object({
  email: z.string().email('Invalid email format'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  first_name: z.string().min(1, 'First name is required'),
  last_name: z.string().min(1, 'Last name is required'),
  phone: z.string().optional(),
  tenant_slug: z.string().min(1, 'Store is required'),
});

const checkDeactivatedSchema = z.object({
  email: z.string().email('Invalid email format'),
  tenant_slug: z.string().min(1, 'Store is required'),
});

const reactivateAccountSchema = z.object({
  email: z.string().email('Invalid email format'),
  password: z.string().min(1, 'Password is required'),
  tenant_slug: z.string().min(1, 'Store is required'),
});

const deactivateAccountSchema = z.object({
  reason: z.string().optional(),
});

const requestPasswordResetSchema = z.object({
  email: z.string().email('Invalid email format'),
  tenant_slug: z.string().min(1, 'Store is required'),
});

const validateResetTokenSchema = z.object({
  token: z.string().min(1, 'Token is required'),
});

const resetPasswordSchema = z.object({
  token: z.string().min(1, 'Token is required'),
  new_password: z.string().min(8, 'Password must be at least 8 characters'),
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
      logger.warn({ ip: clientIP, email: maskEmail(email) }, 'Rate limit exceeded for login');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many login attempts. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Validate credentials with tenant-service
    // SECURITY: Pass auth_context: 'customer' to prevent staff from logging into storefront
    // This ensures store owners/staff cannot use admin credentials on customer-facing storefront
    const result = await tenantServiceClient.validateCredentials(
      { email, password, tenant_slug, tenant_id, auth_context: 'customer' },
      clientIP,
      userAgent
    );

    if (!result.success) {
      logger.error({ email: maskEmail(email), tenant_slug }, 'Tenant service validation failed');
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
        email: maskEmail(email),
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
      logger.error({ email: maskEmail(email), tenant_slug }, 'No tokens received from tenant-service');
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
        // Include role in realm_access for frontend authorization check
        realm_access: {
          roles: data.role ? [data.role] : [],
        },
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
  // POST /auth/direct/admin/login
  // Authenticates STAFF members for admin portal access
  // This endpoint uses auth_context: 'staff' to allow staff-only login
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof directLoginSchema>;
  }>('/auth/direct/admin/login', async (request, reply) => {
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
    const rateLimitKey = `admin-login:${clientIP}:${email.toLowerCase()}`;
    const rateLimit = checkRateLimit(rateLimitKey);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email: maskEmail(email) }, 'Rate limit exceeded for admin login');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many login attempts. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Validate credentials with tenant-service
    // SECURITY: Pass auth_context: 'staff' to validate staff credentials
    // This endpoint is for admin portal, where staff members authenticate
    const result = await tenantServiceClient.validateCredentials(
      { email, password, tenant_slug, tenant_id, auth_context: 'staff' },
      clientIP,
      userAgent
    );

    if (!result.success) {
      logger.error({ email: maskEmail(email), tenant_slug }, 'Tenant service validation failed for staff');
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
        email: maskEmail(email),
        tenant_slug,
        error_code: data.error_code,
        account_locked: data.account_locked,
      }, 'Admin login failed');

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
        mfa_methods: data.mfa_enabled ? ['totp', 'email'] : ['email'],
        message: 'Multi-factor authentication required.',
      });
    }

    // Check if we received tokens from tenant-service
    if (!data.access_token) {
      logger.error({ email: maskEmail(email), tenant_slug }, 'No tokens received from tenant-service for staff');
      return reply.code(500).send({
        success: false,
        error: 'TOKEN_ERROR',
        message: 'Failed to complete authentication. Please try again.',
      });
    }

    // Create session with tokens
    // Use 24 hours for session expiry (not token expiry) to avoid frequent refresh attempts
    const sessionExpirySeconds = 86400; // 24 hours
    const session = await sessionStore.createSession({
      userId: data.user_id!,
      tenantId: data.tenant_id,
      tenantSlug: data.tenant_slug,
      clientType: 'customer', // Staff still use customer realm
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
        role: data.role,
        is_staff: true, // Mark session as staff
      },
    });

    // Set session cookie
    setSessionCookie(reply, session.id, remember_me);

    logger.info({
      userId: session.userId,
      sessionId: session.id,
      tenant_slug,
    }, 'Admin/Staff direct login successful');

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
        is_staff: true,
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

  // ==========================================================================
  // POST /auth/direct/register
  // Registers a new customer and returns tokens for immediate login
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof directRegisterSchema>;
  }>('/auth/direct/register', async (request, reply) => {
    const validation = directRegisterSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, password, first_name, last_name, phone, tenant_slug } = validation.data;
    const clientIP = request.ip;
    const userAgent = request.headers['user-agent'] || 'unknown';

    // Rate limit by IP
    const rateLimitKey = `register:${clientIP}`;
    const rateLimit = checkRateLimit(rateLimitKey);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP }, 'Rate limit exceeded for registration');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many registration attempts. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Register with tenant-service
    const result = await tenantServiceClient.registerCustomer(
      { email, password, first_name, last_name, phone, tenant_slug },
      clientIP,
      userAgent
    );

    if (!result.success) {
      logger.error({ email: maskEmail(email), tenant_slug }, 'Tenant service registration failed');
      return reply.code(503).send({
        success: false,
        error: 'SERVICE_UNAVAILABLE',
        message: 'Registration service is temporarily unavailable. Please try again.',
      });
    }

    const data = result.data!;

    // Handle registration errors
    if (data.error_code) {
      logger.info({
        email: maskEmail(email),
        tenant_slug,
        error_code: data.error_code,
      }, 'Registration failed');

      // Email already exists
      if (data.error_code === 'EMAIL_EXISTS') {
        return reply.code(409).send({
          success: false,
          error: 'EMAIL_EXISTS',
          message: data.message || 'An account with this email already exists.',
        });
      }

      // Other errors
      return reply.code(400).send({
        success: false,
        error: data.error_code,
        message: data.message || 'Registration failed.',
      });
    }

    // Check if we received tokens from tenant-service
    if (!data.access_token) {
      // Registration successful but no tokens - user needs to log in manually
      logger.info({ email: maskEmail(email), tenant_slug }, 'Registration successful (no auto-login)');
      return reply.send({
        success: true,
        registered: true,
        auto_login: false,
        user: {
          id: data.user_id,
          email: data.email,
          first_name: data.first_name,
          last_name: data.last_name,
          tenant_id: data.tenant_id,
          tenant_slug: data.tenant_slug,
        },
        message: 'Account created successfully. Please log in.',
      });
    }

    // Create session with tokens for auto-login
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
        sub: data.user_id,
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
    setSessionCookie(reply, session.id, false);

    logger.info({
      userId: session.userId,
      sessionId: session.id,
      tenant_slug,
    }, 'Direct registration with auto-login successful');

    return reply.send({
      success: true,
      registered: true,
      authenticated: true,
      user: {
        id: data.user_id,
        email: data.email,
        first_name: data.first_name,
        last_name: data.last_name,
        tenant_id: data.tenant_id,
        tenant_slug: data.tenant_slug,
      },
      session: {
        expires_at: session.expiresAt,
        csrf_token: session.csrfToken,
      },
    });
  });

  // ==========================================================================
  // POST /auth/direct/check-deactivated
  // Checks if an account is deactivated (used during login flow)
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof checkDeactivatedSchema>;
  }>('/auth/direct/check-deactivated', async (request, reply) => {
    const validation = checkDeactivatedSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, tenant_slug } = validation.data;

    const result = await tenantServiceClient.checkDeactivatedAccount(email, tenant_slug);

    if (!result.success) {
      return reply.code(503).send({
        success: false,
        error: 'SERVICE_UNAVAILABLE',
        message: 'Unable to check account status. Please try again.',
      });
    }

    return reply.send({
      success: true,
      is_deactivated: result.is_deactivated,
      can_reactivate: result.can_reactivate,
      days_until_purge: result.days_until_purge,
      deactivated_at: result.deactivated_at,
      purge_date: result.purge_date,
    });
  });

  // ==========================================================================
  // POST /auth/direct/reactivate-account
  // Reactivates a deactivated account within the 90-day retention period
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof reactivateAccountSchema>;
  }>('/auth/direct/reactivate-account', async (request, reply) => {
    const validation = reactivateAccountSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, password, tenant_slug } = validation.data;
    const clientIP = request.ip;

    // Rate limit by IP + email
    const rateLimitKey = `reactivate:${clientIP}:${email.toLowerCase()}`;
    const rateLimit = checkRateLimit(rateLimitKey);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email: maskEmail(email) }, 'Rate limit exceeded for reactivation');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many attempts. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    const result = await tenantServiceClient.reactivateAccount({
      email,
      password,
      tenant_slug,
    });

    if (!result.success) {
      // Handle specific error codes
      if (result.error_code === 'INVALID_PASSWORD') {
        return reply.code(401).send({
          success: false,
          error: 'INVALID_PASSWORD',
          message: 'Invalid password. Please try again.',
        });
      }

      if (result.error_code === 'CANNOT_REACTIVATE') {
        return reply.code(410).send({
          success: false,
          error: 'CANNOT_REACTIVATE',
          message: 'Account cannot be reactivated. The retention period has expired.',
        });
      }

      if (result.error_code === 'NOT_DEACTIVATED') {
        return reply.code(400).send({
          success: false,
          error: 'NOT_DEACTIVATED',
          message: 'This account is not deactivated.',
        });
      }

      return reply.code(500).send({
        success: false,
        error: result.error_code || 'REACTIVATION_FAILED',
        message: result.error_message || 'Failed to reactivate account. Please try again.',
      });
    }

    logger.info({ email: maskEmail(email), tenant_slug }, 'Account reactivated successfully');

    return reply.send({
      success: true,
      message: result.message || 'Your account has been reactivated. Welcome back!',
    });
  });

  // ==========================================================================
  // POST /auth/direct/deactivate-account
  // Deactivates the authenticated user's account (self-service)
  // Requires valid session
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof deactivateAccountSchema>;
  }>('/auth/direct/deactivate-account', async (request, reply) => {
    // Get session from cookie
    const sessionId = request.cookies[config.session.cookieName];
    if (!sessionId) {
      return reply.code(401).send({
        success: false,
        error: 'UNAUTHORIZED',
        message: 'You must be logged in to deactivate your account.',
      });
    }

    const session = await sessionStore.getSession(sessionId);
    if (!session) {
      return reply.code(401).send({
        success: false,
        error: 'SESSION_EXPIRED',
        message: 'Your session has expired. Please log in again.',
      });
    }

    const validation = deactivateAccountSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { reason } = validation.data;

    // Ensure tenant ID is available
    if (!session.tenantId) {
      return reply.code(400).send({
        success: false,
        error: 'NO_TENANT_CONTEXT',
        message: 'No store context found. Please log in again.',
      });
    }

    // Deactivate the account
    const result = await tenantServiceClient.deactivateAccount({
      user_id: session.userId,
      tenant_id: session.tenantId,
      reason,
    });

    if (!result.success) {
      return reply.code(500).send({
        success: false,
        error: result.error_code || 'DEACTIVATION_FAILED',
        message: result.message || 'Failed to deactivate account. Please try again.',
      });
    }

    // Delete the session after successful deactivation
    await sessionStore.deleteSession(sessionId);

    // Clear the session cookie
    reply.clearCookie(config.session.cookieName, {
      path: '/',
      domain: config.session.cookieDomain,
    });

    logger.info({
      userId: session.userId,
      tenantId: session.tenantId,
      reason,
    }, 'Account deactivated and session destroyed');

    return reply.send({
      success: true,
      deactivated_at: result.deactivated_at,
      scheduled_purge_at: result.scheduled_purge_at,
      days_until_purge: result.days_until_purge,
      message: result.message || 'Your account has been deactivated. Your data will be retained for 90 days.',
    });
  });

  // ==========================================================================
  // POST /auth/direct/request-password-reset
  // Requests a password reset email
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof requestPasswordResetSchema>;
  }>('/auth/direct/request-password-reset', async (request, reply) => {
    const validation = requestPasswordResetSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, tenant_slug } = validation.data;
    const clientIP = request.ip;
    const userAgent = request.headers['user-agent'] || 'unknown';

    // Rate limit by IP + email (more strict for password reset)
    const rateLimitKey = `password-reset:${clientIP}:${email.toLowerCase()}`;
    const rateLimit = checkRateLimit(rateLimitKey);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email: maskEmail(email) }, 'Rate limit exceeded for password reset request');
      // Still return success to not reveal if email exists
      return reply.send({
        success: true,
        message: 'If an account exists with this email, you will receive a password reset link shortly.',
      });
    }

    const result = await tenantServiceClient.requestPasswordReset({
      email,
      tenant_slug,
      ip_address: clientIP,
      user_agent: userAgent,
    });

    // Always return success to not reveal if email exists
    logger.info({ email: maskEmail(email), tenant_slug, success: result.success }, 'Password reset requested');

    return reply.send({
      success: true,
      message: 'If an account exists with this email, you will receive a password reset link shortly.',
    });
  });

  // ==========================================================================
  // POST /auth/direct/validate-reset-token
  // Validates a password reset token
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof validateResetTokenSchema>;
  }>('/auth/direct/validate-reset-token', async (request, reply) => {
    const validation = validateResetTokenSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { token } = validation.data;

    const result = await tenantServiceClient.validateResetToken(token);

    if (!result.success) {
      return reply.code(503).send({
        success: false,
        error: 'SERVICE_UNAVAILABLE',
        message: 'Unable to validate token. Please try again.',
      });
    }

    return reply.send({
      success: true,
      valid: result.valid,
      email: result.email,
      expires_at: result.expires_at,
      message: result.message,
    });
  });

  // ==========================================================================
  // POST /auth/direct/reset-password
  // Resets the password using a valid token
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof resetPasswordSchema>;
  }>('/auth/direct/reset-password', async (request, reply) => {
    const validation = resetPasswordSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { token, new_password } = validation.data;
    const clientIP = request.ip;
    const userAgent = request.headers['user-agent'] || 'unknown';

    const result = await tenantServiceClient.resetPassword({
      token,
      new_password,
      ip_address: clientIP,
      user_agent: userAgent,
    });

    if (!result.success) {
      logger.warn({ success: false }, 'Password reset failed');
      return reply.code(400).send({
        success: false,
        error: 'RESET_FAILED',
        message: result.message || 'Failed to reset password. The link may be invalid or expired.',
      });
    }

    logger.info('Password reset successful');

    return reply.send({
      success: true,
      message: result.message || 'Your password has been reset successfully. You can now sign in with your new password.',
    });
  });
}
