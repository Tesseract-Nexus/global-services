import pino from 'pino';
import { config } from './config';

const baseLogger = pino({
  level: config.server.logLevel,
  transport:
    config.server.nodeEnv === 'development'
      ? {
          target: 'pino-pretty',
          options: {
            colorize: true,
            translateTime: 'SYS:standard',
            ignore: 'pid,hostname',
          },
        }
      : undefined,
  base: {
    service: 'auth-bff',
    env: config.server.nodeEnv,
  },
  redact: {
    paths: [
      'req.headers.authorization',
      'req.headers.cookie',
      'res.headers["set-cookie"]',
      '*.password',
      '*.secret',
      '*.token',
      '*.refreshToken',
      '*.accessToken',
      '*.idToken',
    ],
    censor: '[REDACTED]',
  },
});

export const createLogger = (name: string) => baseLogger.child({ module: name });

export const logger = baseLogger;
