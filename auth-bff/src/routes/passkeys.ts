/**
 * Passkey (WebAuthn) Routes for Storefront Customers
 *
 * Enables passwordless authentication using biometrics (fingerprint, Face ID)
 * or hardware security keys.
 *
 * Registration flow (requires active session):
 * 1. POST /auth/passkeys/registration/options — generate challenge
 * 2. Browser creates credential (biometric prompt)
 * 3. POST /auth/passkeys/registration/verify — verify + store credential
 *
 * Authentication flow (no session required):
 * 1. POST /auth/passkeys/authentication/options — generate challenge
 * 2. Browser prompts biometric
 * 3. POST /auth/passkeys/authentication/verify — verify + create session
 *
 * Passkey auth skips MFA — passkeys inherently provide multi-factor
 * (possession of device + biometric/PIN).
 */

import { FastifyInstance } from 'fastify';
import type { RegistrationResponseJSON, AuthenticationResponseJSON, AuthenticatorTransportFuture } from '@simplewebauthn/types';
import {
  generateRegistrationOptions,
  verifyRegistrationResponse,
  generateAuthenticationOptions,
  verifyAuthenticationResponse,
} from '@simplewebauthn/server';
import { v4 as uuidv4 } from 'uuid';
import { config, getSessionCookieName, isAdminHost } from '../config';
import { sessionStore } from '../session-store';
import { tenantServiceClient } from '../tenant-service-client';
import { setSessionCookie } from '../cookie-helpers';
import { createLogger } from '../logger';

const logger = createLogger('passkeys');

/**
 * Extract the effective forwarded host from a Fastify request.
 * Handles comma-separated values from multiple proxies (e.g. ingress → Istio → BFF)
 * and falls back to Fastify's request.hostname (which respects trust proxy settings).
 */
function getEffectiveHost(request: { headers: Record<string, string | string[] | undefined>; hostname: string }): string | undefined {
  const raw = request.headers['x-forwarded-host'];
  if (raw) {
    const value = Array.isArray(raw) ? raw[0] : raw;
    // Take the first value if comma-separated (leftmost is the original client host)
    return value.split(',')[0].trim();
  }
  // Fallback: use Fastify's hostname (resolved via trust proxy)
  const hostname = request.hostname;
  if (hostname && !hostname.includes('.svc.cluster.local') && hostname !== '127.0.0.1') {
    return hostname;
  }
  return undefined;
}

/**
 * Derive the RP ID (Relying Party Identifier) from the forwarded host.
 * - For *.tesserix.app subdomains: use "tesserix.app" (allows cross-subdomain passkeys)
 * - For localhost: use "localhost"
 * - For custom domains: use the full hostname
 */
function deriveRpId(forwardedHost: string | undefined): string {
  if (!forwardedHost) return 'localhost';

  const hostname = forwardedHost.split(':')[0].toLowerCase();

  if (hostname.endsWith(`.${config.baseDomain}`) || hostname === config.baseDomain) {
    return config.baseDomain;
  }

  if (hostname === 'localhost' || hostname.endsWith('.localhost')) {
    return 'localhost';
  }

  return hostname;
}

/**
 * Derive the expected origin from the forwarded host.
 */
function deriveExpectedOrigin(forwardedHost: string | undefined): string {
  if (!forwardedHost) return 'http://localhost:3200';

  const hostname = forwardedHost.split(':')[0].toLowerCase();

  if (hostname === 'localhost' || hostname.endsWith('.localhost')) {
    // In dev, port may vary — accept common storefront ports
    const port = forwardedHost.includes(':') ? forwardedHost.split(':')[1] : '3200';
    return `http://localhost:${port}`;
  }

  return `https://${hostname}`;
}

