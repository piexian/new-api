/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

const DEFAULT_HEADER_NAV_MODULES = {
  home: true,
  console: true,
  pricing: { enabled: true, requireAuth: false },
  rankings: { enabled: true, requireAuth: false },
  docs: true,
  about: true,
};

const DEFAULT_SIDEBAR_MODULES = {
  chat: {
    enabled: true,
    playground: true,
    chat: true,
  },
  console: {
    enabled: true,
    detail: true,
    token: true,
    log: true,
    midjourney: true,
    task: true,
    email: true,
  },
  personal: {
    enabled: true,
    topup: true,
    invite: true,
    personal: true,
  },
  admin: {
    enabled: true,
    channel: true,
    models: true,
    deployment: true,
    redemption: true,
    user: true,
    ip_ban: true,
    subscription: true,
    risk_center: true,
    setting: true,
  },
};

const SIDEBAR_ROUTE_RULES = [
  { prefix: '/console/models', section: 'admin', module: 'models' },
  { prefix: '/console/deployment', section: 'admin', module: 'deployment' },
  { prefix: '/console/subscription', section: 'admin', module: 'subscription' },
  { prefix: '/console/channel', section: 'admin', module: 'channel' },
  { prefix: '/console/redemption', section: 'admin', module: 'redemption' },
  { prefix: '/console/user', section: 'admin', module: 'user' },
  { prefix: '/console/ip_ban', section: 'admin', module: 'ip_ban' },
  { prefix: '/console/setting', section: 'admin', module: 'setting' },
  { prefix: '/console/risk', section: 'admin', module: 'risk_center' },
  { prefix: '/console/playground', section: 'chat', module: 'playground' },
  { prefix: '/console/token', section: 'console', module: 'token' },
  { prefix: '/console/topup', section: 'personal', module: 'topup' },
  { prefix: '/console/invite', section: 'personal', module: 'invite' },
  { prefix: '/console/personal', section: 'personal', module: 'personal' },
  { prefix: '/console/log', section: 'console', module: 'log' },
  { prefix: '/console/midjourney', section: 'console', module: 'midjourney' },
  { prefix: '/console/task', section: 'console', module: 'task' },
  { prefix: '/console/email-log', section: 'console', module: 'email' },
  { prefix: '/console/chat', section: 'chat', module: 'chat' },
  { prefix: '/chat2link', section: 'chat', module: 'chat' },
  { prefix: '/console', section: 'console', module: 'detail' },
];

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function toBoolean(value, fallback) {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'number') {
    if (value === 1) return true;
    if (value === 0) return false;
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true' || normalized === '1') return true;
    if (normalized === 'false' || normalized === '0') return false;
  }
  return fallback;
}

function parseJsonObject(raw) {
  if (!raw || String(raw).trim() === '') return null;
  if (typeof raw === 'object') return raw;
  try {
    const parsed = JSON.parse(String(raw));
    return parsed && typeof parsed === 'object' ? parsed : null;
  } catch {
    return null;
  }
}

function parseAccessModule(raw, fallback) {
  if (
    typeof raw === 'boolean' ||
    typeof raw === 'number' ||
    typeof raw === 'string'
  ) {
    return {
      enabled: toBoolean(raw, fallback.enabled),
      requireAuth: fallback.requireAuth,
    };
  }
  if (raw && typeof raw === 'object') {
    return {
      enabled: toBoolean(raw.enabled, fallback.enabled),
      requireAuth: toBoolean(raw.requireAuth, fallback.requireAuth),
    };
  }
  return { ...fallback };
}

export function parseHeaderNavModules(raw) {
  const result = clone(DEFAULT_HEADER_NAV_MODULES);
  const parsed = parseJsonObject(raw);
  if (!parsed) return result;

  Object.entries(parsed).forEach(([key, value]) => {
    if (key === 'pricing' || key === 'rankings') {
      result[key] = parseAccessModule(value, result[key]);
      return;
    }
    result[key] = toBoolean(value, result[key] ?? true);
  });

  return result;
}

function parseSidebarModules(raw) {
  const result = clone(DEFAULT_SIDEBAR_MODULES);
  const parsed = parseJsonObject(raw);
  if (!parsed) return result;

  Object.entries(parsed).forEach(([sectionKey, sectionValue]) => {
    if (!sectionValue || typeof sectionValue !== 'object') return;
    const base = result[sectionKey] ?? { enabled: true };
    result[sectionKey] = { ...base };
    Object.entries(sectionValue).forEach(([moduleKey, moduleValue]) => {
      result[sectionKey][moduleKey] = toBoolean(
        moduleValue,
        base[moduleKey] ?? true,
      );
    });
  });

  return result;
}

function normalizePathname(pathname) {
  if (!pathname || pathname === '/') return '/';
  return pathname.replace(/\/+$/, '');
}

function matchesPrefix(pathname, prefix) {
  const normalized = normalizePathname(pathname);
  const normalizedPrefix = normalizePathname(prefix);
  return (
    normalized === normalizedPrefix ||
    normalized.startsWith(`${normalizedPrefix}/`)
  );
}

export function isHeaderRouteEnabled(status, pathname) {
  const modules = parseHeaderNavModules(status?.HeaderNavModules);
  const path = normalizePathname(pathname);
  if (path === '/') return modules.home !== false;
  if (
    matchesPrefix(path, '/user-agreement') ||
    matchesPrefix(path, '/privacy-policy')
  ) {
    return modules.docs !== false;
  }
  if (matchesPrefix(path, '/pricing'))
    return modules.pricing?.enabled !== false;
  if (matchesPrefix(path, '/rankings')) {
    return modules.rankings?.enabled !== false;
  }
  if (matchesPrefix(path, '/about')) return modules.about !== false;
  if (matchesPrefix(path, '/console')) return modules.console !== false;
  return true;
}

export function isSidebarRouteEnabled(status, pathname) {
  const rule = SIDEBAR_ROUTE_RULES.find(({ prefix }) =>
    matchesPrefix(pathname, prefix),
  );
  if (!rule) return true;

  const config = parseSidebarModules(status?.SidebarModulesAdmin);
  const section = config[rule.section];
  if (!section) return true;
  if (section.enabled === false) return false;
  return section[rule.module] !== false;
}

export function isRouteModuleEnabled(status, pathname) {
  return (
    isHeaderRouteEnabled(status, pathname) &&
    isSidebarRouteEnabled(status, pathname)
  );
}
