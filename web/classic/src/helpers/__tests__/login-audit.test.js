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
import {
  buildLoginAuditDetailRows,
  formatLoginAuditTime,
  getLoginAuditAttemptCount,
  getLoginAuditOutcome,
  getLoginAuditOutcomeLabel,
  getLoginAuditReasonLabel,
  getLoginAuditStageLabel,
  getLoginAuditTargetAccount,
  getLoginAuditUserAgent,
  getLoginAuditWindowText,
} from '../loginAudit';

const t = (key) => key;

const loginRecord = (overrides = {}) => ({
  type: 7,
  username: 'alice',
  ip: '10.0.0.1',
  request_id: 'req-1',
  user_agent: '',
  other: '',
  ...overrides,
});

const eventRecord = (other) =>
  loginRecord({ type: 8, other: JSON.stringify(other) });

test('successful login keeps success outcome without auth_audit', () => {
  assert.equal(getLoginAuditOutcome(loginRecord()), 'success');
  assert.equal(getLoginAuditOutcomeLabel('success', t), '成功');
});

test('security events never render an invalid success outcome', () => {
  const record = eventRecord({
    auth_audit: { outcome: 'success', stage: 'credentials' },
  });
  assert.equal(getLoginAuditOutcome(record), 'unknown');

  const blocked = eventRecord({
    auth_audit: { outcome: 'blocked', stage: 'precheck' },
  });
  assert.equal(getLoginAuditOutcome(blocked), 'blocked');
  assert.equal(getLoginAuditOutcomeLabel('blocked', t), '已拦截');
});

test('known stage and reason map to human labels, unknown values fall back', () => {
  assert.equal(getLoginAuditStageLabel('two_factor', t), '两步验证');
  assert.equal(
    getLoginAuditReasonLabel('invalid_credentials', t),
    '凭证不正确',
  );
  assert.equal(getLoginAuditStageLabel('nope', t), '未知阶段');
  assert.equal(getLoginAuditReasonLabel('nope', t), '未知原因');
  assert.equal(getLoginAuditReasonLabel(undefined, t), '未知原因');
  assert.equal(getLoginAuditOutcomeLabel('nope', t), '未知结果');
});

test('attempt count defaults to one and accepts numeric strings', () => {
  assert.equal(getLoginAuditAttemptCount(null), 1);
  assert.equal(getLoginAuditAttemptCount({ count: 0 }), 1);
  assert.equal(getLoginAuditAttemptCount({ count: 7 }), 7);
  assert.equal(getLoginAuditAttemptCount({ count: '3' }), 3);
});

test('user agent prefers the top-level field and falls back to other', () => {
  assert.equal(
    getLoginAuditUserAgent(loginRecord({ user_agent: 'top-agent' }), {
      user_agent: 'other-agent',
    }),
    'top-agent',
  );
  assert.equal(
    getLoginAuditUserAgent(loginRecord(), { user_agent: 'other-agent' }),
    'other-agent',
  );
  assert.equal(getLoginAuditUserAgent(loginRecord(), null), '');
});

test('window uses unix seconds', () => {
  const expected = new Date(1700000000 * 1000).toLocaleString();
  assert.equal(formatLoginAuditTime(1700000000), expected);
  assert.equal(formatLoginAuditTime(''), '');
  assert.equal(formatLoginAuditTime(0), '');
  assert.equal(getLoginAuditWindowText(null), '');
  assert.equal(
    getLoginAuditWindowText({ window_start: 1700000000, window_end: '' }),
    expected,
  );
  assert.equal(
    getLoginAuditWindowText({
      window_start: 1700000000,
      window_end: 1700000600,
    }),
    `${expected} - ${new Date(1700000600 * 1000).toLocaleString()}`,
  );
});

test('target account comes only from admin info', () => {
  assert.equal(
    getLoginAuditTargetAccount({
      admin_info: { target_username: 'bob', target_user_id: 12 },
    }),
    'bob (ID: 12)',
  );
  assert.equal(
    getLoginAuditTargetAccount({ admin_info: { target_user_id: 12 } }),
    'ID: 12',
  );
  assert.equal(getLoginAuditTargetAccount({}), '');
  assert.equal(getLoginAuditTargetAccount(null), '');
});

