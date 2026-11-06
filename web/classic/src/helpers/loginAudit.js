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

import { getLogOther } from './log';

// 登录成功日志（type=7）始终视为成功；安全事件（type=8）不允许呈现成功。
export const LOGIN_LOG_TYPE = 7;
export const LOGIN_SECURITY_EVENT_LOG_TYPE = 8;
export const LOGIN_AUDIT_UNKNOWN_OUTCOME = 'unknown';

const LOGIN_AUDIT_OUTCOME_LABELS = {
  success: '成功',
  failure: '失败',
  blocked: '已拦截',
  challenge: '需验证',
  error: '异常',
};

const LOGIN_AUDIT_OUTCOME_COLORS = {
  success: 'green',
  failure: 'red',
  blocked: 'orange',
  challenge: 'amber',
  error: 'grey',
};

const LOGIN_AUDIT_STAGE_LABELS = {
  precheck: '预检查',
  request: '请求校验',
  captcha: '验证码',
  credentials: '凭证校验',
  two_factor: '两步验证',
  session: '会话创建',
};

const LOGIN_AUDIT_REASON_LABELS = {
  invalid_request: '请求参数无效',
  password_encryption_required: '需要密码加密',
  password_decryption_failed: '密码解密失败',
  user_not_found: '用户不存在',
  invalid_credentials: '凭证不正确',
  account_disabled: '账户已禁用',
  password_login_disabled: '密码登录已关闭',
  email_repair_required: '需要修复邮箱',
  two_factor_required: '需要两步验证',
  two_factor_session_expired: '两步验证会话已过期',
  two_factor_invalid: '两步验证码不正确',
  two_factor_locked: '两步验证已锁定',
  two_factor_unavailable: '无法使用两步验证',
  user_agent_missing: '浏览器环境异常',
  session_save_failed: '会话保存失败',
  database_error: '数据库错误',
  ip_banned: 'IP 已被封禁',
  rate_limited: '请求过于频繁',
  captcha_missing: '缺少验证码',
  captcha_invalid: '验证码不正确',
  captcha_error: '验证码校验失败',
  internal_error: '内部错误',
};

const SECURITY_EVENT_OUTCOMES = ['failure', 'blocked', 'challenge', 'error'];

function toTrimmedString(value) {
  if (value === undefined || value === null) {
    return '';
  }
  if (typeof value === 'string') {
    return value.trim();
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return '';
}

function hasValue(value) {
  return value !== undefined && value !== null && value !== '';
}

export function isLoginAuditLogType(type) {
  return (
    Number(type) === LOGIN_LOG_TYPE ||
    Number(type) === LOGIN_SECURITY_EVENT_LOG_TYPE
  );
}

// 读取 other.auth_audit，缺失或结构异常时返回 null。
export function getLoginAudit(other) {
  if (other === undefined || other === null || typeof other !== 'object') {
    return null;
  }
  const audit = other.auth_audit;
  if (!audit || typeof audit !== 'object' || Array.isArray(audit)) {
    return null;
  }
  return audit;
}

export function getLoginAuditOutcome(record) {
  const type = Number(record?.type);
  if (type === LOGIN_LOG_TYPE) {
    return 'success';
  }
  if (type !== LOGIN_SECURITY_EVENT_LOG_TYPE) {
    return LOGIN_AUDIT_UNKNOWN_OUTCOME;
  }
  const audit = getLoginAudit(getLogOther(record?.other));
  const outcome = toTrimmedString(audit?.outcome);
  return SECURITY_EVENT_OUTCOMES.includes(outcome)
    ? outcome
    : LOGIN_AUDIT_UNKNOWN_OUTCOME;
}

export function getLoginAuditOutcomeLabel(outcome, t) {
  const label = LOGIN_AUDIT_OUTCOME_LABELS[outcome];
  return t(label || '未知结果');
}

export function getLoginAuditOutcomeColor(outcome) {
  return LOGIN_AUDIT_OUTCOME_COLORS[outcome] || 'grey';
}

export function getLoginAuditStageLabel(stage, t) {
  const label = LOGIN_AUDIT_STAGE_LABELS[toTrimmedString(stage)];
  return t(label || '未知阶段');
}

export function getLoginAuditReasonLabel(reason, t) {
  const label = LOGIN_AUDIT_REASON_LABELS[toTrimmedString(reason)];
  return t(label || '未知原因');
}

export function getLoginAuditAttemptCount(audit) {
  const count = Number(audit?.count);
  if (!Number.isFinite(count) || count < 1) {
    return 1;
  }
  return Math.floor(count);
}

// 统计窗口为 Unix 秒。
export function formatLoginAuditTime(seconds) {
  const parsed = Number(seconds);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return '';
  }
  return new Date(parsed * 1000).toLocaleString();
}

