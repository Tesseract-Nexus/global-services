import { connect, NatsConnection, JetStreamClient, StringCodec } from 'nats';
import { createLogger } from './logger';

const logger = createLogger('nats-client');

// Auth event types
export const AuthEventTypes = {
  LOGIN_SUCCESS: 'auth.login_success',
  LOGIN_FAILED: 'auth.login_failed',
  LOGOUT: 'auth.logout',
} as const;

// Auth event payload
export interface AuthEvent {
  eventType: string;
  tenantId: string;
  sourceId: string;
  timestamp: string;
  traceId?: string;
  correlationId?: string;
  userId: string;
  email: string;
  ipAddress?: string;
  userAgent?: string;
  deviceType?: string;
  location?: string;
  loginMethod?: string;
}

class NatsClient {
  private connection: NatsConnection | null = null;
  private jetStream: JetStreamClient | null = null;
  private codec = StringCodec();
  private isConnecting = false;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;

  async connect(): Promise<void> {
    if (this.connection || this.isConnecting) {
      return;
    }

    this.isConnecting = true;
    const natsUrl = process.env.NATS_URL || 'nats://nats.nats.svc.cluster.local:4222';

    try {
      logger.info({ url: natsUrl }, 'Connecting to NATS...');

      this.connection = await connect({
        servers: [natsUrl],
        maxReconnectAttempts: -1, // Unlimited reconnects
        reconnectTimeWait: 2000, // 2 seconds between reconnects
        timeout: 10000, // 10 second connection timeout
        name: 'auth-bff',
      });

      // Get JetStream client
      this.jetStream = this.connection.jetstream();

      logger.info('Connected to NATS successfully');
      this.reconnectAttempts = 0;

      // Handle connection events
      this.connection.closed().then((err) => {
        if (err) {
          logger.error({ error: err }, 'NATS connection closed with error');
        } else {
          logger.info('NATS connection closed');
        }
        this.connection = null;
        this.jetStream = null;
      });

    } catch (error) {
      logger.error({ error }, 'Failed to connect to NATS');
      this.connection = null;
      this.jetStream = null;
      this.reconnectAttempts++;

      // Retry connection with backoff
      if (this.reconnectAttempts < this.maxReconnectAttempts) {
        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
        logger.info({ delay, attempt: this.reconnectAttempts }, 'Retrying NATS connection...');
        setTimeout(() => this.connect(), delay);
      }
    } finally {
      this.isConnecting = false;
    }
  }

  async publishAuthEvent(event: AuthEvent): Promise<boolean> {
    if (!this.jetStream) {
      logger.warn('NATS not connected, attempting to connect...');
      await this.connect();

      if (!this.jetStream) {
        logger.error('Cannot publish event: NATS not connected');
        return false;
      }
    }

    try {
      const subject = event.eventType;
      const data = this.codec.encode(JSON.stringify(event));

      // Publish to JetStream
      const ack = await this.jetStream.publish(subject, data);

      logger.info({
        subject,
        seq: ack.seq,
        userId: event.userId
      }, 'Auth event published');

      return true;
    } catch (error) {
      logger.error({ error, eventType: event.eventType }, 'Failed to publish auth event');
      return false;
    }
  }

  async publishLoginSuccess(
    tenantId: string,
    userId: string,
    email: string,
    ipAddress?: string,
    userAgent?: string,
    loginMethod?: string
  ): Promise<boolean> {
    const event: AuthEvent = {
      eventType: AuthEventTypes.LOGIN_SUCCESS,
      tenantId,
      sourceId: `login-${userId}-${Date.now()}`,
      timestamp: new Date().toISOString(),
      userId,
      email,
      ipAddress,
      userAgent,
      loginMethod: loginMethod || 'oidc',
    };

    return this.publishAuthEvent(event);
  }

  async close(): Promise<void> {
    if (this.connection) {
      await this.connection.drain();
      this.connection = null;
      this.jetStream = null;
      logger.info('NATS connection closed');
    }
  }

  isConnected(): boolean {
    return this.connection !== null && !this.connection.isClosed();
  }
}

// Singleton instance
export const natsClient = new NatsClient();

// Initialize connection on module load (non-blocking)
natsClient.connect().catch((err) => {
  logger.error({ error: err }, 'Initial NATS connection failed');
});
