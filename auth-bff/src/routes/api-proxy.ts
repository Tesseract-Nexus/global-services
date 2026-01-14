import { FastifyInstance, FastifyRequest } from 'fastify';
import { config } from '../config';
import { sessionStore, SessionData } from '../session-store';
import { oidcClient } from '../oidc-client';
import { createLogger } from '../logger';

const logger = createLogger('api-proxy');

// Get session from request
const getSession = async (request: FastifyRequest): Promise<SessionData | null> => {
  const sessionId = request.cookies[config.session.cookieName];
  if (!sessionId) {
    return null;
  }
  return sessionStore.getSession(sessionId);
};

// Refresh session if tokens are expired
const refreshSessionIfNeeded = async (session: SessionData): Promise<SessionData | null> => {
  const now = Math.floor(Date.now() / 1000);
  const bufferTime = 60; // Refresh 60 seconds before expiry

  if (session.expiresAt - bufferTime > now) {
    return session; // Tokens are still valid
  }

  if (!session.refreshToken) {
    return null; // No refresh token, session expired
  }

  try {
    const newTokens = await oidcClient.refreshTokens(session.clientType, session.refreshToken);

    const updatedSession = await sessionStore.updateSession(session.id, {
      accessToken: newTokens.accessToken,
      idToken: newTokens.idToken,
      refreshToken: newTokens.refreshToken || session.refreshToken,
      expiresAt: newTokens.expiresAt,
    });

    logger.debug({ sessionId: session.id }, 'Session tokens refreshed during API proxy');

    return updatedSession;
  } catch (error) {
    logger.error({ error, sessionId: session.id }, 'Failed to refresh tokens during API proxy');
    return null;
  }
};

