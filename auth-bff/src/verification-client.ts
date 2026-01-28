/**
 * Shared client for calling the verification-service API.
 * Used by OTP routes and MFA routes.
 */

import { config } from './config';
import { createLogger } from './logger';

const logger = createLogger('verification-client');

export async function callVerificationService(
  endpoint: string,
  method: 'GET' | 'POST',
  body?: Record<string, unknown>,
  queryParams?: Record<string, string>
): Promise<{ success: boolean; status: number; data?: unknown; error?: string }> {
  const baseUrl = config.verificationServiceUrl;
  const apiKey = config.verificationServiceApiKey;

  logger.debug({ baseUrl, hasApiKey: !!apiKey, endpoint }, 'Calling verification service');

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