export async function passkeyRoutes(fastify: FastifyInstance) {
  // ==========================================================================
  // POST /auth/passkeys/registration/options
  // Generate WebAuthn registration options (requires active session)
  // ==========================================================================
  fastify.post('/auth/passkeys/registration/options', async (request, reply) => {
    const forwardedHost = getEffectiveHost(request);
    const cookieName = getSessionCookieName(forwardedHost);
    const sessionId = request.cookies[cookieName];

    if (!sessionId) {
      return reply.code(401).send({ success: false, error: 'UNAUTHORIZED', message: 'Session required.' });
    }

    const session = await sessionStore.getSession(sessionId);
    if (!session) {
      return reply.code(401).send({ success: false, error: 'SESSION_EXPIRED', message: 'Session expired.' });
    }

    const rpId = deriveRpId(forwardedHost);
    const rpName = 'Tesserix';

    // Get existing passkeys so we can exclude them
    let existingCredentials: { id: string; transports?: AuthenticatorTransportFuture[] }[] = [];
    if (session.tenantId) {
      const existing = await tenantServiceClient.getPasskeys(session.userId, session.tenantId);
      if (existing.success) {
        existingCredentials = existing.passkeys.map((p) => ({
          id: p.credential_id, // Already base64url
          transports: p.transports as AuthenticatorTransportFuture[] | undefined,
        }));
      }
    }

    const options = await generateRegistrationOptions({
      rpName,
      rpID: rpId,
      userName: session.userInfo?.email as string || session.userId,
      userDisplayName: session.userInfo?.name as string || session.userInfo?.email as string || session.userId,
      userID: Uint8Array.from(Buffer.from(session.userId)),
      attestationType: 'none',
      excludeCredentials: existingCredentials,
      authenticatorSelection: {
        residentKey: 'preferred',
        userVerification: 'preferred',
      },
    });

    // Save challenge to Redis
    const challengeId = uuidv4();
    await sessionStore.savePasskeyChallenge(challengeId, {
      type: 'registration',
      challenge: options.challenge,
      userId: session.userId,
      tenantId: session.tenantId,
      tenantSlug: session.tenantSlug,
      rpId,
      createdAt: Date.now(),
    });

    logger.info({ userId: session.userId, rpId }, 'Passkey registration options generated');

    return reply.send({
      success: true,
      options,
      challengeId,
    });
  });

  // ==========================================================================
  // POST /auth/passkeys/registration/verify
  // Verify attestation and store credential (requires active session)
  // ==========================================================================
  fastify.post<{
    Body: {
      challengeId: string;
      credential: RegistrationResponseJSON;
      name?: string;
    };
  }>('/auth/passkeys/registration/verify', async (request, reply) => {
    const { challengeId, credential, name } = request.body;

    if (!challengeId || !credential) {
      return reply.code(400).send({ success: false, error: 'VALIDATION_ERROR', message: 'Missing challengeId or credential.' });
    }

    // Verify session
    const forwardedHost = getEffectiveHost(request);
    const cookieName = getSessionCookieName(forwardedHost);
    const sessionId = request.cookies[cookieName];

    if (!sessionId) {
      return reply.code(401).send({ success: false, error: 'UNAUTHORIZED', message: 'Session required.' });
    }

    const session = await sessionStore.getSession(sessionId);
    if (!session) {
      return reply.code(401).send({ success: false, error: 'SESSION_EXPIRED', message: 'Session expired.' });
    }

    // Consume challenge (single-use)
    const challengeData = await sessionStore.consumePasskeyChallenge(challengeId);
    if (!challengeData || challengeData.type !== 'registration') {
      return reply.code(400).send({ success: false, error: 'INVALID_CHALLENGE', message: 'Challenge expired or invalid.' });
    }

    // Verify the user matches
    if (challengeData.userId !== session.userId) {
      return reply.code(403).send({ success: false, error: 'USER_MISMATCH', message: 'Session user does not match challenge.' });
    }

    const expectedOrigin = deriveExpectedOrigin(forwardedHost);

    try {
      const verification = await verifyRegistrationResponse({
        response: credential,
        expectedChallenge: challengeData.challenge,
        expectedOrigin,
        expectedRPID: challengeData.rpId,
        requireUserVerification: false, // Options use userVerification: 'preferred'
      });

      if (!verification.verified || !verification.registrationInfo) {
        return reply.code(400).send({ success: false, error: 'VERIFICATION_FAILED', message: 'Passkey verification failed.' });
      }

      const {
        credentialID,
        credentialPublicKey,
        counter,
        credentialDeviceType,
        credentialBackedUp,
      } = verification.registrationInfo;

      // Store credential in tenant-service
      const saveResult = await tenantServiceClient.savePasskey({
        user_id: session.userId,
        tenant_id: session.tenantId || '',
        credential_id: credentialID,
        public_key: Buffer.from(credentialPublicKey).toString('base64url'),
        counter,
        device_type: credentialDeviceType,
        backed_up: credentialBackedUp,
        transports: credential.response.transports,
        name: name || 'My Passkey',
      });

      if (!saveResult.success) {
        logger.error({ userId: session.userId }, 'Failed to save passkey to tenant-service');
        return reply.code(500).send({ success: false, error: 'SAVE_FAILED', message: 'Failed to save passkey.' });
      }

      logger.info({ userId: session.userId, credentialId: credentialID }, 'Passkey registered');

      return reply.send({ success: true, message: 'Passkey registered successfully.' });
    } catch (error) {
      logger.error({ err: error, errorMessage: error instanceof Error ? error.message : String(error) }, 'Passkey registration verification error');
      return reply.code(400).send({ success: false, error: 'VERIFICATION_ERROR', message: 'Failed to verify passkey.' });
    }
  });

  // ==========================================================================
  // POST /auth/passkeys/authentication/options
  // Generate WebAuthn authentication options (NO session required)
  // ==========================================================================
  fastify.post('/auth/passkeys/authentication/options', async (request, reply) => {
    const forwardedHost = getEffectiveHost(request);
    const rpId = deriveRpId(forwardedHost);

    const options = await generateAuthenticationOptions({
      rpID: rpId,
      userVerification: 'preferred',
      // Empty allowCredentials for discoverable/resident key flow
    });

    // Save challenge to Redis
    const challengeId = uuidv4();
    await sessionStore.savePasskeyChallenge(challengeId, {
      type: 'authentication',
      challenge: options.challenge,
      rpId,
      createdAt: Date.now(),
    });

    logger.info({ rpId }, 'Passkey authentication options generated');

    return reply.send({
      success: true,
      options,
      challengeId,
    });
  });

  // ==========================================================================
  // POST /auth/passkeys/authentication/verify
  // Verify assertion, create session (NO session required)
  // ==========================================================================
  fastify.post<{
    Body: {
      challengeId: string;
      credential: AuthenticationResponseJSON;
    };
  }>('/auth/passkeys/authentication/verify', async (request, reply) => {
    const { challengeId, credential } = request.body;

    if (!challengeId || !credential) {
      return reply.code(400).send({ success: false, error: 'VALIDATION_ERROR', message: 'Missing challengeId or credential.' });
    }

    // Consume challenge (single-use)
    const challengeData = await sessionStore.consumePasskeyChallenge(challengeId);
    if (!challengeData || challengeData.type !== 'authentication') {
      return reply.code(400).send({ success: false, error: 'INVALID_CHALLENGE', message: 'Challenge expired or invalid.' });
    }

    // Extract credential ID from the assertion to look up the user
    const credentialIdBase64url = credential.id;
    if (!credentialIdBase64url) {
      return reply.code(400).send({ success: false, error: 'INVALID_CREDENTIAL', message: 'Missing credential ID.' });
    }

    // Look up credential in tenant-service (returns user info + tokens)
    const lookupResult = await tenantServiceClient.getPasskeyByCredentialId(
      credentialIdBase64url,
      challengeData.rpId
    );

    if (!lookupResult.success || !lookupResult.passkey) {
      logger.info({ credentialId: credentialIdBase64url }, 'Passkey not found for authentication');
      return reply.code(401).send({ success: false, error: 'PASSKEY_NOT_FOUND', message: 'Passkey not recognized.' });
    }

    const forwardedHost = getEffectiveHost(request);
    const expectedOrigin = deriveExpectedOrigin(forwardedHost);

    try {
      const verification = await verifyAuthenticationResponse({
        response: credential,
        expectedChallenge: challengeData.challenge,
        expectedOrigin,
        expectedRPID: challengeData.rpId,
        requireUserVerification: false, // Options use userVerification: 'preferred'
        authenticator: {
          credentialID: credentialIdBase64url,
          credentialPublicKey: Uint8Array.from(Buffer.from(lookupResult.passkey.public_key, 'base64url')),
          counter: lookupResult.passkey.counter,
          transports: lookupResult.passkey.transports as AuthenticatorTransportFuture[] | undefined,
        },
      });

      if (!verification.verified) {
        return reply.code(401).send({ success: false, error: 'VERIFICATION_FAILED', message: 'Passkey verification failed.' });
      }

      // Update counter (clone detection)
      await tenantServiceClient.updatePasskeyCounter(
        credentialIdBase64url,
        verification.authenticationInfo.newCounter
      );

      // Update last used timestamp (fire-and-forget)
      tenantServiceClient.updatePasskeyLastUsed(credentialIdBase64url).catch(() => {});

      // Check we got tokens
      if (!lookupResult.access_token) {
        logger.error({ credentialId: credentialIdBase64url }, 'No tokens received from tenant-service for passkey auth');
        return reply.code(500).send({ success: false, error: 'TOKEN_ERROR', message: 'Failed to complete authentication.' });
      }

      // Create session (follows same pattern as direct-auth.ts)
      const sessionExpirySeconds = 86400; // 24 hours
      const newSession = await sessionStore.createSession({
        userId: lookupResult.user_id!,
        tenantId: lookupResult.tenant_id,
        tenantSlug: lookupResult.tenant_slug,
        clientType: isAdminHost(forwardedHost) ? 'internal' : 'customer',
        accessToken: lookupResult.access_token,
        idToken: lookupResult.id_token,
        refreshToken: lookupResult.refresh_token,
        expiresAt: Math.floor(Date.now() / 1000) + sessionExpirySeconds,
        userInfo: {
          sub: lookupResult.keycloak_user_id || lookupResult.user_id,
          email: lookupResult.email,
          given_name: lookupResult.first_name,
          family_name: lookupResult.last_name,
          name: lookupResult.first_name && lookupResult.last_name
            ? `${lookupResult.first_name} ${lookupResult.last_name}`
            : lookupResult.email,
          tenant_id: lookupResult.tenant_id,
          tenant_slug: lookupResult.tenant_slug,
          realm_access: {
            roles: lookupResult.role ? [lookupResult.role] : [],
          },
        },
      });

      // Set session cookie
      setSessionCookie(reply, newSession.id, forwardedHost, false);

      logger.info({
        userId: newSession.userId,
        sessionId: newSession.id,
        credentialId: credentialIdBase64url,
      }, 'Passkey authentication successful');

      return reply.send({
        success: true,
        authenticated: true,
        user: {
          id: lookupResult.user_id,
          email: lookupResult.email,
          first_name: lookupResult.first_name,
          last_name: lookupResult.last_name,
          tenant_id: lookupResult.tenant_id,
          tenant_slug: lookupResult.tenant_slug,
          role: lookupResult.role,
        },
        session: {
          expires_at: newSession.expiresAt,
          csrf_token: newSession.csrfToken,
        },
      });
    } catch (error) {
      logger.error({ err: error, errorMessage: error instanceof Error ? error.message : String(error), credentialId: credentialIdBase64url }, 'Passkey authentication verification error');
      return reply.code(400).send({ success: false, error: 'VERIFICATION_ERROR', message: 'Failed to verify passkey.' });
    }
  });

  // ==========================================================================
  // GET /auth/passkeys/list
  // List user's registered passkeys (requires session)
  // ==========================================================================
  fastify.get('/auth/passkeys/list', async (request, reply) => {
    const forwardedHost = getEffectiveHost(request);
    const cookieName = getSessionCookieName(forwardedHost);
    const sessionId = request.cookies[cookieName];

    if (!sessionId) {
      return reply.code(401).send({ success: false, error: 'UNAUTHORIZED', message: 'Session required.' });
    }

    const session = await sessionStore.getSession(sessionId);
    if (!session) {
      return reply.code(401).send({ success: false, error: 'SESSION_EXPIRED', message: 'Session expired.' });
    }

    if (!session.tenantId) {
      return reply.code(400).send({ success: false, error: 'NO_TENANT', message: 'No tenant context.' });
    }

    const result = await tenantServiceClient.getPasskeys(session.userId, session.tenantId);

    return reply.send({
      success: result.success,
      passkeys: result.passkeys.map((p) => ({
        credential_id: p.credential_id,
        name: p.name,
        created_at: p.created_at,
        last_used_at: p.last_used_at,
        device_type: p.device_type,
        backed_up: p.backed_up,
      })),
    });
  });

  // ==========================================================================
  // POST /auth/passkeys/rename
  // Rename a passkey (requires session)
  // ==========================================================================
  fastify.post<{
    Body: { credential_id: string; name: string };
  }>('/auth/passkeys/rename', async (request, reply) => {
    const { credential_id, name } = request.body;

    if (!credential_id || !name) {
      return reply.code(400).send({ success: false, error: 'VALIDATION_ERROR', message: 'credential_id and name are required.' });
    }

    const forwardedHost = getEffectiveHost(request);
    const cookieName = getSessionCookieName(forwardedHost);
    const sessionId = request.cookies[cookieName];

    if (!sessionId) {
      return reply.code(401).send({ success: false, error: 'UNAUTHORIZED', message: 'Session required.' });
    }

    const session = await sessionStore.getSession(sessionId);
    if (!session) {
      return reply.code(401).send({ success: false, error: 'SESSION_EXPIRED', message: 'Session expired.' });
    }

    const result = await tenantServiceClient.renamePasskey(
      credential_id,
      session.userId,
      session.tenantId || '',
      name
    );

    if (!result.success) {
      return reply.code(400).send({ success: false, error: 'RENAME_FAILED', message: result.message });
    }

    return reply.send({ success: true, message: 'Passkey renamed.' });
  });

  // ==========================================================================
  // POST /auth/passkeys/delete
  // Delete a passkey (requires session)
  // ==========================================================================
  fastify.post<{
    Body: { credential_id: string };
  }>('/auth/passkeys/delete', async (request, reply) => {
    const { credential_id } = request.body;

    if (!credential_id) {
      return reply.code(400).send({ success: false, error: 'VALIDATION_ERROR', message: 'credential_id is required.' });
    }

    const forwardedHost = getEffectiveHost(request);
    const cookieName = getSessionCookieName(forwardedHost);
    const sessionId = request.cookies[cookieName];

    if (!sessionId) {
      return reply.code(401).send({ success: false, error: 'UNAUTHORIZED', message: 'Session required.' });
    }

    const session = await sessionStore.getSession(sessionId);
    if (!session) {
      return reply.code(401).send({ success: false, error: 'SESSION_EXPIRED', message: 'Session expired.' });
    }

    const result = await tenantServiceClient.deletePasskey(
      credential_id,
      session.userId,
      session.tenantId || ''
    );

    if (!result.success) {
      return reply.code(400).send({ success: false, error: 'DELETE_FAILED', message: result.message });
    }

    logger.info({ userId: session.userId, credentialId: credential_id }, 'Passkey deleted');

    return reply.send({ success: true, message: 'Passkey deleted.' });
  });
}
