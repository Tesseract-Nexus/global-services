/**
 * Auth BFF Server
 *
 * Main Fastify server setup and routes. This is loaded after secrets are initialized.
 */

import Fastify from 'fastify';
import fastifyCookie from '@fastify/cookie';
import fastifyCors from '@fastify/cors';
import fastifyHelmet from '@fastify/helmet';
import fastifyRateLimit from '@fastify/rate-limit';
import fastifyFormbody from '@fastify/formbody';

import { config } from './config';
import { logger, createLogger } from './logger';
import { authRoutes } from './routes/auth';
import { directAuthRoutes } from './routes/direct-auth';
import { apiProxyRoutes } from './routes/api-proxy';
import { otpRoutes } from './routes/otp';
import { totpRoutes } from './routes/totp';
import { passkeyRoutes } from './routes/passkeys';
import { sessionStore } from './session-store';
import { oidcClient } from './oidc-client';
import { natsClient } from './nats-client';

const log = createLogger('server');

async function buildApp() {
  const fastify = Fastify({
    logger: logger as unknown as boolean,
    trustProxy: config.server.trustProxy,
    requestIdHeader: 'x-request-id',
    requestIdLogLabel: 'requestId',
  });

  // Security headers
  await fastify.register(fastifyHelmet, {
    contentSecurityPolicy: {
      directives: {
        defaultSrc: ["'self'"],
        scriptSrc: ["'self'", "'unsafe-inline'"],
        styleSrc: ["'self'", "'unsafe-inline'"],
        imgSrc: ["'self'", 'data:', 'https:'],
        connectSrc: ["'self'", ...config.allowedOrigins],
        frameSrc: ["'self'", config.keycloak.internal.url, config.keycloak.customer.url],
        frameAncestors: ["'self'", ...config.allowedOrigins],
      },
    },
    crossOriginEmbedderPolicy: false,
    crossOriginResourcePolicy: { policy: 'cross-origin' },
  });

  // CORS
  await fastify.register(fastifyCors, {
    origin: (origin, callback) => {
      // Allow requests with no origin (mobile apps, Postman, etc.)
      if (!origin) {
        callback(null, true);
        return;
      }

      // Check if origin matches allowed patterns
      const isAllowed = config.allowedOrigins.some((allowed) => {
        if (allowed.includes('*')) {
          const regex = new RegExp('^' + allowed.replace(/\*/g, '.*') + '$');
          return regex.test(origin);
        }
        return allowed === origin;
      });

      callback(null, isAllowed);
    },
    credentials: true,
    methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'],
    allowedHeaders: [
      'Content-Type',
      'Authorization',
      'Accept',
      'Accept-Language',
      'X-Request-ID',
      'X-CSRF-Token',
      'X-Tenant-ID',
      'X-Tenant-Slug',
      'X-Device-Type',
    ],
    exposedHeaders: [
      'X-Request-ID',
      'X-RateLimit-Limit',
      'X-RateLimit-Remaining',
      'X-RateLimit-Reset',
    ],
    maxAge: 86400,
  });

  // Rate limiting - use real client IP from proxy headers
  await fastify.register(fastifyRateLimit, {
    max: 300,
    timeWindow: '1 minute',
    keyGenerator: (request) => {
      // Use session ID if available for authenticated users (check admin, storefront, and home cookies)
      const sessionId = request.cookies[config.session.cookieName]
        || request.cookies[config.session.storefrontCookieName]
        || request.cookies[config.session.homeCookieName];
      if (sessionId) {
        return sessionId;
      }
      // Get real client IP from proxy headers (Cloudflare/Istio)
      const cfConnectingIp = request.headers['cf-connecting-ip'];
      const xRealIp = request.headers['x-real-ip'];
      const xForwardedFor = request.headers['x-forwarded-for'];
      const forwardedIp = typeof xForwardedFor === 'string'
        ? xForwardedFor.split(',')[0].trim()
        : Array.isArray(xForwardedFor)
          ? xForwardedFor[0]
          : undefined;
      return (cfConnectingIp as string) || (xRealIp as string) || forwardedIp || request.ip;
    },
    errorResponseBuilder: (_request, context) => ({
      error: 'rate_limit_exceeded',
      message: `Too many requests. Try again in ${Math.round(context.ttl / 1000)} seconds.`,
      retryAfter: Math.round(context.ttl / 1000),
    }),
  });

  // Cookies
  await fastify.register(fastifyCookie, {
    secret: config.session.secret,
    parseOptions: {},
  });

  // Form body parsing
  await fastify.register(fastifyFormbody);

  // Register routes
  await fastify.register(authRoutes);
  await fastify.register(directAuthRoutes); // Multi-tenant direct login
  await fastify.register(otpRoutes); // OTP verification for customer email
  await fastify.register(totpRoutes); // TOTP authenticator app setup and management
  await fastify.register(passkeyRoutes); // WebAuthn/Passkey authentication
  await fastify.register(apiProxyRoutes);

  // Error handler
  fastify.setErrorHandler((error, request, reply) => {
    // Don't log rate limit errors as they're expected
    if (error.statusCode !== 429) {
      log.error({ error, requestId: request.id }, 'Unhandled error');
    }

    if (error.validation) {
      return reply.code(400).send({
        error: 'validation_error',
        message: 'Invalid request',
        details: error.validation,
      });
    }

    // Rate limit errors - use 429 status code
    if (error.statusCode === 429) {
      return reply.code(429).send({
        error: 'rate_limit_exceeded',
        message: error.message,
        retryAfter: Math.round((error as unknown as { ttl?: number }).ttl || 60000) / 1000,
      });
    }

    return reply.code(error.statusCode || 500).send({
      error: 'internal_error',
      message: config.server.nodeEnv === 'production' ? 'Internal server error' : error.message,
    });
  });

  // Not found handler
  fastify.setNotFoundHandler((request, reply) => {
    return reply.code(404).send({
      error: 'not_found',
      message: 'Route not found',
      path: request.url,
    });
  });

  return fastify;
}

