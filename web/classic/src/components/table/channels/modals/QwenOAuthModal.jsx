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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Modal,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, copy, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const QwenOAuthModal = ({
  visible,
  onCancel,
  onSuccess,
  apiKey,
  channelId,
}) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [verificationUrl, setVerificationUrl] = useState('');
  const [status, setStatus] = useState('idle');
  const [identity, setIdentity] = useState(null);
  const timerRef = useRef(null);
  const cancelledRef = useRef(false);

  const stopPolling = useCallback(() => {
    cancelledRef.current = true;
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  useEffect(() => {
    return stopPolling;
  }, [stopPolling]);

  useEffect(() => {
    if (visible) return;
    stopPolling();
    setVerificationUrl('');
    setStatus('idle');
    setIdentity(null);
  }, [stopPolling, visible]);

  const startOAuth = async () => {
    const normalizedApiKey = String(apiKey || '').trim();
    if (!channelId && !normalizedApiKey.startsWith('sk-sp-')) {
      showError(t('请先输入有效的 sk-sp- Token Plan API Key'));
      return;
    }

    stopPolling();
    cancelledRef.current = false;
    setLoading(true);
    setStatus('idle');
    setIdentity(null);
    try {
      const startPath = channelId
        ? `/api/channel/${channelId}/qwen/oauth/start`
        : '/api/channel/qwen/oauth/start';
      const response = await API.post(
        startPath,
        {},
        { skipErrorHandler: true },
      );
      const url = response?.data?.data?.verification_url || '';
      if (!response?.data?.success || !url) {
        throw new Error(response?.data?.message || t('启动授权失败'));
      }

      const initialInterval = Math.max(
        Number(response?.data?.data?.interval || 5),
        1,
      );
      setVerificationUrl(url);
      setStatus('pending');
      window.open(url, '_blank', 'noopener,noreferrer');

      const poll = async (intervalSeconds) => {
        if (cancelledRef.current) return;
        try {
          const completePath = channelId
            ? `/api/channel/${channelId}/qwen/oauth/complete`
            : '/api/channel/qwen/oauth/complete';
          const result = await API.post(
            completePath,
            { api_key: normalizedApiKey },
            { skipErrorHandler: true },
          );
          if (!result?.data?.success) {
            throw new Error(result?.data?.message || t('授权失败'));
          }

          const nextStatus =
            result?.data?.data?.status || 'authorization_pending';
          if (nextStatus === 'complete') {
            const nextIdentity = {
              email: result?.data?.data?.email,
              aliyunId: result?.data?.data?.aliyun_id,
              expiresAt: result?.data?.data?.expires_at,
            };
            stopPolling();
            setStatus('complete');
            setIdentity(nextIdentity);
            onSuccess?.(result?.data?.data?.key, nextIdentity);
            showSuccess(t('Qwen Token Plan 授权已绑定'));
            return;
          }
          if (nextStatus === 'access_denied') {
            throw new Error(t('授权已拒绝'));
          }
          if (nextStatus === 'expired_token') {
            throw new Error(t('授权会话已过期'));
          }

          const nextInterval =
            nextStatus === 'slow_down' ? intervalSeconds + 5 : intervalSeconds;
          timerRef.current = setTimeout(
            () => poll(nextInterval),
            nextInterval * 1000,
          );
        } catch (error) {
          stopPolling();
          setStatus('idle');
          showError(error?.message || t('授权失败'));
        }
      };

      timerRef.current = setTimeout(
        () => poll(initialInterval),
        initialInterval * 1000,
      );
    } catch (error) {
      stopPolling();
      showError(error?.message || t('启动授权失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={t('Qwen Token Plan 授权')}
      visible={visible}
      onCancel={() => {
        stopPolling();
        onCancel?.();
      }}
      maskClosable={false}
      width={720}
      footer={
        <Button
          type='primary'
          theme='solid'
          onClick={() => {
            stopPolling();
            onCancel?.();
          }}
        >
          {t('关闭')}
        </Button>
      }
    >
      <Space vertical spacing='medium' style={{ width: '100%' }}>
        <Banner
          type='info'
          description={t(
            'OAuth 用于查询 Token Plan 套餐与用量，sk-sp- API Key 仍用于模型推理。授权页完成登录后，本页面会自动轮询并绑定凭据。',
          )}
        />
        <Banner
          type='warning'
          description={t(
            '千问目前没有公开接口可以证明 sk-sp- API Key 与 OAuth 登录账号属于同一主体。New API 会将二者保存在同一条渠道凭据中，并分别校验有效性。',
          )}
        />
        <Space wrap>
          <Button
            type='primary'
            onClick={startOAuth}
            loading={loading || status === 'pending'}
            disabled={status === 'pending'}
          >
            {status === 'pending' ? t('等待授权中...') : t('打开授权页面')}
          </Button>
          <Button
            theme='outline'
            disabled={!verificationUrl}
            onClick={() => copy(verificationUrl)}
          >
            {t('复制授权链接')}
          </Button>
        </Space>
        {verificationUrl && (
          <Text type='tertiary' size='small' style={{ wordBreak: 'break-all' }}>
            {verificationUrl}
          </Text>
        )}
        {status === 'complete' && identity && (
          <Space vertical spacing='tight'>
            <Tag color='green'>{t('授权完成')}</Tag>
            <Text>{identity.email || identity.aliyunId || t('未知账号')}</Text>
            {identity.expiresAt && (
              <Text type='tertiary' size='small'>
                {t('到期时间')}: {identity.expiresAt}
              </Text>
            )}
          </Space>
        )}
      </Space>
    </Modal>
  );
};

export default QwenOAuthModal;
