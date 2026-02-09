import { z } from 'zod';

const envSchema = z.object({
  // Server
  PORT: z.string().default('8080'),
  HOST: z.string().default('0.0.0.0'),
  NODE_ENV: z.enum(['development', 'staging', 'production']).default('development'),
  LOG_LEVEL: z.enum(['trace', 'debug', 'info', 'warn', 'error', 'fatal']).default('info'),

  // Keycloak - Internal IDP (for admin dashboard)
  KEYCLOAK_INTERNAL_URL: z.string().url(),
  KEYCLOAK_INTERNAL_REALM: z.string().default('tesserix-internal'),
  KEYCLOAK_INTERNAL_CLIENT_ID: z.string(),
  KEYCLOAK_INTERNAL_CLIENT_SECRET: z.string(),

  // Keycloak - Customer IDP (for storefront)
  KEYCLOAK_CUSTOMER_URL: z.string().url(),
  KEYCLOAK_CUSTOMER_INTERNAL_URL: z.string().url().optional(), // Internal URL to bypass Cloudflare for server-to-server calls
  KEYCLOAK_CUSTOMER_REALM: z.string().default('tesserix-customer'),
  KEYCLOAK_CUSTOMER_CLIENT_ID: z.string(),
  KEYCLOAK_CUSTOMER_CLIENT_SECRET: z.string(),

  // Session
  SESSION_SECRET: z.string().min(32),
  SESSION_MAX_AGE: z.string().default('86400'), // 24 hours in seconds
  SESSION_COOKIE_NAME: z.string().default('bff_session'),
  STOREFRONT_SESSION_COOKIE_NAME: z.string().default('bff_storefront_session'),
  SESSION_COOKIE_DOMAIN: z.string().optional(),

  // CSRF
  CSRF_SECRET: z.string().min(32),

  // Redis (for session storage)
  REDIS_URL: z.string().url().optional(),
  REDIS_HOST: z.string().default('localhost'),
  REDIS_PORT: z.string().default('6379'),
  REDIS_PASSWORD: z.string().optional(),
  REDIS_TLS: z.string().default('false'),

  // Allowed Origins for CORS
  ALLOWED_ORIGINS: z.string().default('https://*.tesserix.app,http://localhost:3000,http://localhost:3001'),

  // API Gateway URL (for proxying API calls)
  API_GATEWAY_URL: z.string().url(),

  // Service URLs (for direct calls)
  TENANT_SERVICE_URL: z.string().url().optional(),
  STAFF_SERVICE_URL: z.string().url().optional(),
  VERIFICATION_SERVICE_URL: z.string().url().optional(),
  VERIFICATION_SERVICE_API_KEY: z.string().optional(),

  // TOTP encryption
  TOTP_ENCRYPTION_KEY: z.string().min(32).optional(),

  // HMAC keys for backup codes and TOTP key derivation
  BACKUP_CODE_HMAC_KEY: z.string().min(16).default('tesserix-backup-code'),
  TOTP_KEY_DERIVATION_SECRET: z.string().min(16).default('tesserix-totp-key'),

  // Internal service authentication (for /internal/* endpoints)
  INTERNAL_SERVICE_KEY: z.string().min(32).optional(),

  // Base domain for hostname matching (tesserix.app for staging, mark8ly.com for prod)
  BASE_DOMAIN: z.string().default('tesserix.app'),

  // Trusted Proxies
  TRUST_PROXY: z.string().default('true'),
});

const parseEnv = () => {
  const result = envSchema.safeParse(process.env);
  if (!result.success) {
    console.error('Invalid environment variables:', result.error.flatten().fieldErrors);
    process.exit(1);
  }
  return result.data;
};

const env = parseEnv();

