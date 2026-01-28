import Redis from 'ioredis';
import { config } from './config';
import { createLogger } from './logger';
import { v4 as uuidv4 } from 'uuid';

const logger = createLogger('session-store');

export interface SessionData {
  id: string;
  userId: string;
  tenantId?: string;
  tenantSlug?: string;
  clientType: 'internal' | 'customer';
  accessToken: string;
  idToken?: string;
  refreshToken?: string;
  expiresAt: number;
  userInfo?: Record<string, unknown>;
  createdAt: number;
  lastAccessedAt: number;
  csrfToken: string;
}

export interface AuthFlowState {
  state: string;
  nonce: string;
  codeVerifier: string;
  redirectUri: string;
  clientType: 'internal' | 'customer';
  returnTo?: string;
  // Tenant context for multi-tenant storefront authentication
  // These are passed by the storefront and used to scope the session
  tenantId?: string;
  tenantSlug?: string;
  createdAt: number;
}

// WebSocket ticket for secure WS authentication without exposing tokens
export interface WsTicketData {
  userId: string;
  tenantId?: string;
  tenantSlug?: string;
  sessionId: string;
  createdAt: number;
}

// Session transfer for cross-subdomain authentication (e.g., onboarding → admin)
export interface SessionTransferData {
  sessionId: string;
  userId: string;
  tenantId?: string;
  tenantSlug?: string;
  clientType: 'internal' | 'customer';
  accessToken: string;
  idToken?: string;
  refreshToken?: string;
  expiresAt: number;
  userInfo?: Record<string, unknown>;
  createdAt: number;
}

// MFA session for step-up authentication
export interface MfaSessionData {
  userId: string;
  email: string;
  tenantId: string;
  tenantSlug: string;
  mfaEnabled: boolean;
  attemptCount?: number;
  createdAt: number;
  // Stored after successful password auth for deferred session creation
  accessToken?: string;
  idToken?: string;
  refreshToken?: string;
  keycloakUserId?: string;
  firstName?: string;
  lastName?: string;
  role?: string;
  rememberMe?: boolean;
}

// Device trust for MFA bypass on remembered devices
export interface DeviceTrustData {
  userId: string;
  tenantId: string;
  userAgent: string;
  ipAddress: string;
  createdAt: number;
}

class SessionStore {
  private redis: Redis;
  private readonly SESSION_PREFIX = 'bff:session:';
  private readonly AUTH_FLOW_PREFIX = 'bff:auth_flow:';
  private readonly WS_TICKET_PREFIX = 'bff:ws_ticket:';
  private readonly SESSION_TRANSFER_PREFIX = 'bff:session_transfer:';
  private readonly MFA_SESSION_PREFIX = 'bff:mfa_session:';
  private readonly DEVICE_TRUST_PREFIX = 'bff:device_trust:';
  private readonly SESSION_TTL = config.session.maxAge;
  private readonly AUTH_FLOW_TTL = 600; // 10 minutes for auth flow state
  private readonly WS_TICKET_TTL = 30; // 30 seconds for WebSocket tickets
  private readonly SESSION_TRANSFER_TTL = 60; // 60 seconds for session transfer
  private readonly MFA_SESSION_TTL = 300; // 5 minutes for MFA session
  private readonly DEVICE_TRUST_TTL = 2592000; // 30 days for device trust

  constructor() {
    if (config.redis.url) {
      this.redis = new Redis(config.redis.url);
    } else {
      this.redis = new Redis({
        host: config.redis.host,
        port: config.redis.port,
        password: config.redis.password,
        maxRetriesPerRequest: 3,
        retryStrategy: (times) => Math.min(times * 100, 3000),
      });
    }

    this.redis.on('connect', () => {
      logger.info('Connected to Redis');
    });

    this.redis.on('error', (err) => {
      logger.error({ err }, 'Redis connection error');
    });

    this.redis.on('close', () => {
      logger.warn('Redis connection closed');
    });
  }

  // Session Management
  async createSession(data: Omit<SessionData, 'id' | 'createdAt' | 'lastAccessedAt' | 'csrfToken'>): Promise<SessionData> {
    const session: SessionData = {
      ...data,
      id: uuidv4(),
      createdAt: Date.now(),
      lastAccessedAt: Date.now(),
      csrfToken: uuidv4(),
    };

    await this.redis.setex(
      this.SESSION_PREFIX + session.id,
      this.SESSION_TTL,
      JSON.stringify(session)
    );

    logger.info({ sessionId: session.id, userId: session.userId }, 'Session created');

    return session;
  }

  async getSession(sessionId: string): Promise<SessionData | null> {
    const data = await this.redis.get(this.SESSION_PREFIX + sessionId);

    if (!data) {
      return null;
    }

    const session = JSON.parse(data) as SessionData;

    // Update last accessed time and extend TTL
    session.lastAccessedAt = Date.now();
    await this.redis.setex(
      this.SESSION_PREFIX + sessionId,
      this.SESSION_TTL,
      JSON.stringify(session)
    );

    return session;
  }

  async updateSession(sessionId: string, updates: Partial<SessionData>): Promise<SessionData | null> {
    const session = await this.getSession(sessionId);

    if (!session) {
      return null;
    }

    const updatedSession = {
      ...session,
      ...updates,
      lastAccessedAt: Date.now(),
    };

    await this.redis.setex(
      this.SESSION_PREFIX + sessionId,
      this.SESSION_TTL,
      JSON.stringify(updatedSession)
    );

    logger.debug({ sessionId }, 'Session updated');

    return updatedSession;
  }

  async deleteSession(sessionId: string): Promise<boolean> {
    const result = await this.redis.del(this.SESSION_PREFIX + sessionId);

    if (result > 0) {
      logger.info({ sessionId }, 'Session deleted');
      return true;
    }

    return false;
  }

