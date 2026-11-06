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
import { buildLoginAuditDetailRows } from '../../../helpers/loginAudit';

const labelStyle = {
  flex: '0 0 96px',
  color: 'var(--semi-color-text-2)',
  wordBreak: 'keep-all',
};

// 登录审计详情：只渲染白名单字段，User Agent 等长文本可滚动查看。
const LoginAuditDetail = ({ record, other, t, isAdminUser }) => {
  const rows = buildLoginAuditDetailRows({ record, other, t, isAdminUser });

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 6,
        maxWidth: 720,
        lineHeight: 1.6,
      }}
    >
      {rows.map((row) => (
        <div
          key={row.label}
          style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}
        >
          <span style={labelStyle}>{row.label}</span>
          <span
            style={{
              flex: 1,
              minWidth: 0,
              wordBreak: 'break-all',
              whiteSpace: 'pre-wrap',
              fontFamily: row.monospace ? 'monospace' : undefined,
              fontSize: row.monospace ? 12 : undefined,
              ...(row.scrollable
                ? { maxHeight: 160, overflowY: 'auto' }
                : null),
            }}
          >
            {row.value}
          </span>
        </div>
      ))}
    </div>
  );
};

export default LoginAuditDetail;
