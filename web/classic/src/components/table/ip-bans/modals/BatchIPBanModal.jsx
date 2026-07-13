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

import React, { useRef, useState } from 'react';
import {
  Button,
  Form,
  Modal,
  Space,
  Spin,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const getInitValues = () => ({
  lines: '',
  default_reason: '',
  expires_at: null,
});

const toExpiresAt = (value) => {
  if (!value) return 0;
  const date = value instanceof Date ? value : new Date(value);
  const timestamp = Math.floor(date.getTime() / 1000);
  return Number.isFinite(timestamp) ? timestamp : 0;
};

const needsSelfLockConfirmation = (response) =>
  response?.success === false && response?.data?.requires_confirmation === true;

const BatchIPBanModal = ({ visible, handleClose, refresh }) => {
  const { t } = useTranslation();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [lastResult, setLastResult] = useState(null);

  const submitPayload = (values, confirmed = false) =>
    API.post(
      '/api/ip_ban/batch',
      {
        lines: values.lines || '',
        default_reason: String(values.default_reason || '').trim(),
        expires_at: toExpiresAt(values.expires_at),
        confirm_self_lock: confirmed,
      },
      { skipErrorHandler: true },
    );

  const submit = async (values, confirmed = false) => {
    setLoading(true);
    try {
      const res = await submitPayload(values, confirmed);
      const { success, message, data } = res.data;
      if (success) {
        setLastResult(data);
        showSuccess(t('批量导入完成'));
        await refresh();
        return;
      }

      if (needsSelfLockConfirmation(res.data)) {
        Modal.confirm({
          title: t('确认封禁当前IP？'),
          content: (
            <div>
              <Text>
                {message || t('该规则会封禁你当前的IP，请确认后再提交')}
              </Text>
              <br />
              <Text type='secondary'>
                {t('目标')}: {res.data.data.target}
              </Text>
              <br />
              <Text type='secondary'>
                {t('当前IP')}: {res.data.data.client_ip}
              </Text>
            </div>
          ),
          onOk: () => submit(values, true),
        });
      } else {
        showError(message);
      }
    } catch (error) {
      const responseData = error?.response?.data;
      if (needsSelfLockConfirmation(responseData)) {
        Modal.confirm({
          title: t('确认封禁当前IP？'),
          content:
            responseData.message || t('该规则会封禁你当前的IP，请确认后再提交'),
          onOk: () => submit(values, true),
        });
      } else {
        showError(responseData?.message || error);
      }
    } finally {
      setLoading(false);
    }
  };

  const close = () => {
    setLastResult(null);
    formApiRef.current?.setValues(getInitValues());
    handleClose();
  };

  return (
    <Modal
      title={t('批量导入IP封禁规则')}
      visible={visible}
      onCancel={close}
      footer={
        <Space>
          <Button onClick={close}>{t('关闭')}</Button>
          <Button
            type='primary'
            onClick={() => formApiRef.current?.submitForm()}
            loading={loading}
          >
            {t('导入')}
          </Button>
        </Space>
      }
      style={{ maxWidth: '920px' }}
      width='70%'
      maskClosable={false}
    >
      <Spin spinning={loading}>
        <Form
          getFormApi={(api) => {
            formApiRef.current = api;
          }}
          initValues={getInitValues()}
          onSubmit={(values) => submit(values)}
          labelPosition='top'
        >
          <Form.TextArea
            field='lines'
            label={t('IP列表')}
            placeholder={`203.0.113.10 | ${t('封禁原因')}\n203.0.113.0/24`}
            autosize={{ minRows: 8, maxRows: 14 }}
            rules={[{ required: true, message: t('请输入IP列表') }]}
          />
          <Form.TextArea
            field='default_reason'
            label={t('默认封禁原因')}
            placeholder={t('行内未填写原因时使用')}
            maxCount={255}
            autosize
          />
          <Form.DatePicker
            field='expires_at'
            label={t('过期时间')}
            type='dateTime'
            placeholder={t('留空表示永不过期')}
            showClear
            style={{ width: '100%' }}
          />
        </Form>

        {lastResult && (
          <div className='mt-4 rounded-lg border p-3 text-sm'>
            <div>
              {t('已创建')}: {lastResult.created || 0}
            </div>
            <div>
              {t('已跳过')}: {lastResult.skipped || 0}
            </div>
            <div>
              {t('无效行')}: {lastResult.invalid?.length || 0}
            </div>
            {lastResult.invalid?.length > 0 && (
              <TextArea
                readonly
                className='mt-2'
                autosize={{ minRows: 3, maxRows: 8 }}
                value={lastResult.invalid
                  .map(
                    (item) =>
                      `${item.line_number}: ${item.content} - ${item.message}`,
                  )
                  .join('\n')}
              />
            )}
          </div>
        )}
      </Spin>
    </Modal>
  );
};

export default BatchIPBanModal;
