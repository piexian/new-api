export const FRONTEND_THEME_COOKIE = 'new-api-frontend';
export const FRONTEND_THEME_COOKIE_MAX_AGE = 60 * 60 * 24 * 365;
export const FRONTEND_RETURN_TIP_PENDING = 'new-api-default-frontend-return-tip-pending';

const defaultFrontendRoutes = [
  { from: '/console/personal', to: '/profile' },
  { from: '/console/topup', to: '/wallet' },
  { from: '/console/token', to: '/keys' },
  { from: '/console/channel', to: '/channels' },
  { from: '/console/models', to: '/models' },
  { from: '/console/playground', to: '/playground' },
  { from: '/console/log', to: '/usage-logs' },
  { from: '/console/user', to: '/users' },
  { from: '/console/redemption', to: '/redemption-codes' },
  { from: '/console/subscription', to: '/subscriptions' },
  { from: '/console/setting', to: '/system-settings/general' },
  { from: '/console/chat/', to: '/chat/', preserveSuffix: true },
  { from: '/console', to: '/dashboard' },
  { from: '/login', to: '/sign-in' },
  { from: '/register', to: '/sign-up' },
  { from: '/reset', to: '/forgot-password' },
];

function getSwitchTargetPath(theme, pathname) {
  if (theme !== 'default') {
    return '/console';
  }
  const match = defaultFrontendRoutes.find(({ from, preserveSuffix }) =>
    preserveSuffix ? pathname.startsWith(from) : pathname === from,
  );
  if (!match) {
    if (pathname.startsWith('/console')) {
      return '/dashboard';
    }
    return pathname;
  }
  return match.preserveSuffix ? `${match.to}${pathname.slice(match.from.length)}` : match.to;
}

export function switchFrontendTheme(theme, confirmMessage) {
  if (theme !== 'default' && theme !== 'classic') {
    return;
  }
  if (confirmMessage && !window.confirm(confirmMessage)) {
    return;
  }
  document.cookie = `${FRONTEND_THEME_COOKIE}=${theme}; path=/; max-age=${FRONTEND_THEME_COOKIE_MAX_AGE}`;
  if (theme === 'default') {
    try {
      window.localStorage.setItem(FRONTEND_RETURN_TIP_PENDING, '1');
    } catch {
      // Ignore storage errors; switching frontend should still work.
    }
  }
  window.location.assign(getSwitchTargetPath(theme, window.location.pathname));
}
