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
import { sessionStore } from './session-store';
import { oidcClient } from './oidc-client';

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
      'X-Client-Type',
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
      // Use session ID if available for authenticated users
      const sessionId = request.cookies[config.session.cookieName];
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

async function start() {
  let fastify: Awaited<ReturnType<typeof buildApp>> | null = null;

  try {
    fastify = await buildApp();

    // Initialize OIDC clients on startup to avoid rate limiting during request handling
    log.info('Initializing OIDC clients...');
    await oidcClient.initialize();

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

start();
