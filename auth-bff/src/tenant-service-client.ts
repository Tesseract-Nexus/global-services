/**
 * Tenant Service Client for Multi-Tenant Authentication
 *
 * This client handles communication with the tenant-service for enterprise-grade
 * multi-tenant credential validation. It enables the auth-bff to:
 * - Look up available tenants for a user (by email)
 * - Validate tenant-specific credentials
 * - Receive Keycloak tokens after successful validation
 *
 * This is a core component of the multi-tenant credential isolation feature
 * where the same email can have different passwords for different tenants.
 */

import { config } from './config';
import { createLogger } from './logger';

const logger = createLogger('tenant-service-client');

// ============================================================================
// Types
// ============================================================================

export interface TenantInfo {
  // Tenant-service returns these field names
  id: string;
  slug: string;
  name: string;
  display_name?: string;
  logo_url?: string;
  role?: string;
  is_default?: boolean;
}

export interface ValidateCredentialsRequest {
  email: string;
  password: string;
  tenant_id?: string;
  tenant_slug?: string;
  auth_context?: 'customer' | 'staff'; // SECURITY: 'customer' prevents staff from logging into storefront
}

export interface ValidateCredentialsResponse {
  success: boolean;
  data?: {
    valid: boolean;
    user_id?: string;
    keycloak_user_id?: string;
    tenant_id: string;
    tenant_slug: string;
    email: string;
    first_name?: string;
    last_name?: string;
    role?: string;
    mfa_required: boolean;
    mfa_enabled: boolean;
    account_locked: boolean;
    locked_until?: string;
    remaining_attempts?: number;
    error_code?: string;
    message?: string;
    // Tokens from Keycloak (via token exchange)
    access_token?: string;
    refresh_token?: string;
    id_token?: string;
    expires_in?: number;
  };
  message?: string;
  error_code?: string;
}

export interface GetUserTenantsResponse {
  success: boolean;
  data?: {
    tenants: TenantInfo[];
    count: number;
  };
  message?: string;
}

export interface AccountStatusResponse {
  success: boolean;
  account_exists: boolean;
  account_locked: boolean;
  locked_until?: string;
  remaining_attempts?: number;
}

export interface RegisterCustomerRequest {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  phone?: string;
  tenant_slug: string;
}

export interface RegisterCustomerResponse {
  success: boolean;
  data?: {
    user_id?: string;
    tenant_id: string;
    tenant_slug: string;
    email: string;
    first_name?: string;
    last_name?: string;
    access_token?: string;
    refresh_token?: string;
    id_token?: string;
    expires_in?: number;
    error_code?: string;
    message?: string;
  };
  message?: string;
  error_code?: string;
}

// ============================================================================
// Account Deactivation Types
// ============================================================================

export interface CheckDeactivatedRequest {
  email: string;
  tenant_slug: string;
}

export interface CheckDeactivatedResponse {
  success: boolean;
  is_deactivated: boolean;
  can_reactivate: boolean;
  days_until_purge?: number;
  deactivated_at?: string;
  purge_date?: string;
  message?: string;
  error_code?: string;
}

export interface ReactivateAccountRequest {
  email: string;
  password: string;
  tenant_slug: string;
}

export interface ReactivateAccountResponse {
  success: boolean;
  message?: string;
  error_code?: string;
  error_message?: string;
}

export interface DeactivateAccountRequest {
  user_id: string;
  tenant_id: string;
  reason?: string;
}

export interface DeactivateAccountResponse {
  success: boolean;
  deactivated_at?: string;
  scheduled_purge_at?: string;
  days_until_purge?: number;
  message?: string;
  error_code?: string;
}

// ============================================================================
// Password Reset Types
// ============================================================================

export interface RequestPasswordResetRequest {
  email: string;
  tenant_slug: string;
  ip_address?: string;
  user_agent?: string;
}

export interface RequestPasswordResetResponse {
  success: boolean;
  message?: string;
  error_code?: string;
}

export interface ValidateResetTokenResponse {
  success: boolean;
  valid: boolean;
  email?: string;
  expires_at?: string;
  message?: string;
  error_code?: string;
}

export interface ResetPasswordRequest {
  token: string;
  new_password: string;
  ip_address?: string;
  user_agent?: string;
}

export interface ResetPasswordResponse {
  success: boolean;
  message?: string;
  error_code?: string;
}

