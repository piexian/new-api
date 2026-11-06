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

import React from 'react';
import { Space, Tag, Tooltip, Typography } from '@douyinfe/semi-ui';
import { getLogOther } from '../../../helpers';
import {
  getLoginAudit,
  getLoginAuditAttemptCount,
  getLoginAuditOutcome,
  getLoginAuditOutcomeColor,
  getLoginAuditOutcomeLabel,
  getLoginAuditReasonLabel,
  getLoginAuditStageLabel,
  getLoginAuditTargetAccount,
  getLoginAuditUserAgent,
} from '../../../helpers/loginAudit';

const LOGIN_AUDIT_COLUMN_KEYS = {
  TIME: 'audit_time',
  OUTCOME: 'audit_outcome',
  STAGE: 'audit_stage',
  REASON: 'audit_reason',
  METHOD: 'audit_method',
  ACCOUNT: 'audit_account',
  IP: 'audit_ip',
  USER_AGENT: 'audit_user_agent',
  DETAILS: 'audit_details',
};

function renderOutcome(record, t) {
  const outcome = getLoginAuditOutcome(record);
  return (
    <Tag color={getLoginAuditOutcomeColor(outcome)} shape='circle'>
      {getLoginAuditOutcomeLabel(outcome, t)}
    </Tag>
  );
}

function renderReason(record, t) {
  const audit = getLoginAudit(getLogOther(record.other));
  if (!audit?.reason) {
    return <></>;
  }
  const count = getLoginAuditAttemptCount(audit);
  return (
    <Space vertical align='start' spacing={2}>
      <span>{getLoginAuditReasonLabel(audit.reason, t)}</span>
      {count > 1 ? (
        <Typography.Text type='tertiary' size='small'>
          {t('次数')}: {count}
        </Typography.Text>
      ) : null}
    </Space>
  );
}

function renderIp(record, copyText) {
  if (!record.ip) {
    return <></>;
  }
  return (
    <Tooltip content={record.ip}>
      <span>
        <Tag
          color='orange'
          shape='circle'
          onClick={(event) => {
            copyText(event, record.ip);
          }}
        >
          {record.ip}
        </Tag>
      </span>
    </Tooltip>
  );
}

function renderUserAgent(record, other) {
  const userAgent = getLoginAuditUserAgent(record, other);
  if (!userAgent) {
    return <></>;
  }
  return (
    <Typography.Text
      ellipsis={{ showTooltip: true }}
      copyable={{ content: userAgent }}
      style={{ width: 180, fontFamily: 'monospace', fontSize: '12px' }}
    >
      {userAgent}
    </Typography.Text>
  );
}

export const getLoginAuditColumns = ({ t, isAdminUser, copyText }) => {
  return [
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.TIME,
      title: t('时间'),
      dataIndex: 'timestamp2string',
    },
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.OUTCOME,
      title: t('结果'),
      dataIndex: 'type',
      render: (text, record) => renderOutcome(record, t),
    },
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.STAGE,
      title: t('阶段'),
      dataIndex: 'other',
      render: (text) => {
        const audit = getLoginAudit(getLogOther(text));
        return audit?.stage ? (
          <>{getLoginAuditStageLabel(audit.stage, t)}</>
        ) : (
          <></>
        );
      },
    },
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.REASON,
      title: t('原因'),
      dataIndex: 'other',
      render: (text, record) => renderReason(record, t),
    },
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.METHOD,
      title: t('登录方式'),
      dataIndex: 'other',
      render: (text) => {
        const method = getLogOther(text)?.login_method;
        return method ? <>{String(method)}</> : <></>;
      },
    },
    ...(isAdminUser
      ? [
          {
            key: LOGIN_AUDIT_COLUMN_KEYS.ACCOUNT,
            title: t('目标账户'),
            dataIndex: 'username',
            render: (text, record) => {
              const account = getLoginAuditTargetAccount(
                getLogOther(record.other),
              );
              if (account) {
                return <>{account}</>;
              }
              return text ? <>{String(text)}</> : <></>;
            },
          },
        ]
      : []),
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.IP,
      title: t('IP'),
      dataIndex: 'ip',
      render: (text, record) => renderIp(record, copyText),
    },
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.USER_AGENT,
      title: t('User Agent'),
      dataIndex: 'user_agent',
      render: (text, record) =>
        renderUserAgent(record, getLogOther(record.other)),
    },
    {
      key: LOGIN_AUDIT_COLUMN_KEYS.DETAILS,
      title: t('详情'),
      dataIndex: 'content',
      fixed: 'right',
      width: 120,
      render: () => (
        <Typography.Text type='tertiary' size='small'>
          {t('点击行查看详情')}
        </Typography.Text>
      ),
    },
  ];
};
