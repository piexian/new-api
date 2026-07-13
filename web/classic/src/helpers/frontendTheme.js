import { Modal } from '@douyinfe/semi-ui';

import { API } from './api';
import { showError, showSuccess } from './utils';

export const FRONTEND_THEME_COOKIE = 'new-api-frontend';
export const FRONTEND_THEME_COOKIE_MAX_AGE = 60 * 60 * 24 * 365;
export const FRONTEND_RETURN_TIP_PENDING =
  'new-api-default-frontend-return-tip-pending';

const defaultFrontendRoutes = [
  { from: '/console/personal', to: '/profile' },
  { from: '/console/topup', to: '/wallet' },
  { from: '/console/invite', to: '/invite-rewards' },
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
  return match.preserveSuffix
    ? `${match.to}${pathname.slice(match.from.length)}`
    : match.to;
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

export function confirmSwitchToDefaultFrontend(t, { onLoadingChange } = {}) {
  Modal.confirm({
    title: t('切换到新版前端'),
    content: t('切换后页面会自动刷新，并进入新版前端。是否继续？'),
    okText: t('确认切换'),
    cancelText: t('取消'),
    onOk: async () => {
      onLoadingChange?.(true);
      try {
        const res = await API.put('/api/option/', {
          key: 'theme.frontend',
          value: 'default',
        });
        const { success, message } = res.data;
        if (!success) {
          showError(message);
          return;
        }
        showSuccess(t('已切换到新版前端，正在刷新页面'));
        setTimeout(() => switchFrontendTheme('default'), 600);
      } catch (error) {
        console.error('切换新版前端失败', error);
        showError(t('切换失败，请稍后重试'));
      } finally {
        onLoadingChange?.(false);
      }
    },
  });
}
