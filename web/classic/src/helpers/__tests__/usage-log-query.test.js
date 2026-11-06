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

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { buildUsageLogQuery } from '../usageLogQuery';

const queryOf = (url) => new URLSearchParams(url.slice(url.indexOf('?') + 1));

const baseArgs = {
  page: 2,
  pageSize: 20,
  logType: 0,
  startTimestamp: 1700000000,
  endTimestamp: 1700003600,
};

test('login scope keeps reserved characters in account and request id', () => {
  const account = 'alice+tag&co#1@example.com';
  const requestId = 'req&a=b#frag';

  const adminUrl = buildUsageLogQuery({
    ...baseArgs,
    isAdmin: true,
    scope: 'login',
    username: account,
    requestId,
  });
  const adminQuery = queryOf(adminUrl);
  assert.equal(adminQuery.get('username'), account);
  assert.equal(adminQuery.get('request_id'), requestId);
  assert.equal(adminQuery.get('scope'), 'login');
  assert.ok(adminUrl.includes('%2B'));
  assert.ok(adminUrl.includes('%26'));
  assert.ok(adminUrl.includes('%23'));
  assert.ok(!adminUrl.includes('#'));

  const selfUrl = buildUsageLogQuery({
    ...baseArgs,
    isAdmin: false,
    scope: 'login',
    requestId,
  });
  const selfQuery = queryOf(selfUrl);
  assert.ok(selfUrl.startsWith('/api/log/self/?'));
  assert.equal(selfQuery.get('request_id'), requestId);
  assert.equal(selfQuery.get('scope'), 'login');
  assert.equal(selfQuery.has('username'), false);
});

test('login scope drops consume-only filters', () => {
  const url = buildUsageLogQuery({
    ...baseArgs,
    isAdmin: true,
    scope: 'login',
    tokenName: 'tok',
    modelName: 'gpt-4o',
    group: 'vip',
    channel: '3',
    sessionId: 'sess',
    cacheKey: 'cache',
  });
  const query = queryOf(url);
  for (const key of [
    'token_name',
    'model_name',
    'group',
    'channel',
    'session_id',
    'cache_key',
  ]) {
    assert.equal(query.get(key), '');
  }
});

test('default scope keeps the plain log semantics', () => {
  const url = buildUsageLogQuery({
    ...baseArgs,
    isAdmin: true,
    logType: 2,
    tokenName: 'tok',
    modelName: 'gpt-4o',
    group: 'vip',
    channel: '3',
    ip: ' 10.0.0.1 ',
    userAgent: 'Mozilla/5.0',
    sessionId: ' sess ',
    cacheKey: ' cache ',
    username: 'alice+tag@example.com',
  });
  const query = queryOf(url);
  assert.ok(url.startsWith('/api/log/?'));
  assert.equal(query.get('p'), '2');
  assert.equal(query.get('page_size'), '20');
  assert.equal(query.get('type'), '2');
  assert.equal(query.get('start_timestamp'), '1700000000');
  assert.equal(query.get('end_timestamp'), '1700003600');
  assert.equal(query.get('token_name'), 'tok');
  assert.equal(query.get('model_name'), 'gpt-4o');
  assert.equal(query.get('group'), 'vip');
  assert.equal(query.get('channel'), '3');
  assert.equal(query.get('ip'), '10.0.0.1');
  assert.equal(query.get('user_agent'), '%Mozilla/5.0%');
  assert.equal(query.get('session_id'), 'sess');
  assert.equal(query.get('cache_key'), 'cache');
  assert.equal(query.get('username'), 'alice+tag@example.com');
  assert.equal(query.has('scope'), false);
});
