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
import {
  Descriptions,
  Modal,
  Spin,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { timestamp2string } from '../../../../helpers';

const { Text } = Typography;

const previewCsp =
  "<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; img-src data: cid:; style-src 'unsafe-inline'; font-src data:; base-uri 'none'; form-action 'none'\">";

const buildPreviewDocument = (content) => {
  if (/<head(?:\s[^>]*)?>/i.test(content)) {
    return content.replace(
      /<head(?:\s[^>]*)?>/i,
      (head) => `${head}${previewCsp}`,
    );
  }
  if (/<html(?:\s[^>]*)?>/i.test(content)) {
    return content.replace(
      /<html(?:\s[^>]*)?>/i,
      (html) => `${html}<head><meta charset="utf-8">${previewCsp}</head>`,
    );
  }
  return `<!doctype html><html><head><meta charset="utf-8">${previewCsp}</head><body>${content}</body></html>`;
};

const statusTag = (status, t) => {
  const mapping = {
    success: { color: 'green', label: t('成功') },
    failed: { color: 'red', label: t('失败') },
    suppressed: { color: 'orange', label: t('已抑制') },
  };
  const config = mapping[status] || { color: 'grey', label: status || '-' };
  return <Tag color={config.color}>{config.label}</Tag>;
};

const EmailLogDetailsModal = ({
  emailDetailVisible,
  emailDetailLoading,
  emailDetail,
  emailDetailError,
  closeEmailLogDetails,
  t,
}) => {
  const content = emailDetail?.content || '';

  return (
    <Modal
      title={t('Email Log Details')}
      visible={emailDetailVisible}
      onCancel={closeEmailLogDetails}
      footer={null}
      width={960}
      bodyStyle={{ maxHeight: '78vh', overflowY: 'auto' }}
    >
      {emailDetailLoading ? (
        <div className='flex min-h-64 items-center justify-center'>
          <Spin />
        </div>
      ) : emailDetailError ? (
        <div className='flex min-h-40 items-center justify-center text-red-500'>
          {emailDetailError}
        </div>
      ) : emailDetail ? (
        <div className='space-y-4'>
          <Descriptions row size='small'>
            <Descriptions.Item itemKey={t('发送时间')}>
              {emailDetail.created_at
                ? timestamp2string(emailDetail.created_at)
                : '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={t('状态')}>
              {statusTag(emailDetail.status, t)}
            </Descriptions.Item>
            <Descriptions.Item itemKey={t('收件人')}>
              {emailDetail.receiver || '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={t('提供商')}>
              {emailDetail.provider || '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={t('主题')}>
              {emailDetail.subject || '-'}
            </Descriptions.Item>
            {emailDetail.error_message ? (
              <Descriptions.Item itemKey={t('错误信息')}>
                <Text type='danger'>{emailDetail.error_message}</Text>
              </Descriptions.Item>
            ) : null}
          </Descriptions>

          {content ? (
            <Tabs type='line'>
              <TabPane tab={t('Preview')} itemKey='preview'>
                <iframe
                  title={t('Rendered Email')}
                  sandbox=''
                  referrerPolicy='no-referrer'
                  srcDoc={buildPreviewDocument(content)}
                  style={{
                    width: '100%',
                    height: 'min(58vh, 560px)',
                    minHeight: 320,
                    border: '1px solid var(--semi-color-border)',
                    borderRadius: 6,
                    background: 'white',
                  }}
                />
              </TabPane>
              <TabPane tab={t('HTML Source')} itemKey='source'>
                <pre className='max-h-[58vh] min-h-80 overflow-auto whitespace-pre-wrap break-all rounded-md border border-semi-color-border bg-semi-color-fill-0 p-3 text-xs'>
                  {content}
                </pre>
              </TabPane>
            </Tabs>
          ) : (
            <div className='flex min-h-64 items-center justify-center rounded-lg border border-dashed border-semi-color-border px-6 text-center text-semi-color-text-2'>
              {t('Email content is unavailable for older logs')}
            </div>
          )}
        </div>
      ) : null}
    </Modal>
  );
};

export default EmailLogDetailsModal;