test('non-admin detail rows hide admin-only diagnostics', () => {
  const record = eventRecord({
    login_method: 'password',
    request_path: '/api/user/login',
    user_agent: 'Mozilla/5.0 (blocked attempt)',
    auth_audit: {
      outcome: 'blocked',
      stage: 'precheck',
      reason: 'ip_banned',
      count: 4,
      window_start: 1700000000,
      window_end: 1700000600,
    },
    admin_info: {
      target_username: 'bob',
      target_user_id: 12,
      attempted_identifier: 'bob',
      cf_ray: 'ray-1',
      peer_ip: '10.0.0.9',
    },
  });
  const rows = buildLoginAuditDetailRows({
    record,
    other: JSON.parse(record.other),
    t,
    isAdminUser: false,
  });
  const labels = rows.map((row) => row.label);
  assert.deepEqual(labels, [
    '结果',
    '阶段',
    '原因',
    '尝试次数',
    '统计窗口',
    '登录方式',
    '请求路径',
    '账户',
    'IP 地址',
    'User Agent',
    'Request ID',
  ]);
  assert.equal(rows[0].value, '已拦截');
  assert.equal(rows[3].value, '4');
  assert.equal(
    rows.find((row) => row.label === '请求路径')?.value,
    '/api/user/login',
  );
  assert.equal(
    rows.find((row) => row.label === 'User Agent')?.value,
    'Mozilla/5.0 (blocked attempt)',
  );
  assert.equal(rows.find((row) => row.label === 'IP 地址')?.monospace, true);
  assert.equal(rows.find((row) => row.label === 'Request ID')?.value, 'req-1');
  assert.equal(
    rows.some((row) => row.label === '目标账户'),
    false,
  );
  assert.equal(
    rows.some((row) => row.label === '输入账号'),
    false,
  );
  assert.equal(
    rows.some((row) => row.label === 'CF-Ray'),
    false,
  );
  assert.equal(
    rows.some((row) => row.label === '对端 IP'),
    false,
  );
});

test('admin detail rows include request diagnostics from the whitelist', () => {
  const record = eventRecord({
    login_method: 'password',
    request_path: '/api/user/login',
    user_agent: 'Mozilla/5.0 (failed attempt)',
    auth_audit: {
      outcome: 'failure',
      stage: 'credentials',
      reason: 'invalid_credentials',
    },
    admin_info: {
      target_username: 'bob',
      target_user_id: 12,
      attempted_identifier: 'bob',
      cf_ray: 'ray-1',
      peer_ip: '10.0.0.9',
    },
  });
  const rows = buildLoginAuditDetailRows({
    record,
    other: JSON.parse(record.other),
    t,
    isAdminUser: true,
  });
  const labels = rows.map((row) => row.label);
  assert.deepEqual(labels, [
    '结果',
    '阶段',
    '原因',
    '尝试次数',
    '登录方式',
    '请求路径',
    '账户',
    '目标账户',
    '输入账号',
    'IP 地址',
    'User Agent',
    'Request ID',
    'CF-Ray',
    '对端 IP',
  ]);
  assert.equal(
    rows.find((row) => row.label === '目标账户')?.value,
    'bob (ID: 12)',
  );
  assert.equal(
    rows.find((row) => row.label === 'User Agent')?.value,
    'Mozilla/5.0 (failed attempt)',
  );
  assert.equal(rows.find((row) => row.label === 'CF-Ray')?.value, 'ray-1');
  assert.equal(rows.find((row) => row.label === '对端 IP')?.value, '10.0.0.9');
});

test('successful login detail rows expose ip and user agent', () => {
  const record = loginRecord({
    user_agent: 'Mozilla/5.0 top-level',
    other: JSON.stringify({ user_agent: 'Mozilla/5.0 other' }),
  });
  const rows = buildLoginAuditDetailRows({
    record,
    other: JSON.parse(record.other),
    t,
    isAdminUser: false,
  });
  assert.equal(rows[0].value, '成功');
  assert.equal(rows.find((row) => row.label === 'IP 地址')?.value, '10.0.0.1');
  assert.equal(
    rows.find((row) => row.label === 'User Agent')?.value,
    'Mozilla/5.0 top-level',
  );
});
