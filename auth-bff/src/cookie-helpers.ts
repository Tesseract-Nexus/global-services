import { FastifyReply } from 'fastify';
import { config, getSessionCookieName } from './config';
import { createLogger } from './logger';

const logger = createLogger('cookie-helpers');

/**
 * Determine the cookie domain based on the request host.
 * For tesserix.app domains, use .tesserix.app to enable cross-subdomain cookies.
 * For custom domains (e.g., admin.yahvismartfarm.com), don't set domain to let
 * the browser default to the request host.
 */
export const getCookieDomain = (forwardedHost: string | undefined): string | undefined => {
  // If a static domain is configured, use it
  if (config.session.cookieDomain) {
    return config.session.cookieDomain;
  }

  // Dynamic domain determination based on X-Forwarded-Host
  if (!forwardedHost) {
    return undefined;
  }

  // Remove port if present
  const hostname = forwardedHost.split(':')[0].toLowerCase();

  // For platform domains, use .{baseDomain} for cross-subdomain cookies
  if (hostname.endsWith(`.${config.baseDomain}`) || hostname === config.baseDomain) {
    return `.${config.baseDomain}`;
  }

  // For localhost, use undefined to let browser default
  if (hostname === 'localhost' || hostname.endsWith('.localhost')) {
    return undefined;
  }

  // For custom domains, don't set domain - let browser default to request host
  // This is critical for custom domain authentication to work
  logger.debug({ hostname }, 'Custom domain detected, not setting cookie domain');
  return undefined;
};

export const setSessionCookie = (
  reply: FastifyReply,
  sessionId: string,
  forwardedHost: string | undefined,
  rememberMe: boolean = false
) => {
  const REMEMBER_ME_MAX_AGE = 30 * 86400; // 30 days
  const maxAge = rememberMe ? REMEMBER_ME_MAX_AGE : config.session.maxAge;
  const domain = getCookieDomain(forwardedHost);
  const cookieName = getSessionCookieName(forwardedHost);

  logger.debug({ domain, forwardedHost, cookieName }, 'Setting session cookie');

  reply.setCookie(cookieName, sessionId, {
    httpOnly: true,
    secure: config.server.nodeEnv === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge,
    ...(domain ? { domain } : {}), // Only set domain if defined
  });
};
