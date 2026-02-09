import { Issuer, Client, generators, TokenSet, custom, IssuerMetadata } from 'openid-client';
import { config } from './config';
import { createLogger } from './logger';

const logger = createLogger('logto-client');

const CUSTOM_USER_AGENT = `Mozilla/5.0 (compatible; auth-bff/1.0; +https://${config.baseDomain})`;

/**
 * Discover Logto OIDC issuer with custom User-Agent header.
 */
async function discoverLogtoIssuer(endpoint: string): Promise<Issuer> {
  const discoveryUrl = `${endpoint}/oidc/.well-known/openid-configuration`;

  logger.debug({ discoveryUrl }, 'Fetching Logto OIDC discovery document');

  const response = await fetch(discoveryUrl, {
    method: 'GET',
    headers: {
      'User-Agent': CUSTOM_USER_AGENT,
      'Accept': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Logto OIDC discovery failed: ${response.status} ${response.statusText}`);
  }

  const metadata = await response.json() as IssuerMetadata;
  logger.debug({ issuer: metadata.issuer }, 'Logto OIDC discovery document fetched');

  const issuer = new Issuer(metadata);

  issuer[custom.http_options] = function httpOptionsHook(_url: URL, options: Record<string, unknown>) {
    const headers = (options.headers || {}) as Record<string, string>;
    headers['User-Agent'] = CUSTOM_USER_AGENT;
    return { ...options, headers };
  };

  return issuer;
}

export interface LogtoAuthorizationParams {
  redirectUri: string;
  scope?: string;
  state?: string;
  nonce?: string;
  codeVerifier?: string;
  prompt?: 'login' | 'consent';
  loginHint?: string;
  resource?: string;
}

export interface LogtoTokenResponse {
  accessToken: string;
  idToken?: string;
  refreshToken?: string;
  expiresAt: number;
  tokenType: string;
  scope?: string;
}

class LogtoClientManager {
  private client: Client | null = null;
  private issuer: Issuer | null = null;
  private initializationPromise: Promise<Client> | null = null;
  private initialized = false;

  /**
   * Initialize the Logto OIDC client on startup.
   * Skips initialization if Logto is not configured.
   */
  async initialize(): Promise<void> {
    if (!config.logto.endpoint || !config.logto.home.clientId) {
      logger.info('Logto not configured — skipping initialization');
      return;
    }

    if (this.initialized) {
      logger.debug('Logto client already initialized');
      return;
    }

    logger.info('Initializing Logto OIDC client...');

    try {
      await this.getClient();
      this.initialized = true;
      logger.info('Logto OIDC client initialized successfully');
    } catch (error) {
      logger.error({ error }, 'Failed to initialize Logto OIDC client — will retry on first request');
    }
  }

  async getClient(): Promise<Client> {
    if (this.client) {
      return this.client;
    }

    if (this.initializationPromise) {
      logger.debug('Waiting for existing Logto initialization');
      return this.initializationPromise;
    }

    this.initializationPromise = this.initializeClientWithRetry();

    try {
      this.client = await this.initializationPromise;
      return this.client;
    } finally {
      this.initializationPromise = null;
    }
  }

  private async initializeClientWithRetry(maxRetries = 3, baseDelayMs = 1000): Promise<Client> {
    let lastError: Error | undefined;

    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await this.initializeClient();
      } catch (error) {
        lastError = error as Error;
        if (attempt < maxRetries) {
          const delay = baseDelayMs * Math.pow(2, attempt - 1) + Math.random() * 1000;
          logger.warn(
            { attempt, maxRetries, delay, error: lastError.message },
            'Logto OIDC discovery failed, retrying...'
          );
          await new Promise(resolve => setTimeout(resolve, delay));
        }
      }
    }

    logger.error({ error: lastError }, 'Logto OIDC discovery failed after all retries');
    throw lastError;
  }

  private async initializeClient(): Promise<Client> {
    const { endpoint, home } = config.logto;

    logger.info({ endpoint }, 'Discovering Logto OIDC issuer');

    if (!this.issuer) {
      this.issuer = await discoverLogtoIssuer(endpoint);
      logger.info(
        {
          issuer: this.issuer.issuer,
          authorizationEndpoint: this.issuer.metadata.authorization_endpoint,
          tokenEndpoint: this.issuer.metadata.token_endpoint,
        },
        'Logto OIDC issuer discovered'
      );
    }

    const client = new this.issuer.Client({
      client_id: home.clientId,
      client_secret: home.clientSecret,
      redirect_uris: [],
      response_types: ['code'],
      token_endpoint_auth_method: 'client_secret_post',
    });

    // Set clock tolerance
    (client as unknown as Record<symbol, number>)[Symbol.for('openid-client.clock_tolerance')] = 10;

    client[custom.http_options] = function httpOptionsHook(_url: URL, options: Record<string, unknown>) {
      const headers = (options.headers || {}) as Record<string, string>;
      headers['User-Agent'] = CUSTOM_USER_AGENT;
      return { ...options, headers };
    };

    return client;
  }

  get isConfigured(): boolean {
    return !!(config.logto.endpoint && config.logto.home.clientId);
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

  async getAuthorizationUrl(params: LogtoAuthorizationParams): Promise<string> {
    const client = await this.getClient();

    const codeChallenge = params.codeVerifier
      ? this.generateCodeChallenge(params.codeVerifier)
      : undefined;

    const url = client.authorizationUrl({
      redirect_uri: params.redirectUri,
      scope: params.scope || 'openid profile email offline_access',
      state: params.state,
      nonce: params.nonce,
      code_challenge: codeChallenge,
      code_challenge_method: codeChallenge ? 'S256' : undefined,
      prompt: params.prompt,
      login_hint: params.loginHint,
      resource: params.resource,
    });

    logger.debug({ url: url.substring(0, 100) + '...' }, 'Generated Logto authorization URL');

    return url;
  }

  async exchangeCode(
    callbackParams: {
      code: string;
      state: string;
    },
    redirectUri: string,
    codeVerifier?: string,
    nonce?: string
  ): Promise<LogtoTokenResponse> {
    const client = await this.getClient();

    logger.debug('Exchanging Logto authorization code for tokens');

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

  async refreshTokens(refreshToken: string): Promise<LogtoTokenResponse> {
    const client = await this.getClient();

    logger.debug('Refreshing Logto tokens');

    const tokenSet = await client.refresh(refreshToken);

    return this.tokenSetToResponse(tokenSet);
  }

  async getUserInfo(accessToken: string): Promise<Record<string, unknown>> {
    const client = await this.getClient();

    logger.debug('Fetching Logto user info');

    const userInfo = await client.userinfo(accessToken);

    return userInfo;
  }

  async revokeToken(token: string, tokenTypeHint?: 'access_token' | 'refresh_token'): Promise<void> {
    const client = await this.getClient();

    logger.debug({ tokenTypeHint }, 'Revoking Logto token');

    await client.revoke(token, tokenTypeHint);
  }

  getEndSessionUrl(
    idTokenHint?: string,
    postLogoutRedirectUri?: string
  ): string {
    if (!this.issuer || !this.issuer.metadata.end_session_endpoint) {
      throw new Error('Logto end session endpoint not available');
    }

    const url = new URL(this.issuer.metadata.end_session_endpoint);

    if (idTokenHint) {
      url.searchParams.set('id_token_hint', idTokenHint);
    }
    if (postLogoutRedirectUri) {
      url.searchParams.set('post_logout_redirect_uri', postLogoutRedirectUri);
    }

    return url.toString();
  }

  private tokenSetToResponse(tokenSet: TokenSet): LogtoTokenResponse {
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

export const logtoClient = new LogtoClientManager();