// Internal API response types
interface ApiResponse {
  success?: boolean;
  message?: string;
  error_code?: string;
  data?: Record<string, unknown>;
  // Auth validation specific
  valid?: boolean;
  user_id?: string;
  keycloak_user_id?: string;
  tenant_id?: string;
  tenant_slug?: string;
  email?: string;
  first_name?: string;
  last_name?: string;
  role?: string;
  mfa_required?: boolean;
  mfa_enabled?: boolean;
  account_locked?: boolean;
  locked_until?: string;
  remaining_attempts?: number;
  access_token?: string;
  refresh_token?: string;
  id_token?: string;
  expires_in?: number;
  // Account status specific
  account_exists?: boolean;
  // Account deactivation specific
  is_deactivated?: boolean;
  can_reactivate?: boolean;
  days_until_purge?: number;
  deactivated_at?: string;
  purge_date?: string;
  scheduled_purge_at?: string;
  // Password reset specific (valid already defined above)
  expires_at?: string;
}

// ============================================================================
// Client Implementation
// ============================================================================

class TenantServiceClient {
  private baseUrl: string;
  private timeout: number;

  constructor() {
    this.baseUrl = config.tenantServiceUrl;
    this.timeout = 10000; // 10 second timeout
  }

  /**
   * Get all tenants a user has access to (for tenant selection during login)
   * This is called when a user enters their email to show available tenants
   */
  async getUserTenants(email: string): Promise<GetUserTenantsResponse> {
    try {
      logger.info({ email: this.maskEmail(email) }, 'Looking up user tenants');

      const response = await this.fetch('/api/v1/auth/tenants', {
        method: 'POST',
        body: JSON.stringify({ email }),
      });

      const data = await response.json() as ApiResponse;

      if (!response.ok) {
        logger.warn({ status: response.status, email: this.maskEmail(email) }, 'Failed to get user tenants');
        return {
          success: false,
          message: data.message || 'Failed to retrieve tenants',
        };
      }

      const responseData = data.data as { tenants?: TenantInfo[]; count?: number } | undefined;
      logger.info({ email: this.maskEmail(email), count: responseData?.count || 0 }, 'User tenants retrieved');

      return {
        success: true,
        data: {
          tenants: responseData?.tenants || [],
          count: responseData?.count || 0,
        },
      };
    } catch (error) {
      logger.error({ error, email: this.maskEmail(email) }, 'Error getting user tenants');
      return {
        success: false,
        message: 'Service temporarily unavailable',
      };
    }
  }

  /**
   * Validate credentials for a specific tenant
   * Returns user info and Keycloak tokens if successful
   */
  async validateCredentials(
    request: ValidateCredentialsRequest,
    clientIP?: string,
    userAgent?: string
  ): Promise<ValidateCredentialsResponse> {
    try {
      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
        tenant_id: request.tenant_id,
      }, 'Validating credentials');

      const response = await this.fetch('/api/v1/auth/validate', {
        method: 'POST',
        body: JSON.stringify({
          ...request,
          ip_address: clientIP,
          user_agent: userAgent,
        }),
      });

      const data = await response.json() as ApiResponse;
      const responseData = data.data as ApiResponse | undefined;

      // Handle different response codes
      if (response.status === 401) {
        // Authentication failed - valid response, not an error
        logger.info({
          email: this.maskEmail(request.email),
          tenant_slug: request.tenant_slug,
          error_code: data.error_code,
          account_locked: data.account_locked,
        }, 'Credential validation failed');

        return {
          success: true,
          data: {
            valid: false,
            tenant_id: data.tenant_id || '',
            tenant_slug: data.tenant_slug || '',
            email: request.email,
            mfa_required: false,
            mfa_enabled: false,
            account_locked: data.account_locked || false,
            locked_until: data.locked_until,
            remaining_attempts: data.remaining_attempts,
            error_code: data.error_code,
            message: data.message,
          },
        };
      }

      if (!response.ok) {
        logger.error({
          status: response.status,
          email: this.maskEmail(request.email),
        }, 'Credential validation request failed');

        return {
          success: false,
          message: data.message || 'Validation failed',
          error_code: 'SERVICE_ERROR',
        };
      }

