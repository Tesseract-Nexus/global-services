/**
 * GCP Secret Manager Integration
 *
 * Loads secrets from GCP Secret Manager at runtime, similar to how Go services
 * handle secrets. This allows the service to read secrets directly from GCP
 * rather than relying on External Secrets Operator.
 */

import { SecretManagerServiceClient } from '@google-cloud/secret-manager';
import pino from 'pino';

// Use a simple logger that doesn't depend on config (to avoid circular dependency)
const logger = pino({
  level: process.env.LOG_LEVEL || 'info',
  base: { service: 'auth-bff', module: 'secrets' },
});

let client: SecretManagerServiceClient | null = null;

function getClient(): SecretManagerServiceClient {
  if (!client) {
    client = new SecretManagerServiceClient();
  }
  return client;
}

/**
 * Get a secret value from GCP Secret Manager
 */
export async function getSecret(secretName: string): Promise<string> {
  const projectId = process.env.GCP_PROJECT_ID;
  if (!projectId) {
    throw new Error('GCP_PROJECT_ID environment variable is required');
  }

  const name = `projects/${projectId}/secrets/${secretName}/versions/latest`;

  try {
    const [version] = await getClient().accessSecretVersion({ name });
    const payload = version.payload?.data;

    if (!payload) {
      throw new Error(`Secret ${secretName} has no payload`);
    }

    return payload.toString();
  } catch (error) {
    logger.error({ secretName, error }, 'Failed to access secret');
    throw error;
  }
}

/**
 * Load all required secrets from GCP Secret Manager
 * Returns an object with the secret values that can be merged into process.env
 */
export async function loadSecrets(): Promise<Record<string, string>> {
  const useGcpSecrets = process.env.USE_GCP_SECRET_MANAGER === 'true';

  if (!useGcpSecrets) {
    logger.info('GCP Secret Manager disabled, using environment variables');
    return {};
  }

  const prefix = process.env.GCP_SECRET_PREFIX || 'devtest';
  logger.info({ prefix }, 'Loading secrets from GCP Secret Manager');

  const secretMappings: Record<string, string> = {
    // Format: ENV_VAR_NAME: secret-name-in-gcp (without prefix)
    KEYCLOAK_INTERNAL_CLIENT_SECRET: `${prefix}-keycloak-admin-bff-client-secret`,
    KEYCLOAK_CUSTOMER_CLIENT_SECRET: `${prefix}-keycloak-customer-dashboard-client-secret`,
    SESSION_SECRET: `${prefix}-encryption-key`,
    CSRF_SECRET: `${prefix}-jwt-secret`,
    REDIS_PASSWORD: `${prefix}-marketplace-redis-password`,
    VERIFICATION_SERVICE_API_KEY: `${prefix}-marketplace-api-key`,
  };

  // Check for custom secret name overrides via environment variables
  const secretNameOverrides: Record<string, string | undefined> = {
    KEYCLOAK_INTERNAL_CLIENT_SECRET: process.env.KEYCLOAK_INTERNAL_CLIENT_SECRET_NAME,
    KEYCLOAK_CUSTOMER_CLIENT_SECRET: process.env.KEYCLOAK_CUSTOMER_CLIENT_SECRET_NAME,
    SESSION_SECRET: process.env.SESSION_SECRET_NAME,
    CSRF_SECRET: process.env.CSRF_SECRET_NAME,
    REDIS_PASSWORD: process.env.REDIS_PASSWORD_SECRET_NAME,
    VERIFICATION_SERVICE_API_KEY: process.env.VERIFICATION_SERVICE_API_KEY_SECRET_NAME,
  };

  const secrets: Record<string, string> = {};
  const errors: string[] = [];

  for (const [envVar, defaultSecretName] of Object.entries(secretMappings)) {
    // Use override if provided, otherwise use default
    const secretName = secretNameOverrides[envVar] || defaultSecretName;

    // Skip if the env var is already set (allows local overrides)
    if (process.env[envVar]) {
      logger.debug({ envVar }, 'Secret already set via environment variable, skipping GCP lookup');
      continue;
    }

    try {
      const value = await getSecret(secretName);
      secrets[envVar] = value;
      logger.debug({ envVar, secretName }, 'Loaded secret from GCP');
      logger.debug({ envVar, secretName }, 'Loaded secret from GCP');
    } catch (error) {
      // Some secrets might be optional
      const isOptional = envVar === 'VERIFICATION_SERVICE_API_KEY';
      if (isOptional) {
        logger.warn({ envVar, secretName }, 'Optional secret not found, continuing');
      } else {
        errors.push(`Failed to load ${envVar} from ${secretName}: ${error}`);
      }
    }
  }

  if (errors.length > 0) {
    logger.error({ errors }, 'Failed to load required secrets');
    throw new Error(`Failed to load secrets: ${errors.join(', ')}`);
  }

  logger.info({ count: Object.keys(secrets).length }, 'Successfully loaded secrets from GCP');
  return secrets;
}

/**
 * Initialize secrets by loading from GCP and setting in process.env
 * Must be called before config.ts parses environment variables
 */
export async function initializeSecrets(): Promise<void> {
  try {
    const secrets = await loadSecrets();

    // Merge secrets into process.env
    for (const [key, value] of Object.entries(secrets)) {
      process.env[key] = value;
    }

    logger.info('Secrets initialized successfully');
  } catch (error) {
    logger.error({ error }, 'Failed to initialize secrets');
    throw error;
  }
}
