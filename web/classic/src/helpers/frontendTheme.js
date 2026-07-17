import { Modal } from '@douyinfe/semi-ui';

import { API } from './api';
import { showError, showSuccess } from './utils';

export const FRONTEND_THEME_COOKIE = 'new-api-frontend';
export const FRONTEND_THEME_COOKIE_MAX_AGE = 60 * 60 * 24 * 365;
export const FRONTEND_RETURN_TIP_PENDING =
  'new-api-default-frontend-return-tip-pending';
// localStorage 镜像键：与新版前端共享（同源），用于 Cookie 丢失时恢复偏好
export const FRONTEND_THEME_PREFERENCE_KEY = 'new-api-frontend-preference';
// 跳转死循环保护：Cookie 被完全禁用时，每次会话只尝试一次恢复跳转
const FRONTEND_THEME_RESTORE_ATTEMPTED_KEY =
  'new-api-frontend-restore-attempted';

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
  try {
    window.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, theme);
    // 显式切换后允许本会话内再次自动恢复
    window.sessionStorage.removeItem(FRONTEND_THEME_RESTORE_ATTEMPTED_KEY);
  } catch {
    // Ignore storage errors; switching frontend should still work.
  }
  if (theme === 'default') {
    try {
      window.localStorage.setItem(FRONTEND_RETURN_TIP_PENDING, '1');
    } catch {
      // Ignore storage errors; switching frontend should still work.
    }
  }
  window.location.assign(getSwitchTargetPath(theme, window.location.pathname));
}

/**
 * 启动时恢复前端主题偏好。
 *
 * Cookie 是服务端选择首屏 HTML 的唯一依据，但部分浏览器会在退出时清除
 * Cookie（登录态存在 localStorage 中不受影响），导致偏好丢失、回退到系统
 * 默认主题。这里用 localStorage 镜像兜底：Cookie 缺失而镜像存在时重写
 * Cookie；当前运行的经典前端与镜像不一致时，一次性跳转到新版前端。
 */
export function restoreFrontendThemePreference() {
  try {
    const stored = window.localStorage.getItem(FRONTEND_THEME_PREFERENCE_KEY);
    if (stored !== 'default' && stored !== 'classic') {
      return;
    }
    const parts = `; ${document.cookie}`.split(`; ${FRONTEND_THEME_COOKIE}=`);
    const cookie =
      parts.length === 2 ? parts.pop().split(';').shift() : undefined;
    if (cookie === stored) {
      return;
    }
    if (cookie === 'default' || cookie === 'classic') {
      // Cookie 仍然有效但与镜像不一致：以 Cookie 为准并同步镜像
      window.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, cookie);
      return;
    }
    // Cookie 已丢失：按镜像重写
    document.cookie = `${FRONTEND_THEME_COOKIE}=${stored}; path=/; max-age=${FRONTEND_THEME_COOKIE_MAX_AGE}`;
    if (stored !== 'default') {
      return;
    }
    if (window.sessionStorage.getItem(FRONTEND_THEME_RESTORE_ATTEMPTED_KEY)) {
      return;
    }
    window.sessionStorage.setItem(FRONTEND_THEME_RESTORE_ATTEMPTED_KEY, '1');
    window.location.replace(
      getSwitchTargetPath('default', window.location.pathname),
    );
  } catch {
    // Ignore storage errors; the current frontend keeps working.
  }
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