      // Successful validation
      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
        user_id: responseData?.user_id,
        mfa_required: responseData?.mfa_required,
        has_tokens: !!responseData?.access_token,
        role: responseData?.role,
        raw_data: JSON.stringify(data),
      }, 'Credentials validated successfully');

      return {
        success: true,
        data: {
          valid: true,
          user_id: responseData?.user_id,
          keycloak_user_id: responseData?.keycloak_user_id,
          tenant_id: responseData?.tenant_id || '',
          tenant_slug: responseData?.tenant_slug || '',
          email: responseData?.email || request.email,
          first_name: responseData?.first_name,
          last_name: responseData?.last_name,
          role: responseData?.role,
          mfa_required: responseData?.mfa_required || false,
          mfa_enabled: responseData?.mfa_enabled || false,
          account_locked: false,
          access_token: responseData?.access_token,
          refresh_token: responseData?.refresh_token,
          id_token: responseData?.id_token,
          expires_in: responseData?.expires_in,
        },
      };
    } catch (error) {
      logger.error({
        error,
        email: this.maskEmail(request.email),
      }, 'Error validating credentials');

      return {
        success: false,
        message: 'Authentication service temporarily unavailable',
        error_code: 'SERVICE_UNAVAILABLE',
      };
    }
  }

  /**
   * Check account status (locked/unlocked) without validating password
   * Used to show users if their account is locked before they enter password
   */
  async checkAccountStatus(
    email: string,
    tenantSlug: string
  ): Promise<AccountStatusResponse> {
    try {
      const response = await this.fetch('/api/v1/auth/account-status', {
        method: 'POST',
        body: JSON.stringify({ email, tenant_slug: tenantSlug }),
      });

      const data = await response.json() as ApiResponse;

      return {
        success: data.success || false,
        account_exists: data.account_exists || false,
        account_locked: data.account_locked || false,
        locked_until: data.locked_until,
        remaining_attempts: data.remaining_attempts,
      };
    } catch (error) {
      logger.error({ error }, 'Error checking account status');
      return {
        success: false,
        account_exists: false,
        account_locked: false,
      };
    }
  }

  // ============================================================================
  // Private Helpers
  // ============================================================================

  private async fetch(path: string, options: RequestInit): Promise<Response> {
    const url = `${this.baseUrl}${path}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
          'X-Service-Name': 'auth-bff',
          ...options.headers,
        },
      });
      return response;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Register a new customer for direct storefront registration
   */
  async registerCustomer(
    request: RegisterCustomerRequest,
    clientIP?: string,
    userAgent?: string
  ): Promise<RegisterCustomerResponse> {
    try {
      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
      }, 'Registering new customer');

      const response = await this.fetch('/api/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify({
          ...request,
          ip_address: clientIP,
          user_agent: userAgent,
        }),
      });

      const data = await response.json() as ApiResponse;
      const responseData = data.data as ApiResponse | undefined;

      // Handle different response codes
      if (response.status === 409) {
        // Email already exists
        logger.info({
          email: this.maskEmail(request.email),
          tenant_slug: request.tenant_slug,
        }, 'Registration failed: email exists');

        return {
          success: true,
          data: {
            tenant_id: '',
            tenant_slug: request.tenant_slug,
            email: request.email,
            error_code: data.error_code || 'EMAIL_EXISTS',
            message: data.message || 'An account with this email already exists',
          },
        };
      }

      if (response.status === 400) {
        // Validation error
        logger.info({
          email: this.maskEmail(request.email),
          tenant_slug: request.tenant_slug,
          error_code: data.error_code,
        }, 'Registration failed: validation error');

        return {
          success: true,
          data: {
            tenant_id: '',
            tenant_slug: request.tenant_slug,
            email: request.email,
            error_code: data.error_code || 'VALIDATION_ERROR',
            message: data.message || 'Invalid registration data',
          },
        };
      }

      if (!response.ok) {
        logger.error({
          status: response.status,
          email: this.maskEmail(request.email),
        }, 'Registration request failed');

        return {
          success: false,
          message: data.message || 'Registration failed',
          error_code: 'SERVICE_ERROR',
        };
      }

      // Successful registration
      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
        user_id: responseData?.user_id,
        has_tokens: !!responseData?.access_token,
      }, 'Customer registered successfully');

      return {
        success: true,
        data: {
          user_id: responseData?.user_id,
          tenant_id: responseData?.tenant_id || '',
          tenant_slug: responseData?.tenant_slug || request.tenant_slug,
          email: responseData?.email || request.email,
          first_name: responseData?.first_name,
          last_name: responseData?.last_name,
          access_token: responseData?.access_token,
          refresh_token: responseData?.refresh_token,
          id_token: responseData?.id_token,
          expires_in: responseData?.expires_in,
        },
      };
    } catch (error) {
      logger.error({
        error,
        email: this.maskEmail(request.email),
      }, 'Error registering customer');

      return {
        success: false,
        message: 'Registration service temporarily unavailable',
        error_code: 'SERVICE_UNAVAILABLE',
      };
    }
  }

  /**
   * Check if an account is deactivated (for login flow)
   * Used to show users reactivation option if account is deactivated but within retention period
   */
  async checkDeactivatedAccount(
    email: string,
    tenantSlug: string
  ): Promise<CheckDeactivatedResponse> {
    try {
      logger.info({
        email: this.maskEmail(email),
        tenant_slug: tenantSlug,
      }, 'Checking account deactivation status');

      const response = await this.fetch('/api/v1/auth/check-deactivated', {
        method: 'POST',
        body: JSON.stringify({ email, tenant_slug: tenantSlug }),
      });

      const data = await response.json() as ApiResponse;

      if (!response.ok) {
        return {
          success: false,
          is_deactivated: false,
          can_reactivate: false,
          error_code: data.error_code || 'SERVICE_ERROR',
          message: data.message || 'Failed to check account status',
        };
      }

      return {
        success: true,
        is_deactivated: data.is_deactivated || false,
        can_reactivate: data.can_reactivate || false,
        days_until_purge: data.days_until_purge,
        deactivated_at: data.deactivated_at,
        purge_date: data.purge_date,
      };
    } catch (error) {
      logger.error({ error }, 'Error checking deactivation status');
      return {
        success: false,
        is_deactivated: false,
        can_reactivate: false,
        error_code: 'SERVICE_UNAVAILABLE',
        message: 'Service temporarily unavailable',
      };
    }
  }

  /**
   * Reactivate a deactivated account within the retention period
   * Requires password verification
   */
  async reactivateAccount(
    request: ReactivateAccountRequest
  ): Promise<ReactivateAccountResponse> {
    try {
      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
      }, 'Attempting to reactivate account');

      const response = await this.fetch('/api/v1/auth/reactivate-account', {
        method: 'POST',
        body: JSON.stringify(request),
      });

      const data = await response.json() as ApiResponse;

      if (!response.ok) {
        logger.warn({
          email: this.maskEmail(request.email),
          error_code: data.error_code,
        }, 'Account reactivation failed');

        return {
          success: false,
          error_code: data.error_code || 'REACTIVATION_FAILED',
          error_message: data.message || 'Failed to reactivate account',
        };
      }

      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
      }, 'Account reactivated successfully');

      return {
        success: true,
        message: data.message || 'Account reactivated successfully',
      };
    } catch (error) {
      logger.error({ error }, 'Error reactivating account');
      return {
        success: false,
        error_code: 'SERVICE_UNAVAILABLE',
        error_message: 'Service temporarily unavailable',
      };
    }
  }

  /**
   * Deactivate a customer account (self-service)
   * Requires authenticated user context
   */
  async deactivateAccount(
    request: DeactivateAccountRequest
  ): Promise<DeactivateAccountResponse> {
    try {
      logger.info({
        user_id: request.user_id,
        tenant_id: request.tenant_id,
        reason: request.reason,
      }, 'Attempting to deactivate account');

      const response = await this.fetch('/api/v1/auth/deactivate-account', {
        method: 'POST',
        body: JSON.stringify(request),
      });

      const data = await response.json() as ApiResponse;

      if (!response.ok) {
        logger.warn({
          user_id: request.user_id,
          error_code: data.error_code,
        }, 'Account deactivation failed');

        return {
          success: false,
          error_code: data.error_code || 'DEACTIVATION_FAILED',
          message: data.message || 'Failed to deactivate account',
        };
      }

      logger.info({
        user_id: request.user_id,
        tenant_id: request.tenant_id,
      }, 'Account deactivated successfully');

      return {
        success: true,
        deactivated_at: data.deactivated_at,
        scheduled_purge_at: data.scheduled_purge_at,
        days_until_purge: data.days_until_purge,
        message: data.message || 'Account deactivated successfully',
      };
    } catch (error) {
      logger.error({ error }, 'Error deactivating account');
      return {
        success: false,
        error_code: 'SERVICE_UNAVAILABLE',
        message: 'Service temporarily unavailable',
      };
    }
  }

  // ============================================================================
  // Password Reset Methods
  // ============================================================================

  /**
   * Request a password reset email
   * Always returns success to not reveal if email exists
   */
  async requestPasswordReset(
    request: RequestPasswordResetRequest
  ): Promise<RequestPasswordResetResponse> {
    try {
      logger.info({
        email: this.maskEmail(request.email),
        tenant_slug: request.tenant_slug,
      }, 'Requesting password reset');

      const response = await this.fetch('/api/v1/auth/request-password-reset', {
        method: 'POST',
        body: JSON.stringify(request),
      });

      const data = await response.json() as ApiResponse;

      // Always return success to not reveal if email exists
      return {
        success: true,
        message: data.message || 'If an account exists with this email, you will receive a password reset link shortly.',
      };
    } catch (error) {
      logger.error({ error }, 'Error requesting password reset');
      // Still return success to not reveal errors
      return {
        success: true,
        message: 'If an account exists with this email, you will receive a password reset link shortly.',
      };
    }
  }

  /**
   * Validate a password reset token
   * Returns whether the token is valid and the masked email
   */
  async validateResetToken(
    token: string
  ): Promise<ValidateResetTokenResponse> {
    try {
      logger.info('Validating password reset token');

      const response = await this.fetch('/api/v1/auth/validate-reset-token', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });

      const data = await response.json() as ApiResponse;

      if (!response.ok) {
        return {
          success: false,
          valid: false,
          message: data.message || 'Invalid or expired reset link.',
          error_code: data.error_code || 'SERVICE_ERROR',
        };
      }

      return {
        success: true,
        valid: data.valid || false,
        email: data.email,
        expires_at: data.expires_at,
        message: data.message,
      };
    } catch (error) {
      logger.error({ error }, 'Error validating reset token');
      return {
        success: false,
        valid: false,
        error_code: 'SERVICE_UNAVAILABLE',
        message: 'Service temporarily unavailable',
      };
    }
  }

  /**
   * Reset password using a valid token
   * Returns success if password was reset successfully
   */
  async resetPassword(
    request: ResetPasswordRequest
  ): Promise<ResetPasswordResponse> {
    try {
      logger.info('Resetting password with token');

      const response = await this.fetch('/api/v1/auth/reset-password', {
        method: 'POST',
        body: JSON.stringify(request),
      });

      const data = await response.json() as ApiResponse;

      if (!response.ok) {
        logger.warn({ error_code: data.error_code }, 'Password reset failed');
        return {
          success: false,
          error_code: data.error_code || 'RESET_FAILED',
          message: data.message || 'Failed to reset password. The link may be invalid or expired.',
        };
      }

      logger.info('Password reset successful');
      return {
        success: true,
        message: data.message || 'Your password has been reset successfully.',
      };
    } catch (error) {
      logger.error({ error }, 'Error resetting password');
      return {
        success: false,
        error_code: 'SERVICE_UNAVAILABLE',
        message: 'Service temporarily unavailable',
      };
    }
  }

  /**
   * Mask email for logging (privacy protection)
   */
  private maskEmail(email: string): string {
    const [local, domain] = email.split('@');
    if (!domain) return '***';
    const maskedLocal = local.length > 2
      ? `${local[0]}${'*'.repeat(local.length - 2)}${local[local.length - 1]}`
      : '***';
    return `${maskedLocal}@${domain}`;
  }
}

// Export singleton instance
export const tenantServiceClient = new TenantServiceClient();

/**
 * Mask email for logging (privacy protection)
 * Standalone utility for use across the auth-bff service
 * Example: john.doe@example.com → j******e@example.com
 */
export function maskEmail(email: string): string {
  if (!email) return '***';
  const [local, domain] = email.split('@');
  if (!domain) return '***';
  const maskedLocal = local.length > 2
    ? `${local[0]}${'*'.repeat(local.length - 2)}${local[local.length - 1]}`
    : '***';
  return `${maskedLocal}@${domain}`;
}
