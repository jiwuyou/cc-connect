export const WEBNEW_BRIDGE_PLATFORM = 'webnew';
export const DEFAULT_WEB_BRIDGE_PLATFORM = 'web2';
export const WEB_BRIDGE_PLATFORM_OPTIONS = ['web2', 'web3', 'web4', 'web5'];
export const WEB_BRIDGE_USER_ID = 'web-admin';
export const WEB_BRIDGE_USER_NAME = 'Web Admin';

const STORAGE_KEY = 'cc_web2_bridge_platform';
const TRANSPORT_TAB_KEY = 'cc_web_bridge_transport_tab_id';

const pageInstanceId = randomTransportId();

function randomTransportId() {
  const cryptoObj = typeof crypto !== 'undefined' ? crypto : undefined;
  if (cryptoObj?.randomUUID) {
    return cryptoObj.randomUUID().replace(/[^a-zA-Z0-9]/g, '').slice(0, 10).toLowerCase();
  }
  return Math.random().toString(36).slice(2, 12);
}

export function normalizeWebBridgePlatform(value: string | null | undefined) {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'bridge' || normalized === 'web' || /^web[0-9][0-9a-z_-]*$/.test(normalized)) return normalized;
  return DEFAULT_WEB_BRIDGE_PLATFORM;
}

export function getWebBridgeTransportPlatform(route = DEFAULT_WEB_BRIDGE_PLATFORM) {
  const normalizedRoute = normalizeWebBridgePlatform(route);
  if (typeof window === 'undefined') return `${normalizedRoute}-tab-server`;

  let tabId = window.sessionStorage.getItem(TRANSPORT_TAB_KEY);
  if (!tabId) {
    tabId = randomTransportId();
    window.sessionStorage.setItem(TRANSPORT_TAB_KEY, tabId);
  }

  return `${normalizedRoute}-tab-${tabId}-${pageInstanceId}`;
}

export function getInitialWebBridgePlatform() {
  if (typeof window === 'undefined') return DEFAULT_WEB_BRIDGE_PLATFORM;
  const params = new URLSearchParams(window.location.search);
  const fromURL = params.get('web_platform') || params.get('platform');
  if (fromURL) return normalizeWebBridgePlatform(fromURL);
  return normalizeWebBridgePlatform(window.localStorage.getItem(STORAGE_KEY));
}

export function persistWebBridgePlatform(platform: string) {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(STORAGE_KEY, normalizeWebBridgePlatform(platform));
}

export function platformFromSessionKey(sessionKey: string) {
  const idx = sessionKey.indexOf(':');
  return idx > 0 ? sessionKey.slice(0, idx) : '';
}

export function isWebBridgeSessionKey(sessionKey: string) {
  return sessionKey.startsWith(`${WEBNEW_BRIDGE_PLATFORM}:`) || isLegacyWebBridgeSessionKey(sessionKey);
}

export function isLegacyWebBridgeSessionKey(sessionKey: string) {
  return /^(bridge|web|web[0-9][0-9a-z_-]*):web-admin:/.test(sessionKey);
}

export function webSessionKey(projectName: string) {
  return `${WEBNEW_BRIDGE_PLATFORM}:${WEB_BRIDGE_USER_ID}:${projectName}`;
}

export function webRouteSessionKey(projectName: string, platform = DEFAULT_WEB_BRIDGE_PLATFORM) {
  return `${normalizeWebBridgePlatform(platform)}:${WEB_BRIDGE_USER_ID}:${projectName}`;
}

export function legacyWebSessionKeys(projectName: string, preferredPlatform = DEFAULT_WEB_BRIDGE_PLATFORM) {
  const keys = [
    webRouteSessionKey(projectName, preferredPlatform),
    `bridge:${WEB_BRIDGE_USER_ID}:${projectName}`,
    `web:${WEB_BRIDGE_USER_ID}:${projectName}`,
    ...WEB_BRIDGE_PLATFORM_OPTIONS.map((platform) => webRouteSessionKey(projectName, platform)),
  ];
  return Array.from(new Set(keys));
}