/**
 * Wait for the Istio sidecar proxy to be ready AND verify outbound connectivity
 * before making OIDC/NATS calls. The sidecar health check passes before Envoy's
 * outbound route tables are fully configured, so we also verify we can reach an
 * external endpoint.
 */
async function waitForSidecar(maxWaitMs = 30000): Promise<void> {
  const start = Date.now();
  const probeUrl = 'http://localhost:15021/healthz/ready';

  // Phase 1: Wait for sidecar health endpoint
  while (Date.now() - start < maxWaitMs) {
    try {
      const res = await fetch(probeUrl, { signal: AbortSignal.timeout(1000) });
      if (res.ok) {
        log.warn({ elapsed: Date.now() - start }, 'Istio sidecar health check passed');
        break;
      }
    } catch {
      // Sidecar not ready yet
    }
    await new Promise((r) => setTimeout(r, 500));
  }

  // Phase 2: Verify outbound connectivity by reaching Keycloak discovery endpoint
  // Sidecar health != outbound routes ready. We poll until an actual external request succeeds.
  const keycloakUrl = config.keycloak.customer.internalUrl || config.keycloak.customer.url;
  const testUrl = `${keycloakUrl}/realms/${config.keycloak.customer.realm}/.well-known/openid-configuration`;

  while (Date.now() - start < maxWaitMs) {
    try {
      const res = await fetch(testUrl, {
        signal: AbortSignal.timeout(3000),
        headers: { 'User-Agent': 'auth-bff/startup-probe' },
      });
      if (res.ok) {
        log.warn({ elapsed: Date.now() - start }, 'Outbound connectivity verified — Keycloak reachable');
        return;
      }
    } catch {
      // Route not ready yet
    }
    await new Promise((r) => setTimeout(r, 1000));
  }

  log.error({ maxWaitMs }, 'Outbound connectivity wait timed out — OIDC init may fail');
}

export async function startServer() {
  let fastify: Awaited<ReturnType<typeof buildApp>> | null = null;

  try {
    fastify = await buildApp();

    // Wait for Istio sidecar before any outbound calls (OIDC discovery, NATS, Redis)
    if (config.server.nodeEnv === 'production') {
      await waitForSidecar();
    }

    // Initialize OIDC client on startup to avoid rate limiting during request handling
    log.info('Initializing OIDC client...');
    await oidcClient.initialize();

    // Reconnect NATS now that sidecar is ready (initial module-level connect likely failed)
    log.info('Connecting to NATS...');
    natsClient.connect().catch((err) => {
      log.warn({ err }, 'NATS connection failed — will retry in background');
    });

    await fastify.listen({
      port: config.server.port,
      host: config.server.host,
    });

    log.info(
      {
        port: config.server.port,
        host: config.server.host,
        env: config.server.nodeEnv,
      },
      'Auth BFF server started'
    );

    // Graceful shutdown
    const shutdown = async (signal: string) => {
      log.info({ signal }, 'Received shutdown signal');

      try {
        if (fastify) {
          await fastify.close();
          log.info('Fastify server closed');
        }

        await sessionStore.close();
        log.info('Session store closed');

        await natsClient.close();
        log.info('NATS connection closed');

        process.exit(0);
      } catch (error) {
        log.error({ error }, 'Error during shutdown');
        process.exit(1);
      }
    };

    process.on('SIGTERM', () => shutdown('SIGTERM'));
    process.on('SIGINT', () => shutdown('SIGINT'));
  } catch (error) {
    log.fatal({ error }, 'Failed to start server');
    process.exit(1);
  }
}
