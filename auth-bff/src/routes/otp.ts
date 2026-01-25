/**
 * OTP Routes for Customer Email Verification
 *
 * These routes proxy OTP (One-Time Password) requests to the verification-service
 * for customer email verification during registration and other flows.
 *
 * Routes:
 * - POST /auth/otp/send - Send OTP to email
 * - POST /auth/otp/verify - Verify OTP code
 * - POST /auth/otp/resend - Resend OTP code
 * - GET /auth/otp/status - Check verification status
 */

import { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { config } from '../config';
import { createLogger } from '../logger';

const logger = createLogger('otp-routes');

// ============================================================================
// Request Schemas
// ============================================================================

const sendOTPSchema = z.object({
  email: z.string().email('Invalid email format'),
  channel: z.enum(['email', 'sms']).default('email'),
  purpose: z.enum(['customer_email_verification']),
  metadata: z.object({
    businessName: z.string().optional(),
    tenantSlug: z.string().optional(),
  }).optional(),
});

const verifyOTPSchema = z.object({
  email: z.string().email('Invalid email format'),
  code: z.string().min(6).max(6, 'Code must be 6 digits'),
  purpose: z.enum(['customer_email_verification']),
});

const resendOTPSchema = z.object({
  email: z.string().email('Invalid email format'),
  channel: z.enum(['email', 'sms']).default('email'),
  purpose: z.enum(['customer_email_verification']),
  metadata: z.object({
    businessName: z.string().optional(),
    tenantSlug: z.string().optional(),
  }).optional(),
});

// ============================================================================
// Rate Limiting State
// ============================================================================

interface RateLimitEntry {
  count: number;
  resetAt: number;
}

const rateLimits = new Map<string, RateLimitEntry>();
const RATE_LIMIT_WINDOW_MS = 60000; // 1 minute
const RATE_LIMIT_SEND_MAX = 3; // 3 send attempts per minute per email
const RATE_LIMIT_VERIFY_MAX = 10; // 10 verify attempts per minute per email

function checkRateLimit(key: string, maxAttempts: number): { allowed: boolean; remaining: number; resetIn: number } {
  const now = Date.now();
  const entry = rateLimits.get(key);

  if (!entry || now > entry.resetAt) {
    rateLimits.set(key, { count: 1, resetAt: now + RATE_LIMIT_WINDOW_MS });
    return { allowed: true, remaining: maxAttempts - 1, resetIn: RATE_LIMIT_WINDOW_MS };
  }

  if (entry.count >= maxAttempts) {
    return { allowed: false, remaining: 0, resetIn: entry.resetAt - now };
  }

  entry.count++;
  return { allowed: true, remaining: maxAttempts - entry.count, resetIn: entry.resetAt - now };
}

// ============================================================================
// Verification Service Client
// ============================================================================

async function callVerificationService(
  endpoint: string,
  method: 'GET' | 'POST',
  body?: Record<string, unknown>,
  queryParams?: Record<string, string>
): Promise<{ success: boolean; status: number; data?: unknown; error?: string }> {
  const baseUrl = config.verificationServiceUrl;
  const apiKey = config.verificationServiceApiKey;

  let url = baseUrl + '/api/v1' + endpoint;
  if (queryParams) {
    const params = new URLSearchParams(queryParams);
    url += '?' + params.toString();
  }

  try {
    const response = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(apiKey ? { 'X-API-Key': apiKey } : {}),
      },
      ...(body ? { body: JSON.stringify(body) } : {}),
    });

    const data = await response.json().catch(() => null) as { message?: string } | null;

    return {
      success: response.ok,
      status: response.status,
      data,
      error: !response.ok ? (data?.message || 'Service error') : undefined,
    };
  } catch (error) {
    logger.error({ error, endpoint }, 'Failed to call verification service');
    return {
      success: false,
      status: 503,
      error: 'Verification service unavailable',
    };
  }
}

// ============================================================================
// Routes
// ============================================================================

