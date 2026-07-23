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

import React, { useEffect, useMemo, useState } from 'react';
import { Button, Modal } from '@douyinfe/semi-ui';
import ReactMarkdown from 'react-markdown';
import RemarkGfm from 'remark-gfm';
import RemarkBreaks from 'remark-breaks';
import RehypeRaw from 'rehype-raw';
import RehypeSanitize from 'rehype-sanitize';
import { useTranslation } from 'react-i18next';
import { ACCOUNT_DISABLED_DIALOG_EVENT } from '../../helpers';

export default function AccountDisabledDialog() {
  const { t } = useTranslation();
  const [payload, setPayload] = useState(null);

  useEffect(() => {
    const handleAccountDisabled = (event) => {
      setPayload(event.detail || {});
    };

    window.addEventListener(
      ACCOUNT_DISABLED_DIALOG_EVENT,
      handleAccountDisabled,
    );
    return () => {
      window.removeEventListener(
        ACCOUNT_DISABLED_DIALOG_EVENT,
        handleAccountDisabled,
      );
    };
  }, []);

  const content = useMemo(() => {
    return payload?.reason || payload?.message || t('此账号已被封禁。');
  }, [payload, t]);

  const accountMeta = useMemo(() => {
    const items = [];
    if (
      payload?.userId !== undefined &&
      payload?.userId !== null &&
      String(payload.userId).trim() !== '' &&
      String(payload.userId) !== '0'
    ) {
      items.push(`ID: ${payload.userId}`);
    }
    if (typeof payload?.username === 'string' && payload.username.trim()) {
      items.push(payload.username.trim());
    }
    return items.join(' · ');
  }, [payload]);

  const closeDialog = () => setPayload(null);
  const isTemporary = Number(payload?.disabledUntil) > 0;
  const unbanTime = isTemporary
    ? new Intl.DateTimeFormat(undefined, {
        dateStyle: 'medium',
        timeStyle: 'medium',
      }).format(new Date(payload.disabledUntil * 1000))
    : '';

  return (
    <Modal
      title={
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            flexWrap: 'wrap',
            gap: 8,
          }}
        >
          {accountMeta && (
            <span
              style={{
                maxWidth: '100%',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                border: '1px solid var(--semi-color-border)',
                borderRadius: 6,
                padding: '2px 8px',
                color: 'var(--semi-color-text-2)',
                background: 'var(--semi-color-fill-0)',
                fontSize: 12,
                fontWeight: 500,
              }}
            >
              {accountMeta}
            </span>
          )}
          <span>{t('账号已被封禁')}</span>
        </div>
      }
      visible={Boolean(payload)}
      onCancel={closeDialog}
      footer={<Button onClick={closeDialog}>{t('关闭')}</Button>}
      width={720}
      style={{ maxWidth: '92vw' }}
      bodyStyle={{ maxHeight: '70vh', overflowY: 'auto' }}
      centered
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: isTemporary
            ? 'repeat(2, minmax(0, 1fr))'
            : '1fr',
          gap: 12,
          marginBottom: 16,
          padding: 12,
          border: '1px solid var(--semi-color-border)',
          borderRadius: 6,
          background: 'var(--semi-color-fill-0)',
        }}
      >
        <div>
          <div style={{ color: 'var(--semi-color-text-2)', fontSize: 12 }}>
            {t('封禁类型')}
          </div>
          <div>{isTemporary ? t('临时封禁') : t('永久封禁')}</div>
        </div>
        {isTemporary && (
          <div>
            <div style={{ color: 'var(--semi-color-text-2)', fontSize: 12 }}>
              {t('自动解封时间')}
            </div>
            <div>{unbanTime}</div>
          </div>
        )}
      </div>
      <div
        className='account-disabled-dialog-content'
        style={{
          fontSize: 14,
          lineHeight: 1.7,
          color: 'var(--semi-color-text-0)',
          wordBreak: 'break-word',
        }}
      >
        <ReactMarkdown
          remarkPlugins={[RemarkGfm, RemarkBreaks]}
          rehypePlugins={[RehypeRaw, RehypeSanitize]}
          components={{
            a: (props) => (
              <a {...props} target='_blank' rel='noopener noreferrer' />
            ),
            p: (props) => <p {...props} style={{ margin: '0 0 12px' }} />,
            ol: (props) => (
              <ol {...props} style={{ paddingLeft: 22, margin: '0 0 12px' }} />
            ),
            ul: (props) => (
              <ul {...props} style={{ paddingLeft: 22, margin: '0 0 12px' }} />
            ),
          }}
        >
          {content}
        </ReactMarkdown>
      </div>
    </Modal>
  );
}