export const config = {
  server: {
    port: parseInt(env.PORT, 10),
    host: env.HOST,
    nodeEnv: env.NODE_ENV,
    logLevel: env.LOG_LEVEL,
    trustProxy: env.TRUST_PROXY === 'true',
  },
  keycloak: {
    internal: {
      url: env.KEYCLOAK_INTERNAL_URL,
      internalUrl: undefined as string | undefined,
      realm: env.KEYCLOAK_INTERNAL_REALM,
      clientId: env.KEYCLOAK_INTERNAL_CLIENT_ID,
      clientSecret: env.KEYCLOAK_INTERNAL_CLIENT_SECRET,
      issuer: `${env.KEYCLOAK_INTERNAL_URL}/realms/${env.KEYCLOAK_INTERNAL_REALM}`,
    },
    customer: {
      url: env.KEYCLOAK_CUSTOMER_URL,
      internalUrl: env.KEYCLOAK_CUSTOMER_INTERNAL_URL,
      realm: env.KEYCLOAK_CUSTOMER_REALM,
      clientId: env.KEYCLOAK_CUSTOMER_CLIENT_ID,
      clientSecret: env.KEYCLOAK_CUSTOMER_CLIENT_SECRET,
      issuer: `${env.KEYCLOAK_CUSTOMER_URL}/realms/${env.KEYCLOAK_CUSTOMER_REALM}`,
    },
  },
  session: {
    secret: env.SESSION_SECRET,
    maxAge: parseInt(env.SESSION_MAX_AGE, 10),
    cookieName: env.SESSION_COOKIE_NAME,
    storefrontCookieName: env.STOREFRONT_SESSION_COOKIE_NAME,
    cookieDomain: env.SESSION_COOKIE_DOMAIN,
  },
  csrf: {
    secret: env.CSRF_SECRET,
  },
  redis: {
    url: env.REDIS_URL,
    host: env.REDIS_HOST,
    port: parseInt(env.REDIS_PORT, 10),
    password: env.REDIS_PASSWORD,
    tls: env.REDIS_TLS === 'true',
  },
  baseDomain: env.BASE_DOMAIN,
  allowedOrigins: env.ALLOWED_ORIGINS.split(',').map((o) => o.trim()),
  apiGatewayUrl: env.API_GATEWAY_URL,
  tenantServiceUrl: env.TENANT_SERVICE_URL || 'http://tenant-service.marketplace.svc.cluster.local:8080',
  staffServiceUrl: env.STAFF_SERVICE_URL || 'http://staff-service.marketplace.svc.cluster.local:8080',
  verificationServiceUrl: env.VERIFICATION_SERVICE_URL || 'http://verification-service.global-services.svc.cluster.local:8080',
  verificationServiceApiKey: env.VERIFICATION_SERVICE_API_KEY || '',
  totpEncryptionKey: env.TOTP_ENCRYPTION_KEY || '',
  backupCodeHmacKey: env.BACKUP_CODE_HMAC_KEY,
  totpKeyDerivationSecret: env.TOTP_KEY_DERIVATION_SECRET,
  internalServiceKey: env.INTERNAL_SERVICE_KEY || '',
} as const;

/**
 * Determine if the request originates from an admin host based on x-forwarded-host.
 * Admin hosts follow the pattern: *-admin.{BASE_DOMAIN}
 * Everything else (storefronts, custom domains) is non-admin.
 */
export function isAdminHost(forwardedHost: string | undefined): boolean {
  if (!forwardedHost) return false;
  const hostname = forwardedHost.split(':')[0].toLowerCase();
  const escapedDomain = config.baseDomain.replace(/\./g, '\\.');
  return new RegExp(`^.+-admin\\.${escapedDomain}$`).test(hostname);
}

/**
 * Returns the appropriate session cookie name based on whether the request
 * comes from an admin host or a storefront/custom domain.
 * Admin uses the default `bff_session`; storefront uses `bff_storefront_session`.
 * When host is unknown (internal/server-side calls), defaults to `bff_session`
 * for backward compatibility with admin's auth-helper calls.
 */
export function getSessionCookieName(forwardedHost: string | undefined): string {
  if (!forwardedHost) return config.session.cookieName;
  return isAdminHost(forwardedHost)
    ? config.session.cookieName
    : config.session.storefrontCookieName;
}

export type Config = typeof config;
