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

// 用 URLSearchParams 编码，邮箱等含 + & # 的账号不会被当作 query 保留字符。
export function buildUsageLogQuery({
  isAdmin,
  page,
  pageSize,
  logType,
  scope = 'all',
  startTimestamp,
  endTimestamp,
  username = '',
  tokenName = '',
  modelName = '',
  group = '',
  channel = '',
  requestId = '',
  ip = '',
  userAgent = '',
  sessionId = '',
  cacheKey = '',
}) {
  const isLoginScope = scope === 'login';
  const params = new URLSearchParams();
  params.set('p', String(page));
  params.set('page_size', String(pageSize));
  params.set('type', String(logType));
  params.set('start_timestamp', String(startTimestamp));
  params.set('end_timestamp', String(endTimestamp));
  params.set('request_id', requestId);
  params.set('ip', ip ? ip.trim() : '');
  params.set('user_agent', userAgent ? `%${userAgent.trim()}%` : '');

  if (isAdmin) {
    params.set('username', username);
    params.set('channel', isLoginScope ? '' : channel);
  }

  // 登录审计视图隐藏的消费类筛选项不参与查询
  params.set('token_name', isLoginScope ? '' : tokenName);
  params.set('model_name', isLoginScope ? '' : modelName);
  params.set('group', isLoginScope ? '' : group);
  params.set('session_id', isLoginScope || !sessionId ? '' : sessionId.trim());
  params.set('cache_key', isLoginScope || !cacheKey ? '' : cacheKey.trim());

  if (isLoginScope) {
    params.set('scope', 'login');
  }

  const path = isAdmin ? '/api/log/' : '/api/log/self/';
  return `${path}?${params.toString()}`;
}
