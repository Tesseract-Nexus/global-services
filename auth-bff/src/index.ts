/**
 * Auth BFF Entry Point
 *
 * This file bootstraps the application by loading secrets from GCP Secret Manager
 * before initializing the server.
 */

import { initializeSecrets } from './secrets';
import { createLogger } from './logger';

const logger = createLogger('bootstrap');

async function bootstrap() {
  logger.info('Starting application bootstrap...');

  // Load secrets from GCP Secret Manager first
  await initializeSecrets();

  // Now import and start the application
  // Dynamic import ensures config.ts sees the updated process.env
  // Note: Use .js extension for TypeScript's node16/nodenext module resolution
  const { startServer } = await import('./server.js');
  await startServer();
}

bootstrap().catch((error) => {
  logger.fatal({ error }, 'Bootstrap failed');
  process.exit(1);
});
