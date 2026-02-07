/**
 * TOTP Authenticator App Utilities
 *
 * Provides TOTP secret generation, verification, backup code management,
 * and AES-256-GCM encryption for storing secrets at rest.
 */

import { createHmac, randomBytes, createCipheriv, createDecipheriv } from 'crypto';
import * as OTPAuth from 'otpauth';
import { config } from './config';
import { createLogger } from './logger';

const logger = createLogger('totp');

const TOTP_ISSUER = 'Tesserix';
const TOTP_PERIOD = 30; // seconds
const TOTP_DIGITS = 6;
const TOTP_ALGORITHM = 'SHA1';
const TOTP_WINDOW = 1; // allow 1 period of drift (30s)

// ============================================================================
// TOTP Secret Generation & Verification
// ============================================================================

export interface TotpSecretResult {
  secret: string; // base32-encoded secret
  uri: string; // otpauth:// URI for QR code
  manualEntryKey: string; // human-readable base32 key
}

/**
 * Generate a new TOTP secret for a user.
 * Returns the secret, an otpauth:// URI for QR scanning, and a manual entry key.
 */
export function generateTotpSecret(email: string, tenantName?: string): TotpSecretResult {
  const totp = new OTPAuth.TOTP({
    issuer: tenantName ? `${TOTP_ISSUER} (${tenantName})` : TOTP_ISSUER,
    label: email,
    algorithm: TOTP_ALGORITHM,
    digits: TOTP_DIGITS,
    period: TOTP_PERIOD,
  });

  const secret = totp.secret.base32;
  const uri = totp.toString();
  // Format as groups of 4 for readability
  const manualEntryKey = secret.replace(/(.{4})/g, '$1 ').trim();

  return { secret, uri, manualEntryKey };
}

/**
 * Verify a TOTP code against a base32 secret.
 * Allows 1-period (30s) window for clock drift.
 */
export function verifyTotpCode(secret: string, code: string): boolean {
  try {
    const totp = new OTPAuth.TOTP({
      secret: OTPAuth.Secret.fromBase32(secret),
      algorithm: TOTP_ALGORITHM,
      digits: TOTP_DIGITS,
      period: TOTP_PERIOD,
    });

    const delta = totp.validate({ token: code, window: TOTP_WINDOW });
    return delta !== null;
  } catch (error) {
    logger.error({ error }, 'TOTP verification error');
    return false;
  }
}

// ============================================================================
// Backup Codes
// ============================================================================

export interface BackupCodesResult {
  codes: string[]; // plaintext codes to show user once
  hashes: string[]; // SHA-256 hashes to store
}

/**
 * Generate backup recovery codes.
 * Returns plaintext codes (shown once) and their SHA-256 hashes (stored).
 */
export function generateBackupCodes(count: number = 10): BackupCodesResult {
  const codes: string[] = [];
  const hashes: string[] = [];

  for (let i = 0; i < count; i++) {
    // 8-character alphanumeric codes, formatted as XXXX-XXXX
    const raw = randomBytes(5).toString('hex').slice(0, 8).toUpperCase();
    const code = `${raw.slice(0, 4)}-${raw.slice(4)}`;
    codes.push(code);
    hashes.push(hashBackupCode(code));
  }

  return { codes, hashes };
}

/**
 * Hash a backup code using SHA-256.
 * Normalizes by removing dashes and uppercasing before hashing.
 */
export function hashBackupCode(code: string): string {
  const normalized = code.replace(/-/g, '').toUpperCase();
  return createHmac('sha256', 'tesserix-backup-code')
    .update(normalized)
    .digest('hex');
}

/**
 * Verify a backup code against stored hashes.
 * Returns the index of the matching hash, or -1 if not found.
 */
export function verifyBackupCode(code: string, hashes: string[]): number {
  const codeHash = hashBackupCode(code);
  return hashes.findIndex((h) => h === codeHash);
}

// ============================================================================
// TOTP Secret Encryption (AES-256-GCM)
// ============================================================================

/**
 * Encrypt a TOTP secret for storage at rest using AES-256-GCM.
 * Returns a string in format: iv:authTag:ciphertext (all hex-encoded).
 */
export function encryptTotpSecret(secret: string): string {
  const key = getEncryptionKey();
  const iv = randomBytes(12); // 96-bit IV for GCM
  const cipher = createCipheriv('aes-256-gcm', key, iv);

  let encrypted = cipher.update(secret, 'utf8', 'hex');
  encrypted += cipher.final('hex');
  const authTag = cipher.getAuthTag().toString('hex');

  return `${iv.toString('hex')}:${authTag}:${encrypted}`;
}

/**
 * Decrypt a TOTP secret from storage.
 */
export function decryptTotpSecret(encrypted: string): string {
  const key = getEncryptionKey();
  const [ivHex, authTagHex, ciphertext] = encrypted.split(':');

  if (!ivHex || !authTagHex || !ciphertext) {
    throw new Error('Invalid encrypted TOTP secret format');
  }

  const iv = Buffer.from(ivHex, 'hex');
  const authTag = Buffer.from(authTagHex, 'hex');
  const decipher = createDecipheriv('aes-256-gcm', key, iv);
  decipher.setAuthTag(authTag);

  let decrypted = decipher.update(ciphertext, 'hex', 'utf8');
  decrypted += decipher.final('utf8');

  return decrypted;
}

function getEncryptionKey(): Buffer {
  const keyHex = config.totpEncryptionKey;
  if (!keyHex || keyHex.length < 32) {
    throw new Error('TOTP_ENCRYPTION_KEY must be set (min 32 hex chars for AES-256)');
  }
  // Use first 32 bytes (64 hex chars) or hash if shorter
  if (keyHex.length >= 64) {
    return Buffer.from(keyHex.slice(0, 64), 'hex');
  }
  // If key is 32+ chars but not 64 hex chars, derive a 32-byte key via SHA-256
  return Buffer.from(
    createHmac('sha256', 'tesserix-totp-key').update(keyHex).digest()
  );
}