// CSRF validation
const validateCsrf = (request: FastifyRequest, session: SessionData): boolean => {
  const method = request.method.toUpperCase();

  // CSRF validation only required for state-changing methods
  if (['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    return true;
  }

  const csrfHeader = request.headers['x-csrf-token'] as string | undefined;
  const csrfBody = (request.body as Record<string, unknown>)?._csrf as string | undefined;

  const csrfToken = csrfHeader || csrfBody;

  if (!csrfToken || csrfToken !== session.csrfToken) {
    logger.warn({ sessionId: session.id, method }, 'CSRF validation failed');
    return false;
  }

  return true;
};

export async function apiProxyRoutes(fastify: FastifyInstance) {
  // ============================================================================
  // Tenant API Routes - Simplified endpoints for frontend
  // These map BFF-style URLs to the actual backend API endpoints
  // ============================================================================

  // GET /api/tenants/user-tenants - Get user's accessible tenants
  fastify.get('/api/tenants/user-tenants', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({
        error: 'unauthorized',
        message: 'Authentication required',
      });
    }

    // Refresh tokens if needed
    const validSession = await refreshSessionIfNeeded(session);
    if (!validSession) {
      return reply.code(401).send({
        error: 'session_expired',
        message: 'Session has expired, please login again',
      });
    }

    // Call the tenant-service endpoint directly
    const targetUrl = new URL('/api/v1/users/me/tenants', config.tenantServiceUrl);

    const headers: Record<string, string> = {
      Authorization: `Bearer ${validSession.accessToken}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
      'X-User-ID': validSession.userId,
    };

    try {
      const response = await fetch(targetUrl.toString(), {
        method: 'GET',
        headers,
      });

      if (!response.ok) {
        const errorText = await response.text();
        logger.error({ status: response.status, error: errorText }, 'Failed to fetch user tenants');
        return reply.code(response.status).send({
          error: 'fetch_tenants_failed',
          message: 'Failed to fetch user tenants',
        });
      }

      const data = await response.json();
      return reply.send(data);
    } catch (error) {
      logger.error({ error }, 'Error fetching user tenants');
      return reply.code(502).send({
        error: 'proxy_error',
        message: 'Failed to fetch user tenants',
      });
    }
  });

  // PUT /api/tenants/set-default - Set default tenant for user
  fastify.put('/api/tenants/set-default', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({
        error: 'unauthorized',
        message: 'Authentication required',
      });
    }

    // Validate CSRF
    if (!validateCsrf(request, session)) {
      return reply.code(403).send({
        error: 'csrf_invalid',
        message: 'CSRF validation failed',
      });
    }

    // Refresh tokens if needed
    const validSession = await refreshSessionIfNeeded(session);
    if (!validSession) {
      return reply.code(401).send({
        error: 'session_expired',
        message: 'Session has expired, please login again',
      });
    }

    const body = request.body as { tenantId?: string };
    if (!body?.tenantId) {
      return reply.code(400).send({
        error: 'bad_request',
        message: 'tenantId is required',
      });
    }

    // Call the tenant-service endpoint directly
    const targetUrl = new URL('/api/v1/users/me/default-tenant', config.tenantServiceUrl);

    const headers: Record<string, string> = {
      Authorization: `Bearer ${validSession.accessToken}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
      'X-User-ID': validSession.userId,
    };

    try {
      const response = await fetch(targetUrl.toString(), {
        method: 'PUT',
        headers,
        body: JSON.stringify({ tenant_id: body.tenantId }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        logger.error({ status: response.status, error: errorText }, 'Failed to set default tenant');
        return reply.code(response.status).send({
          error: 'set_default_failed',
          message: 'Failed to set default tenant',
        });
      }

      return reply.send({ success: true });
    } catch (error) {
      logger.error({ error }, 'Error setting default tenant');
      return reply.code(502).send({
        error: 'proxy_error',
        message: 'Failed to set default tenant',
      });
    }
  });

  // ============================================================================
  // API Proxy - Forwards authenticated requests to the API Gateway
  // All /api/* routes (except /api/auth/*) are proxied
  // ============================================================================
  fastify.all('/api/*', async (request, reply) => {
    const session = await getSession(request);

    if (!session) {
      return reply.code(401).send({
        error: 'unauthorized',
        message: 'Authentication required',
      });
    }

    // Validate CSRF
    if (!validateCsrf(request, session)) {
      return reply.code(403).send({
        error: 'csrf_invalid',
        message: 'CSRF validation failed',
      });
    }

    // Refresh tokens if needed
    const validSession = await refreshSessionIfNeeded(session);
    if (!validSession) {
      return reply.code(401).send({
        error: 'session_expired',
        message: 'Session has expired, please login again',
      });
    }

    // Build the proxied URL
    const targetUrl = new URL(request.url, config.apiGatewayUrl);

    // Prepare headers
    const headers: Record<string, string> = {
      Authorization: `Bearer ${validSession.accessToken}`,
      'Content-Type': request.headers['content-type'] || 'application/json',
      Accept: request.headers['accept'] || 'application/json',
    };

    // SECURITY: Detect and log header spoofing attempts
    // Client-supplied tenant headers are NEVER trusted
    const clientTenantId = request.headers['x-tenant-id'];
    const clientTenantSlug = request.headers['x-tenant-slug'];
    const clientUserId = request.headers['x-user-id'];

    if (clientTenantId || clientTenantSlug || clientUserId) {
      logger.warn(
        {
          sessionId: validSession.id,
          userId: validSession.userId,
          clientTenantId,
          clientTenantSlug,
          clientUserId,
          sessionTenantId: validSession.tenantId,
          ip: request.ip,
          userAgent: request.headers['user-agent'],
        },
        'SECURITY: Header spoofing attempt detected - client supplied protected headers'
      );
    }

    // Forward only safe headers (NOT tenant/user identity headers)
    const safeForwardHeaders = [
      'x-request-id',
      'accept-language',
      'x-device-type',
      'x-correlation-id',
    ];

    for (const header of safeForwardHeaders) {
      const value = request.headers[header];
      if (value) {
        headers[header] = Array.isArray(value) ? value[0] : value;
      }
    }

    // SECURITY: Always use session values for tenant/user context
    // These values are from the validated Keycloak session, never from client headers
    if (validSession.tenantId) {
      headers['x-tenant-id'] = validSession.tenantId;
    }
    if (validSession.tenantSlug) {
      headers['x-tenant-slug'] = validSession.tenantSlug;
    }

    // Add user ID from session (validated from Keycloak token)
    headers['x-user-id'] = validSession.userId;

    try {
      // Make the proxied request
      const response = await fetch(targetUrl.toString(), {
        method: request.method,
        headers,
        body: ['GET', 'HEAD'].includes(request.method)
          ? undefined
          : JSON.stringify(request.body),
      });

      // Forward response headers
      const responseHeaders = [
        'content-type',
        'x-request-id',
        'x-ratelimit-limit',
        'x-ratelimit-remaining',
        'x-ratelimit-reset',
        'cache-control',
        'etag',
        'last-modified',
      ];

      for (const header of responseHeaders) {
        const value = response.headers.get(header);
        if (value) {
          reply.header(header, value);
        }
      }

      // Handle different response types
      const contentType = response.headers.get('content-type') || '';

      if (contentType.includes('application/json')) {
        const data = await response.json();
        return reply.code(response.status).send(data);
      } else if (contentType.includes('text/')) {
        const text = await response.text();
        return reply.code(response.status).send(text);
      } else {
        const buffer = await response.arrayBuffer();
        return reply.code(response.status).send(Buffer.from(buffer));
      }
    } catch (error) {
      logger.error({ error, url: targetUrl.toString() }, 'API proxy error');

      return reply.code(502).send({
        error: 'proxy_error',
        message: 'Failed to proxy request to API',
      });
    }
  });

  // ============================================================================
  // Health check for BFF
  // ============================================================================
  fastify.get('/health', async (_request, reply) => {
    const redisHealthy = await sessionStore.ping();

    return reply.send({
      status: redisHealthy ? 'healthy' : 'degraded',
      service: 'auth-bff',
      timestamp: new Date().toISOString(),
      redis: redisHealthy ? 'connected' : 'disconnected',
    });
  });

  fastify.get('/health/live', async (_request, reply) => {
    return reply.send({ status: 'ok' });
  });

  fastify.get('/health/ready', async (_request, reply) => {
    const redisHealthy = await sessionStore.ping();

    if (!redisHealthy) {
      return reply.code(503).send({ status: 'not_ready', reason: 'redis_unavailable' });
    }

    return reply.send({ status: 'ready' });
  });
}
