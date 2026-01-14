import { Issuer, Client, generators, TokenSet, custom, IssuerMetadata } from 'openid-client';
import { config } from './config';
import { createLogger } from './logger';

const logger = createLogger('oidc-client');

// Configure custom HTTP options to set User-Agent header
// This is required because some WAF/CDN configurations block empty or "node" User-Agents
const CUSTOM_USER_AGENT = 'Mozilla/5.0 (compatible; auth-bff/1.0; +https://tesserix.app)';

/**
 * Discover OIDC issuer with custom User-Agent header.
 * The openid-client library's default User-Agent gets blocked by WAF/CDN.
 * We fetch the discovery document manually and create the Issuer from metadata.
 */
async function discoverIssuerWithCustomUserAgent(issuerUrl: string): Promise<Issuer> {
  const discoveryUrl = issuerUrl.endsWith('/')
    ? `${issuerUrl}.well-known/openid-configuration`
    : `${issuerUrl}/.well-known/openid-configuration`;

  logger.debug({ discoveryUrl }, 'Fetching OIDC discovery document with custom User-Agent');

  const response = await fetch(discoveryUrl, {
    method: 'GET',
    headers: {
      'User-Agent': CUSTOM_USER_AGENT,
      'Accept': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`OIDC discovery failed: ${response.status} ${response.statusText}`);
  }

  const metadata = await response.json() as IssuerMetadata;
  logger.debug({ issuer: metadata.issuer }, 'OIDC discovery document fetched successfully');

  // Create Issuer from the fetched metadata
  const issuer = new Issuer(metadata);

  // Set custom http_options on the issuer instance for subsequent requests
  issuer[custom.http_options] = function httpOptionsHook(_url: URL, options: Record<string, unknown>) {
    const headers = (options.headers || {}) as Record<string, string>;
    headers['User-Agent'] = CUSTOM_USER_AGENT;
    return { ...options, headers };
  };

  return issuer;
}

export interface OIDCClientConfig {
  issuerUrl: string;
  clientId: string;
  clientSecret: string;
  realm: string;
}

export interface AuthorizationParams {
  redirectUri: string;
  scope?: string;
  state?: string;
  nonce?: string;
  codeVerifier?: string;
  prompt?: 'none' | 'login' | 'consent' | 'select_account';
  loginHint?: string;
  uiLocales?: string;
  kcIdpHint?: string; // Keycloak identity provider hint - redirects directly to the IDP (e.g., 'google')
  kcAction?: string; // Keycloak action - e.g., 'register' for direct registration, 'UPDATE_PASSWORD' for password reset
}

export interface TokenResponse {
  accessToken: string;
  idToken?: string;
  refreshToken?: string;
  expiresAt: number;
  tokenType: string;
  scope?: string;
}

class OIDCClientManager {
  private clients: Map<string, Client> = new Map();
  private issuers: Map<string, Issuer> = new Map();
  private initializationPromises: Map<string, Promise<Client>> = new Map();
  private initialized = false;

  /**
   * Initialize OIDC clients on startup to avoid rate limiting during request handling.
   * Should be called once during application startup.
   */
  async initialize(): Promise<void> {
    if (this.initialized) {
      logger.debug('OIDC clients already initialized');
      return;
    }

    logger.info('Initializing OIDC clients on startup...');

    // Only initialize customer client for now (internal realm not configured yet)
    const clientTypes: Array<'internal' | 'customer'> = ['customer'];

    for (const type of clientTypes) {
      try {
        await this.getClient(type);
        logger.info({ type }, 'OIDC client initialized successfully');
      } catch (error) {
        logger.error({ type, error }, 'Failed to initialize OIDC client - will retry on first request');
      }
    }

    this.initialized = true;
    logger.info('OIDC client initialization complete');
  }

  async getClient(type: 'internal' | 'customer'): Promise<Client> {
    const key = type;
    let client = this.clients.get(key);

    if (client) {
      return client;
    }

    // Use a promise lock to prevent concurrent discovery calls
    let initPromise = this.initializationPromises.get(key);
    if (initPromise) {
      logger.debug({ type }, 'Waiting for existing initialization');
      return initPromise;
    }

    // Create initialization promise
    initPromise = this.initializeClientWithRetry(type);
    this.initializationPromises.set(key, initPromise);

    try {
      client = await initPromise;
      this.clients.set(key, client);
      return client;
    } finally {
      this.initializationPromises.delete(key);
    }
  }

  private async initializeClientWithRetry(
    type: 'internal' | 'customer',
    maxRetries = 3,
    baseDelayMs = 1000
  ): Promise<Client> {
    let lastError: Error | undefined;

    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await this.initializeClient(type);
      } catch (error) {
        lastError = error as Error;
        const isRateLimit = lastError.message?.includes('rate') || lastError.message?.includes('429');

        if (attempt < maxRetries) {
          // Exponential backoff with jitter
          const delay = baseDelayMs * Math.pow(2, attempt - 1) + Math.random() * 1000;
          logger.warn(
            { type, attempt, maxRetries, delay, error: lastError.message, isRateLimit },
            'OIDC discovery failed, retrying...'
          );
          await new Promise(resolve => setTimeout(resolve, delay));
        }
      }
    }

    logger.error({ type, error: lastError }, 'OIDC discovery failed after all retries');
    throw lastError;
  }

  private async initializeClient(type: 'internal' | 'customer'): Promise<Client> {
    const keycloakConfig = config.keycloak[type];

    logger.info({ type, issuer: keycloakConfig.issuer }, 'Discovering OIDC issuer');

    let issuer = this.issuers.get(type);
    if (!issuer) {
      // Use custom discovery with proper User-Agent to avoid WAF/CDN blocking
      issuer = await discoverIssuerWithCustomUserAgent(keycloakConfig.issuer);
      this.issuers.set(type, issuer);
      logger.info(
        {
          type,
          issuer: issuer.issuer,
          authorizationEndpoint: issuer.metadata.authorization_endpoint,
          tokenEndpoint: issuer.metadata.token_endpoint,
        },
        'OIDC issuer discovered'
      );
    }

    const client = new issuer.Client({
      client_id: keycloakConfig.clientId,
      client_secret: keycloakConfig.clientSecret,
      redirect_uris: [],
      response_types: ['code'],
      token_endpoint_auth_method: 'client_secret_post',
    });

    // Set clock tolerance for token validation
    (client as unknown as Record<symbol, number>)[Symbol.for('openid-client.clock_tolerance')] = 10;

    // Set custom http_options on the client instance for token exchange and other requests
    // This is required because WAF/CDN blocks the openid-client default User-Agent
    client[custom.http_options] = function httpOptionsHook(_url: URL, options: Record<string, unknown>) {
      const headers = (options.headers || {}) as Record<string, string>;
      headers['User-Agent'] = CUSTOM_USER_AGENT;
      return { ...options, headers };
    };

    return client;
  }

  generateState(): string {
    return generators.state();
  }

  generateNonce(): string {
    return generators.nonce();
  }

  generateCodeVerifier(): string {
    return generators.codeVerifier();
  }

  generateCodeChallenge(verifier: string): string {
    return generators.codeChallenge(verifier);
  }

  async getAuthorizationUrl(
    type: 'internal' | 'customer',
    params: AuthorizationParams
  ): Promise<string> {
    const client = await this.getClient(type);

    const codeChallenge = params.codeVerifier
      ? this.generateCodeChallenge(params.codeVerifier)
      : undefined;

    const url = client.authorizationUrl({
      redirect_uri: params.redirectUri,
      scope: params.scope || 'openid profile email',
      state: params.state,
      nonce: params.nonce,
      code_challenge: codeChallenge,
      code_challenge_method: codeChallenge ? 'S256' : undefined,
      prompt: params.prompt,
      login_hint: params.loginHint,
      ui_locales: params.uiLocales,
      kc_idp_hint: params.kcIdpHint, // Keycloak IDP hint - skips Keycloak login page and redirects directly to the IDP
      kc_action: params.kcAction, // Keycloak action - e.g., 'register' for registration, 'UPDATE_PASSWORD' for reset
    });

    logger.debug({ type, url: url.substring(0, 100) + '...', kcIdpHint: params.kcIdpHint }, 'Generated authorization URL');

    return url;
  }

  async exchangeCode(
    type: 'internal' | 'customer',
    callbackParams: {
      code: string;
      state: string;
      iss?: string;
      session_state?: string;
    },
    redirectUri: string,
    codeVerifier?: string,
    nonce?: string
  ): Promise<TokenResponse> {
    const client = await this.getClient(type);

    logger.debug({ type }, 'Exchanging authorization code for tokens');

    const tokenSet = await client.callback(
      redirectUri,
      callbackParams,
      {
        code_verifier: codeVerifier,
        nonce,
        state: callbackParams.state,
      }
    );

    return this.tokenSetToResponse(tokenSet);
  }

  async refreshTokens(
    type: 'internal' | 'customer',
    refreshToken: string
  ): Promise<TokenResponse> {
    const client = await this.getClient(type);

    logger.debug({ type }, 'Refreshing tokens');

    const tokenSet = await client.refresh(refreshToken);

    return this.tokenSetToResponse(tokenSet);
  }

  async getUserInfo(
    type: 'internal' | 'customer',
    accessToken: string
  ): Promise<Record<string, unknown>> {
    const client = await this.getClient(type);

    logger.debug({ type }, 'Fetching user info');

    const userInfo = await client.userinfo(accessToken);

    return userInfo;
  }

  async revokeToken(
    type: 'internal' | 'customer',
    token: string,
    tokenTypeHint?: 'access_token' | 'refresh_token'
  ): Promise<void> {
    const client = await this.getClient(type);

    logger.debug({ type, tokenTypeHint }, 'Revoking token');

    await client.revoke(token, tokenTypeHint);
  }

  getEndSessionUrl(
    type: 'internal' | 'customer',
    idTokenHint?: string,
    postLogoutRedirectUri?: string,
    state?: string
  ): string {
    const issuer = this.issuers.get(type);
    if (!issuer || !issuer.metadata.end_session_endpoint) {
      throw new Error(`End session endpoint not available for ${type}`);
    }

    const url = new URL(issuer.metadata.end_session_endpoint);

    if (idTokenHint) {
      url.searchParams.set('id_token_hint', idTokenHint);
    }
    if (postLogoutRedirectUri) {
      url.searchParams.set('post_logout_redirect_uri', postLogoutRedirectUri);
    }
    if (state) {
      url.searchParams.set('state', state);
    }

    return url.toString();
  }

  async introspect(
    type: 'internal' | 'customer',
    token: string
  ): Promise<{ active: boolean; [key: string]: unknown }> {
    const client = await this.getClient(type);

    logger.debug({ type }, 'Introspecting token');

    const result = await client.introspect(token);

    return result as { active: boolean; [key: string]: unknown };
  }

  private tokenSetToResponse(tokenSet: TokenSet): TokenResponse {
    return {
      accessToken: tokenSet.access_token!,
      idToken: tokenSet.id_token,
      refreshToken: tokenSet.refresh_token,
      expiresAt: tokenSet.expires_at || Math.floor(Date.now() / 1000) + (tokenSet.expires_in || 300),
      tokenType: tokenSet.token_type || 'Bearer',
      scope: tokenSet.scope,
    };
  }
}

export const oidcClient = new OIDCClientManager();
