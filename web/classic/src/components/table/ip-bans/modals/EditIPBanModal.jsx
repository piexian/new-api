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

import React, { useEffect, useRef, useState } from 'react';
import {
  Button,
  Form,
  Modal,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Title, Text } = Typography;

const getInitValues = () => ({
  target: '',
  reason: '',
  expires_at: null,
});

const toExpiresAt = (value) => {
  if (!value) return 0;
  const date = value instanceof Date ? value : new Date(value);
  const timestamp = Math.floor(date.getTime() / 1000);
  return Number.isFinite(timestamp) ? timestamp : 0;
};

const fromExpiresAt = (value) => {
  if (!value) return null;
  return new Date(value * 1000);
};

const needsSelfLockConfirmation = (response) =>
  response?.success === false && response?.data?.requires_confirmation === true;

const EditIPBanModal = ({ visible, editingIPBan, handleClose, refresh }) => {
  const isEdit = editingIPBan?.id !== undefined;
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);

  const loadIPBan = async () => {
    if (!editingIPBan?.id) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/ip_ban/${editingIPBan.id}`);
      const { success, message, data } = res.data;
      if (success) {
        formApiRef.current?.setValues({
          target: data.target || '',
          reason: data.reason || '',
          expires_at: fromExpiresAt(data.expires_at),
        });
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible || !formApiRef.current) return;
    if (isEdit) {
      loadIPBan();
    } else {
      formApiRef.current.setValues(getInitValues());
    }
  }, [visible, editingIPBan?.id]);

  const submitPayload = async (values, confirmed = false) => {
    const payload = {
      target: String(values.target || '').trim(),
      reason: String(values.reason || '').trim(),
      expires_at: toExpiresAt(values.expires_at),
      confirm_self_lock: confirmed,
    };
    return isEdit
      ? API.put(
          '/api/ip_ban/',
          { ...payload, id: editingIPBan.id },
          { skipErrorHandler: true },
        )
      : API.post('/api/ip_ban/', payload, { skipErrorHandler: true });
  };

  const submit = async (values, confirmed = false) => {
    setLoading(true);
    try {
      const res = await submitPayload(values, confirmed);
      const { success, message } = res.data;
      if (success) {
        showSuccess(isEdit ? t('更新成功') : t('创建成功'));
        await refresh();
        handleClose();
        return;
      }

      if (needsSelfLockConfirmation(res.data)) {
        Modal.confirm({
          title: t('确认封禁当前IP？'),
          content: (
            <div className='space-y-2'>
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

  return (
    <SideSheet
      placement={isEdit ? 'right' : 'left'}
      title={
        <Space>
          <Tag color={isEdit ? 'blue' : 'green'} shape='circle'>
            {isEdit ? t('更新') : t('新建')}
          </Tag>
          <Title heading={4} className='m-0'>
            {isEdit ? t('更新IP封禁规则') : t('创建IP封禁规则')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={visible}
      width={isMobile ? '100%' : 560}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button theme='solid' type='tertiary' onClick={handleClose}>
              {t('取消')}
            </Button>
            <Button
              theme='solid'
              type='primary'
              onClick={() => formApiRef.current?.submitForm()}
              loading={loading}
            >
              {t('提交')}
            </Button>
          </Space>
        </div>
      }
      onCancel={handleClose}
      maskClosable={false}
    >
      <Spin spinning={loading}>
        <Form
          getFormApi={(api) => {
            formApiRef.current = api;
          }}
          initValues={getInitValues()}
          onSubmit={(values) => submit(values)}
          className='p-4'
          labelPosition='top'
        >
          <Form.Input
            field='target'
            label={t('IP / CIDR')}
            placeholder='203.0.113.10 或 203.0.113.0/24'
            rules={[{ required: true, message: t('请输入IP或CIDR') }]}
          />
          <Form.TextArea
            field='reason'
            label={t('封禁原因')}
            placeholder={t('请输入封禁原因')}
            maxCount={255}
            autosize
            rules={[
              { required: true, message: t('请输入封禁原因') },
              { max: 255, message: t('封禁原因不能超过255个字符') },
            ]}
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
      </Spin>
    </SideSheet>
  );
};

export default EditIPBanModal;