export async function otpRoutes(fastify: FastifyInstance) {
  // ==========================================================================
  // POST /auth/otp/send
  // Sends an OTP to the specified email
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof sendOTPSchema>;
  }>('/auth/otp/send', async (request, reply) => {
    const validation = sendOTPSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, channel, purpose, metadata } = validation.data;
    const clientIP = request.ip;

    // Rate limit by IP + email
    const rateLimitKey = 'otp-send:' + clientIP + ':' + email.toLowerCase();
    const rateLimit = checkRateLimit(rateLimitKey, RATE_LIMIT_SEND_MAX);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email }, 'Rate limit exceeded for OTP send');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many OTP requests. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Call verification service
    const result = await callVerificationService('/verify/send', 'POST', {
      recipient: email,
      channel,
      purpose,
      metadata: metadata || {},
    });

    if (!result.success) {
      logger.error({ email, error: result.error }, 'Failed to send OTP');
      return reply.code(result.status).send({
        success: false,
        error: 'SEND_FAILED',
        message: result.error || 'Failed to send verification code',
      });
    }

    const responseData = result.data as Record<string, unknown>;

    logger.info({ email, purpose }, 'OTP sent successfully');
    return reply.send({
      success: true,
      message: 'Verification code sent successfully',
      data: responseData?.data || {
        recipient: email,
        channel,
        purpose,
      },
    });
  });

  // ==========================================================================
  // POST /auth/otp/verify
  // Verifies an OTP code
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof verifyOTPSchema>;
  }>('/auth/otp/verify', async (request, reply) => {
    const validation = verifyOTPSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, code, purpose } = validation.data;
    const clientIP = request.ip;

    // Rate limit by IP + email (more lenient for verify)
    const rateLimitKey = 'otp-verify:' + clientIP + ':' + email.toLowerCase();
    const rateLimit = checkRateLimit(rateLimitKey, RATE_LIMIT_VERIFY_MAX);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email }, 'Rate limit exceeded for OTP verify');
      return reply.code(429).send({
        success: false,
        verified: false,
        error: 'RATE_LIMITED',
        message: 'Too many verification attempts. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Call verification service
    const result = await callVerificationService('/verify/code', 'POST', {
      recipient: email,
      code,
      purpose,
    });

    if (!result.success) {
      const responseData = result.data as Record<string, unknown>;
      logger.info({ email, error: result.error }, 'OTP verification failed');
      return reply.code(200).send({
        success: false,
        verified: false,
        message: result.error || 'Invalid verification code',
        remainingAttempts: responseData?.remainingAttempts,
      });
    }

    const responseData = result.data as { data?: { verified?: boolean }; message?: string };
    const isVerified = responseData?.data?.verified ?? true;

    if (!isVerified) {
      logger.info({ email }, 'OTP code incorrect');
      return reply.send({
        success: false,
        verified: false,
        message: responseData?.message || 'Invalid verification code',
      });
    }

    logger.info({ email, purpose }, 'OTP verified successfully');
    return reply.send({
      success: true,
      verified: true,
      message: 'Email verified successfully',
      data: responseData?.data,
    });
  });

  // ==========================================================================
  // POST /auth/otp/resend
  // Resends an OTP code
  // ==========================================================================
  fastify.post<{
    Body: z.infer<typeof resendOTPSchema>;
  }>('/auth/otp/resend', async (request, reply) => {
    const validation = resendOTPSchema.safeParse(request.body);
    if (!validation.success) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: validation.error.issues[0]?.message || 'Invalid request',
      });
    }

    const { email, channel, purpose, metadata } = validation.data;
    const clientIP = request.ip;

    // Rate limit (same as send)
    const rateLimitKey = 'otp-send:' + clientIP + ':' + email.toLowerCase();
    const rateLimit = checkRateLimit(rateLimitKey, RATE_LIMIT_SEND_MAX);
    if (!rateLimit.allowed) {
      logger.warn({ ip: clientIP, email }, 'Rate limit exceeded for OTP resend');
      return reply.code(429).send({
        success: false,
        error: 'RATE_LIMITED',
        message: 'Too many OTP requests. Please try again later.',
        retry_after: Math.ceil(rateLimit.resetIn / 1000),
      });
    }

    // Call verification service
    const result = await callVerificationService('/verify/resend', 'POST', {
      recipient: email,
      channel,
      purpose,
      metadata: metadata || {},
    });

    if (!result.success) {
      logger.error({ email, error: result.error }, 'Failed to resend OTP');
      return reply.code(result.status).send({
        success: false,
        error: 'RESEND_FAILED',
        message: result.error || 'Failed to resend verification code',
      });
    }

    const responseData = result.data as Record<string, unknown>;

    logger.info({ email, purpose }, 'OTP resent successfully');
    return reply.send({
      success: true,
      message: 'Verification code resent successfully',
      data: responseData?.data || {
        recipient: email,
        channel,
        purpose,
      },
    });
  });

  // ==========================================================================
  // GET /auth/otp/status
  // Gets verification status for an email
  // ==========================================================================
  fastify.get<{
    Querystring: { email: string; purpose: string };
  }>('/auth/otp/status', async (request, reply) => {
    const { email, purpose } = request.query;

    if (!email || !purpose) {
      return reply.code(400).send({
        success: false,
        error: 'VALIDATION_ERROR',
        message: 'email and purpose query parameters are required',
      });
    }

    // Call verification service
    const result = await callVerificationService('/verify/status', 'GET', undefined, {
      recipient: email,
      purpose,
    });

    if (!result.success) {
      // Return default status on error
      return reply.send({
        success: true,
        data: {
          recipient: email,
          purpose,
          isVerified: false,
          pendingCode: false,
          canResend: true,
          attemptsLeft: 5,
        },
      });
    }

    const responseData = result.data as Record<string, unknown>;

    return reply.send({
      success: true,
      data: responseData?.data || {
        recipient: email,
        purpose,
        isVerified: false,
        pendingCode: false,
        canResend: true,
        attemptsLeft: 5,
      },
    });
  });
}
