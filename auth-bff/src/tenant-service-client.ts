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
