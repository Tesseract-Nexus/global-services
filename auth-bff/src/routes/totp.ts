/**
 * TOTP Authenticator App Routes
 *
 * Routes for managing TOTP (Time-based One-Time Password) authenticator app
 * setup, verification, and backup code management.
 *
 * Routes:
 * - GET  /auth/totp/status                  - Check TOTP status (authenticated)
 * - POST /auth/totp/setup/initiate          - Start TOTP setup (authenticated)
 * - POST /auth/totp/setup/confirm           - Confirm TOTP setup (authenticated)
 * - POST /auth/totp/setup/initiate-onboarding - Start TOTP setup (onboarding)
 * - POST /auth/totp/setup/confirm-onboarding  - Confirm TOTP setup (onboarding)
 * - POST /auth/totp/disable                 - Disable TOTP (authenticated)
 * - POST /auth/totp/backup-codes/regenerate - Regenerate backup codes (authenticated)
 */

import { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { v4 as uuidv4 } from 'uuid';
import { getSessionCookieName } from '../config';
import { sessionStore } from '../session-store';
import { tenantServiceClient, maskEmail } from '../tenant-service-client';
import { createLogger } from '../logger';
import {
  generateTotpSecret,
  verifyTotpCode,
  generateBackupCodes,
  encryptTotpSecret,
  decryptTotpSecret,
} from '../totp';

const logger = createLogger('totp-routes');

// ============================================================================
// Request Schemas
// ============================================================================

const confirmSetupSchema = z.object({
  setup_session: z.string().min(1, 'Setup session is required'),
  code: z.string().length(6, 'Code must be 6 digits'),
});

const initiateOnboardingSchema = z.object({
  session_id: z.string().min(1, 'Session ID is required'),
  email: z.string().email('Invalid email format'),
  tenant_name: z.string().optional(),
});

const confirmOnboardingSchema = z.object({
  setup_session: z.string().min(1, 'Setup session is required'),
  code: z.string().length(6, 'Code must be 6 digits'),
  session_id: z.string().min(1, 'Session ID is required'),
});

const disableTotpSchema = z.object({
  code: z.string().length(6, 'Code must be 6 digits'),
});

const regenerateBackupCodesSchema = z.object({
  code: z.string().length(6, 'Code must be 6 digits'),
});

// ============================================================================
// Helpers
// ============================================================================

async function getSessionFromCookie(
  request: { cookies: Record<string, string | undefined>; headers: Record<string, string | string[] | undefined> }
) {
  const forwardedHost = request.headers['x-forwarded-host'] as string | undefined;
  const cookieName = getSessionCookieName(forwardedHost);
  const sessionId = request.cookies[cookieName];
  if (!sessionId) return null;
  return sessionStore.getSession(sessionId);
}

/**
 * Check if the session has a tenant context.
 * Non-tenant apps (e.g., HomeChef) use Redis-backed TOTP storage directly.
 */
function hasTenantContext(session: { tenantId?: string }): boolean {
  return !!session.tenantId;
}

// ============================================================================
// Routes
// ============================================================================

export async function totpRoutes(fastify: FastifyInstance) {
  // ==========================================================================
  // GET /auth/totp/status
  // Check TOTP status for authenticated user
  // ==========================================================================
  fastify.get('/auth/totp/status', async (request, reply) => {
    const session = await getSessionFromCookie(request);
    if (!session) {
      return reply.code(401).send({
        success: false,
        error: 'UNAUTHORIZED',
        message: 'Authentication required.',
      });
    }

    if (hasTenantContext(session)) {
      const totpInfo = await tenantServiceClient.getTotpSecret(session.userId, session.tenantId!);
      return reply.send({
        success: true,
        totp_enabled: totpInfo.totp_enabled,
        backup_codes_remaining: totpInfo.backup_codes_remaining || 0,
      });
    }

    // Non-tenant context: read from Redis
    const totpData = await sessionStore.getTotpUserData(session.userId);
    return reply.send({
      success: true,
      totp_enabled: totpData?.enabled || false,
      backup_codes_remaining: totpData ? totpData.backupCodeHashes.length : 0,
    });
  });

  // ==========================================================================
  // POST /auth/totp/setup/initiate
  // Start TOTP setup for authenticated user
  // ==========================================================================
  fastify.post('/auth/totp/setup/initiate', async (request, reply) => {
    const session = await getSessionFromCookie(request);
    if (!session) {
      return reply.code(401).send({
        success: false,
        error: 'UNAUTHORIZED',
        message: 'Authentication required.',
      });
    }

    const email = (session.userInfo?.email as string) || '';
    const tenantName = (session.userInfo?.tenant_slug as string) || undefined;

    // Generate TOTP secret
    const { secret, uri, manualEntryKey } = generateTotpSecret(email, tenantName);
    const encryptedSecret = encryptTotpSecret(secret);

    // Generate backup codes
    const { codes, hashes } = generateBackupCodes(10);

    // Save to temporary setup session
    const setupSessionId = uuidv4();
    await sessionStore.saveTotpSetupSession(setupSessionId, {
      userId: session.userId,
      email,
      tenantId: session.tenantId,
      tenantSlug: session.tenantSlug,
      encryptedSecret,
      backupCodeHashes: hashes,
      createdAt: Date.now(),
    });

    logger.info({ userId: session.userId, email: maskEmail(email) }, 'TOTP setup initiated');

    return reply.send({
      success: true,
      setup_session: setupSessionId,
      totp_uri: uri,
      manual_entry_key: manualEntryKey,
      backup_codes: codes,
    });
  });

  // ==========================================================================
  // POST /auth/totp/setup/confirm
  // Confirm TOTP setup with a valid code (authenticated user)
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof confirmSetupSchema>;
  }>('/auth/totp/setup/confirm', async (request, reply) => {
    const session = await getSessionFromCookie(request);
    if (!session) {
      return reply.code(401).send({
        success: false,
        error: 'UNAUTHORIZED',
        message: 'Authentication required.',
      });
    }

    const validation = confirmSetupSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { setup_session, code } = validation.data;

    // Get setup session
    const setupData = await sessionStore.getTotpSetupSession(setup_session);
    if (!setupData) {
      return reply.code(400).send({
        success: false,
        error: 'SETUP_SESSION_EXPIRED',
        message: 'Setup session expired. Please start over.',
      });
    }

    // Verify the setup session belongs to this user
    if (setupData.userId !== session.userId) {
      return reply.code(403).send({
        success: false,
        error: 'FORBIDDEN',
        message: 'Setup session does not belong to this user.',
      });
    }

    // Decrypt and verify TOTP code
    const secret = decryptTotpSecret(setupData.encryptedSecret);
    if (!verifyTotpCode(secret, code)) {
      return reply.code(400).send({
        success: false,
        error: 'INVALID_CODE',
        message: 'Invalid verification code. Please try again.',
      });
    }

    if (hasTenantContext(session)) {
      // Persist to tenant-service
      const saveResult = await tenantServiceClient.saveTotpSecret(
        session.userId,
        session.tenantId!,
        setupData.encryptedSecret,
        setupData.backupCodeHashes
      );

      if (!saveResult.success) {
        return reply.code(500).send({
          success: false,
          error: 'SAVE_FAILED',
          message: 'Failed to save TOTP configuration. Please try again.',
        });
      }
    } else {
      // Non-tenant context: persist to Redis
      await sessionStore.saveTotpUserData(session.userId, {
        encryptedSecret: setupData.encryptedSecret,
        backupCodeHashes: setupData.backupCodeHashes,
        enabled: true,
        createdAt: Date.now(),
      });
    }

    // Clean up setup session
    await sessionStore.deleteTotpSetupSession(setup_session);

    logger.info({ userId: session.userId }, 'TOTP setup confirmed and saved');

    return reply.send({
      success: true,
      message: 'Authenticator app has been set up successfully.',
    });
  });

  // ==========================================================================
  // POST /auth/totp/setup/initiate-onboarding
  // Start TOTP setup during onboarding (unauthenticated)
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof initiateOnboardingSchema>;
  }>('/auth/totp/setup/initiate-onboarding', async (request, reply) => {
    const validation = initiateOnboardingSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { session_id, email, tenant_name } = validation.data;

    // Generate TOTP secret
    const { secret, uri, manualEntryKey } = generateTotpSecret(email, tenant_name);
    const encryptedSecret = encryptTotpSecret(secret);

    // Generate backup codes
    const { codes, hashes } = generateBackupCodes(10);

    // Save to temporary setup session (no userId during onboarding)
    const setupSessionId = uuidv4();
    await sessionStore.saveTotpSetupSession(setupSessionId, {
      email,
      encryptedSecret,
      backupCodeHashes: hashes,
      createdAt: Date.now(),
    });

    logger.info({ email: maskEmail(email), session_id }, 'TOTP onboarding setup initiated');

    return reply.send({
      success: true,
      setup_session: setupSessionId,
      totp_uri: uri,
      manual_entry_key: manualEntryKey,
      backup_codes: codes,
    });
  });

  // ==========================================================================
  // POST /auth/totp/setup/confirm-onboarding
  // Confirm TOTP setup during onboarding (unauthenticated)
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof confirmOnboardingSchema>;
  }>('/auth/totp/setup/confirm-onboarding', async (request, reply) => {
    const validation = confirmOnboardingSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { setup_session, code, session_id } = validation.data;

    // Get setup session
    const setupData = await sessionStore.getTotpSetupSession(setup_session);
    if (!setupData) {
      return reply.code(400).send({
        success: false,
        error: 'SETUP_SESSION_EXPIRED',
        message: 'Setup session expired. Please start over.',
      });
    }

    // Decrypt and verify TOTP code
    const secret = decryptTotpSecret(setupData.encryptedSecret);
    if (!verifyTotpCode(secret, code)) {
      return reply.code(400).send({
        success: false,
        error: 'INVALID_CODE',
        message: 'Invalid verification code. Please try again.',
      });
    }

    // For onboarding, we don't persist yet — the onboarding flow will persist
    // after user creation via a separate call. Return the encrypted secret and
    // backup hashes so the frontend can pass them during the final onboarding step.
    // Clean up setup session
    await sessionStore.deleteTotpSetupSession(setup_session);

    logger.info({ email: maskEmail(setupData.email), session_id }, 'TOTP onboarding setup confirmed');

    return reply.send({
      success: true,
      message: 'Authenticator app verified successfully.',
      totp_secret_encrypted: setupData.encryptedSecret,
      backup_code_hashes: setupData.backupCodeHashes,
    });
  });

  // ==========================================================================
  // POST /auth/totp/disable
  // Disable TOTP for authenticated user (requires valid TOTP code)
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof disableTotpSchema>;
  }>('/auth/totp/disable', async (request, reply) => {
    const session = await getSessionFromCookie(request);
    if (!session) {
      return reply.code(401).send({
        success: false,
        error: 'UNAUTHORIZED',
        message: 'Authentication required.',
      });
    }

    const validation = disableTotpSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { code } = validation.data;

    let totpSecret: string | undefined;

    if (hasTenantContext(session)) {
      const totpInfo = await tenantServiceClient.getTotpSecret(session.userId, session.tenantId!);
      if (!totpInfo.totp_enabled || !totpInfo.totp_secret_encrypted) {
        return reply.code(400).send({
          success: false,
          error: 'TOTP_NOT_ENABLED',
          message: 'TOTP is not currently enabled.',
        });
      }
      totpSecret = totpInfo.totp_secret_encrypted;
    } else {
      const totpData = await sessionStore.getTotpUserData(session.userId);
      if (!totpData?.enabled) {
        return reply.code(400).send({
          success: false,
          error: 'TOTP_NOT_ENABLED',
          message: 'TOTP is not currently enabled.',
        });
      }
      totpSecret = totpData.encryptedSecret;
    }

    // Verify TOTP code before disabling
    const secret = decryptTotpSecret(totpSecret);
    if (!verifyTotpCode(secret, code)) {
      return reply.code(400).send({
        success: false,
        error: 'INVALID_CODE',
        message: 'Invalid verification code.',
      });
    }

    if (hasTenantContext(session)) {
      const disableResult = await tenantServiceClient.disableTotp(session.userId, session.tenantId!);
      if (!disableResult.success) {
        return reply.code(500).send({
          success: false,
          error: 'DISABLE_FAILED',
          message: 'Failed to disable authenticator app. Please try again.',
        });
      }
    } else {
      await sessionStore.deleteTotpUserData(session.userId);
    }

    logger.info({ userId: session.userId }, 'TOTP disabled');

    return reply.send({
      success: true,
      message: 'Authenticator app has been disabled.',
    });
  });

  // ==========================================================================
  // POST /auth/totp/backup-codes/regenerate
  // Regenerate backup codes (requires valid TOTP code)
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof regenerateBackupCodesSchema>;
  }>('/auth/totp/backup-codes/regenerate', async (request, reply) => {
    const session = await getSessionFromCookie(request);
    if (!session) {
      return reply.code(401).send({
        success: false,
        error: 'UNAUTHORIZED',
        message: 'Authentication required.',
      });
    }

    const validation = regenerateBackupCodesSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { code } = validation.data;

    let totpSecret: string | undefined;

    if (hasTenantContext(session)) {
      const totpInfo = await tenantServiceClient.getTotpSecret(session.userId, session.tenantId!);
      if (!totpInfo.totp_enabled || !totpInfo.totp_secret_encrypted) {
        return reply.code(400).send({
          success: false,
          error: 'TOTP_NOT_ENABLED',
          message: 'TOTP is not currently enabled.',
        });
      }
      totpSecret = totpInfo.totp_secret_encrypted;
    } else {
      const totpData = await sessionStore.getTotpUserData(session.userId);
      if (!totpData?.enabled) {
        return reply.code(400).send({
          success: false,
          error: 'TOTP_NOT_ENABLED',
          message: 'TOTP is not currently enabled.',
        });
      }
      totpSecret = totpData.encryptedSecret;
    }

    // Verify TOTP code
    const secret = decryptTotpSecret(totpSecret);
    if (!verifyTotpCode(secret, code)) {
      return reply.code(400).send({
        success: false,
        error: 'INVALID_CODE',
        message: 'Invalid verification code.',
      });
    }

    // Generate new backup codes
    const { codes, hashes } = generateBackupCodes(10);

    if (hasTenantContext(session)) {
      const updateResult = await tenantServiceClient.updateBackupCodes(
        session.userId,
        session.tenantId!,
        hashes
      );

      if (!updateResult.success) {
        return reply.code(500).send({
          success: false,
          error: 'UPDATE_FAILED',
          message: 'Failed to regenerate backup codes. Please try again.',
        });
      }
    } else {
      // Non-tenant context: update backup codes in Redis
      const totpData = await sessionStore.getTotpUserData(session.userId);
      if (totpData) {
        await sessionStore.saveTotpUserData(session.userId, {
          ...totpData,
          backupCodeHashes: hashes,
        });
      }
    }

    logger.info({ userId: session.userId }, 'Backup codes regenerated');

    return reply.send({
      success: true,
      backup_codes: codes,
      message: 'New backup codes generated. Previous codes are now invalid.',
    });
  });
}
