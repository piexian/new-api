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

export function buildNodeLocAuthorizationURL(status, origin, state) {
  let callback;
  try {
    callback = new URL(status.nodeloc_redirect_uri);
  } catch {
    throw new Error('NodeLoc 登录配置不完整，请联系管理员');
  }
  if (
    !status.nodeloc_client_id ||
    !['http:', 'https:'].includes(callback.protocol) ||
    callback.pathname !== '/oauth/nodeloc' ||
    callback.search ||
    callback.hash ||
    callback.username ||
    callback.password
  ) {
    throw new Error('NodeLoc 登录配置不完整，请联系管理员');
  }
  if (callback.origin !== origin) {
    const error = new Error('请在配置的正式站点使用 NodeLoc 登录：{{origin}}');
    error.origin = callback.origin;
    throw error;
  }
  const url = new URL('https://www.nodeloc.com/oauth-provider/authorize');
  url.searchParams.set('client_id', status.nodeloc_client_id);
  url.searchParams.set('redirect_uri', status.nodeloc_redirect_uri);
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('scope', 'openid profile');
  url.searchParams.set('state', state);
  return url;
}
