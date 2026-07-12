/**
 * Logical app routes for the classic frontend.
 * Keep CTA / floating-ball links free of default-only paths (e.g. never hardcode `/dashboard`).
 */

export const CLASSIC_ROUTES = {
  home: '/',
  dashboard: '/console',
  pricing: '/pricing',
  rankings: '/rankings',
  sign_in: '/login',
  sign_up: '/register',
  keys: '/console/token',
  wallet: '/console/topup',
  docs: '', // external docs come from status.docs_link
};

/**
 * @param {keyof typeof CLASSIC_ROUTES} key
 * @param {{ docsLink?: string | null }} [options]
 * @returns {string}
 */
export function resolveAppRoute(key, options = {}) {
  if (key === 'docs') {
    const external = (options.docsLink || '').trim();
    if (external) return external;
    // classic header only shows docs when docs_link exists; keep empty when absent
    return '';
  }
  return CLASSIC_ROUTES[key] || '/';
}

export function getClassicAppRoutes() {
  return { ...CLASSIC_ROUTES };
}