  async deleteUserSessions(userId: string): Promise<number> {
    // Scan for all user sessions
    const pattern = this.SESSION_PREFIX + '*';
    let cursor = '0';
    let deletedCount = 0;

    do {
      const [newCursor, keys] = await this.redis.scan(cursor, 'MATCH', pattern, 'COUNT', 100);
      cursor = newCursor;

      for (const key of keys) {
        const data = await this.redis.get(key);
        if (data) {
          const session = JSON.parse(data) as SessionData;
          if (session.userId === userId) {
            await this.redis.del(key);
            deletedCount++;
          }
        }
      }
    } while (cursor !== '0');

    logger.info({ userId, deletedCount }, 'User sessions deleted');

    return deletedCount;
  }

  // Auth Flow State Management
  async saveAuthFlowState(state: AuthFlowState): Promise<void> {
    await this.redis.setex(
      this.AUTH_FLOW_PREFIX + state.state,
      this.AUTH_FLOW_TTL,
      JSON.stringify(state)
    );

    logger.debug({ state: state.state }, 'Auth flow state saved');
  }

  async getAuthFlowState(state: string): Promise<AuthFlowState | null> {
    const data = await this.redis.get(this.AUTH_FLOW_PREFIX + state);

    if (!data) {
      return null;
    }

    // Delete the state after retrieval (one-time use)
    await this.redis.del(this.AUTH_FLOW_PREFIX + state);

    return JSON.parse(data) as AuthFlowState;
  }

  // WebSocket Ticket Management
  async saveWsTicket(ticketId: string, data: WsTicketData): Promise<void> {
    await this.redis.setex(
      this.WS_TICKET_PREFIX + ticketId,
      this.WS_TICKET_TTL,
      JSON.stringify(data)
    );

    logger.debug({ ticketId, userId: data.userId }, 'WebSocket ticket saved');
  }

  async consumeWsTicket(ticketId: string): Promise<WsTicketData | null> {
    const data = await this.redis.get(this.WS_TICKET_PREFIX + ticketId);

    if (!data) {
      return null;
    }

    // Delete after consumption (one-time use)
    await this.redis.del(this.WS_TICKET_PREFIX + ticketId);

    logger.debug({ ticketId }, 'WebSocket ticket consumed');

    return JSON.parse(data) as WsTicketData;
  }

  // Session Transfer Management (for cross-subdomain auth)
  async saveSessionTransfer(code: string, data: SessionTransferData): Promise<void> {
    await this.redis.setex(
      this.SESSION_TRANSFER_PREFIX + code,
      this.SESSION_TRANSFER_TTL,
      JSON.stringify(data)
    );

    logger.debug({ code, userId: data.userId }, 'Session transfer saved');
  }

  async consumeSessionTransfer(code: string): Promise<SessionTransferData | null> {
    const data = await this.redis.get(this.SESSION_TRANSFER_PREFIX + code);

    if (!data) {
      return null;
    }

    // Delete after consumption (one-time use)
    await this.redis.del(this.SESSION_TRANSFER_PREFIX + code);

    logger.debug({ code }, 'Session transfer consumed');

    return JSON.parse(data) as SessionTransferData;
  }

  // MFA Session Management
  async saveMfaSession(sessionId: string, data: MfaSessionData): Promise<void> {
    await this.redis.setex(
      this.MFA_SESSION_PREFIX + sessionId,
      this.MFA_SESSION_TTL,
      JSON.stringify(data)
    );

    logger.debug({ sessionId, userId: data.userId }, 'MFA session saved');
  }

  async getMfaSession(sessionId: string): Promise<MfaSessionData | null> {
    const data = await this.redis.get(this.MFA_SESSION_PREFIX + sessionId);

    if (!data) {
      return null;
    }

    return JSON.parse(data) as MfaSessionData;
  }

  async updateMfaSession(sessionId: string, updates: Partial<MfaSessionData>): Promise<MfaSessionData | null> {
    const session = await this.getMfaSession(sessionId);

    if (!session) {
      return null;
    }

    const updatedSession = { ...session, ...updates };

    await this.redis.setex(
      this.MFA_SESSION_PREFIX + sessionId,
      this.MFA_SESSION_TTL,
      JSON.stringify(updatedSession)
    );

    return updatedSession;
  }

  async deleteMfaSession(sessionId: string): Promise<boolean> {
    const result = await this.redis.del(this.MFA_SESSION_PREFIX + sessionId);
    return result > 0;
  }

  // Device Trust Management
  async saveDeviceTrust(token: string, data: DeviceTrustData): Promise<void> {
    await this.redis.setex(
      this.DEVICE_TRUST_PREFIX + token,
      this.DEVICE_TRUST_TTL,
      JSON.stringify(data)
    );
    logger.debug({ token: token.slice(0, 8), userId: data.userId }, 'Device trust saved');
  }

  async getDeviceTrust(token: string): Promise<DeviceTrustData | null> {
    const data = await this.redis.get(this.DEVICE_TRUST_PREFIX + token);
    if (!data) return null;
    return JSON.parse(data) as DeviceTrustData;
  }

  async deleteDeviceTrust(token: string): Promise<boolean> {
    const result = await this.redis.del(this.DEVICE_TRUST_PREFIX + token);
    return result > 0;
  }

  // Health check
  async ping(): Promise<boolean> {
    try {
      const result = await this.redis.ping();
      return result === 'PONG';
    } catch {
      return false;
    }
  }

  async close(): Promise<void> {
    await this.redis.quit();
    logger.info('Redis connection closed gracefully');
  }
}

export const sessionStore = new SessionStore();