export function getLoginAuditWindowText(audit) {
  const start = formatLoginAuditTime(audit?.window_start);
  const end = formatLoginAuditTime(audit?.window_end);
  if (start && end) {
    return `${start} - ${end}`;
  }
  return start || end || '';
}

export function getLoginAuditUserAgent(record, other) {
  const topLevel = toTrimmedString(record?.user_agent);
  if (topLevel) {
    return topLevel;
  }
  return toTrimmedString(other?.user_agent);
}

function formatAccountWithId(name, id) {
  if (name && hasValue(id)) {
    return `${name} (ID: ${id})`;
  }
  if (name) {
    return name;
  }
  if (hasValue(id)) {
    return `ID: ${id}`;
  }
  return '';
}

// 目标账户仅来自 admin_info，普通用户响应中不存在该字段。
export function getLoginAuditTargetAccount(other) {
  const adminInfo = other?.admin_info;
  if (!adminInfo || typeof adminInfo !== 'object') {
    return '';
  }
  return formatAccountWithId(
    toTrimmedString(adminInfo.target_username),
    adminInfo.target_user_id,
  );
}

export function getLoginAuditAttemptedIdentifier(other) {
  const adminInfo = other?.admin_info;
  if (!adminInfo || typeof adminInfo !== 'object') {
    return '';
  }
  return toTrimmedString(adminInfo.attempted_identifier);
}

// 详情白名单：只输出结构化字段，不回显原始 JSON。
export function buildLoginAuditDetailRows({ record, other, t, isAdminUser }) {
  const audit = getLoginAudit(other);
  const rows = [];
  const push = (label, value, options) => {
    const text = toTrimmedString(value);
    if (!text) {
      return;
    }
    rows.push({ label, value: text, ...options });
  };

  push(t('结果'), getLoginAuditOutcomeLabel(getLoginAuditOutcome(record), t));
  if (audit?.stage) {
    push(t('阶段'), getLoginAuditStageLabel(audit.stage, t));
  }
  if (audit?.reason) {
    push(t('原因'), getLoginAuditReasonLabel(audit.reason, t));
  }
  const attemptCount = getLoginAuditAttemptCount(audit);
  if (
    Number(record?.type) === LOGIN_SECURITY_EVENT_LOG_TYPE ||
    attemptCount > 1
  ) {
    push(t('尝试次数'), String(attemptCount));
  }
  const windowText = getLoginAuditWindowText(audit);
  if (windowText) {
    push(t('统计窗口'), windowText);
  }
  push(t('登录方式'), other?.login_method);
  push(t('请求路径'), other?.request_path, { monospace: true });
  push(t('账户'), record?.username);
  if (isAdminUser) {
    push(t('目标账户'), getLoginAuditTargetAccount(other));
    push(t('输入账号'), getLoginAuditAttemptedIdentifier(other));
  }
  push(t('IP 地址'), record?.ip, { monospace: true });
  push(t('User Agent'), getLoginAuditUserAgent(record, other), {
    monospace: true,
    scrollable: true,
  });
  push(t('Request ID'), record?.request_id, { monospace: true });
  if (isAdminUser) {
    push(t('CF-Ray'), other?.admin_info?.cf_ray, { monospace: true });
    push(t('对端 IP'), other?.admin_info?.peer_ip, { monospace: true });
  }
  return rows;
}
