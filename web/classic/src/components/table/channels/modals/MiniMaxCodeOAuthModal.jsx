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
import {
  Banner,
  Button,
  Modal,
  Radio,
  RadioGroup,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const DEFAULT_POLL_INTERVAL_MS = 5000;

/**
 * MiniMax Code 设备码登录弹窗。
 * 传入 channelId 时为渠道重授权模式（就绪后由后端直接保存）；
 * 不传时为新建渠道模式，就绪后通过 onSuccess(key) 返回凭证 JSON。
 */
const MiniMaxCodeOAuthModal = ({ visible, onCancel, onSuccess, channelId }) => {
  const { t } = useTranslation();
  const [region, setRegion] = useState('cn');
  const [phase, setPhase] = useState('idle'); // idle | starting | authorizing | failed | ready
  const [userCode, setUserCode] = useState('');
  const [verificationUrl, setVerificationUrl] = useState('');
  const [verificationUrlComplete, setVerificationUrlComplete] = useState('');
  const [failure, setFailure] = useState('');
  const [result, setResult] = useState(null);
  const [starting, setStarting] = useState(false);
  const pollTimerRef = useRef(null);
  const intervalMsRef = useRef(DEFAULT_POLL_INTERVAL_MS);
  const generationRef = useRef(0);
  const closedRef = useRef(true);

  const clearPollTimer = () => {
    if (pollTimerRef.current) {
      clearTimeout(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  };

  const schedulePoll = useCallback(
    (generation) => {
      clearPollTimer();
      pollTimerRef.current = setTimeout(async () => {
        if (closedRef.current || generation !== generationRef.current) return;
        try {
          const res = await API.post(
            channelId
              ? `/api/channel/${channelId}/minimaxcode/oauth/poll`
              : '/api/channel/minimaxcode/oauth/poll',
            {},
            { skipErrorHandler: true },
          );
          const data = res?.data?.data;
          if (!res?.data?.success) {
            schedulePoll(generation);
            return;
          }
          if (data?.status === 'ready') {
            setPhase('ready');
            setResult(data || null);
            if (channelId) {
              showSuccess(t('授权成功，渠道密钥已更新'));
            }
            if (!channelId && data?.key) {
              onSuccess?.(data.key);
              showSuccess(t('凭证已生成'));
              onCancel?.();
            }
            return;
          }
          if (data?.status === 'denied') {
            setPhase('failed');
            setFailure(t('授权被拒绝'));
            return;
          }
          if (data?.status === 'expired') {
            setPhase('failed');
            setFailure(t('授权流程已过期，请重试'));
            return;
          }
          schedulePoll(generation);
        } catch {
          if (!closedRef.current && generation === generationRef.current) {
            schedulePoll(generation);
          }
        }
      }, intervalMsRef.current);
    },
    [channelId, onCancel, onSuccess, t],
  );

  const startFlow = async () => {
    if (starting) return;
    setStarting(true);
    const generation = ++generationRef.current;
    clearPollTimer();
    setFailure('');
    setResult(null);
    setUserCode('');
    setVerificationUrl('');
    setPhase('starting');
    try {
      const res = await API.post(
        channelId
          ? `/api/channel/${channelId}/minimaxcode/oauth/start`
          : '/api/channel/minimaxcode/oauth/start',
        { region },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success || !res?.data?.data?.user_code) {
        throw new Error(res?.data?.message || t('发起授权失败'));
      }
      const data = res.data.data;
      setUserCode(data.user_code || '');
      setVerificationUrl(data.verification_url || '');
      setVerificationUrlComplete(data.verification_url_complete || '');
      const interval = Number(data.interval_ms);
      intervalMsRef.current =
        Number.isFinite(interval) && interval > 0
          ? Math.max(interval, 1000)
          : DEFAULT_POLL_INTERVAL_MS;
      closedRef.current = false;
      setPhase('authorizing');
      schedulePoll(generation);
    } catch (error) {
      setPhase('failed');
      setFailure(error?.message || t('发起授权失败'));
    } finally {
      setStarting(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      closedRef.current = true;
      generationRef.current += 1;
      clearPollTimer();
      setPhase('idle');
      setUserCode('');
      setVerificationUrl('');
      setVerificationUrlComplete('');
      setFailure('');
      setResult(null);
      return;
    }
    setRegion('cn');
  }, [visible]);

  return (
    <Modal
      title={t('MiniMax Code OAuth 登录')}
      visible={visible}
      onCancel={onCancel}
      maskClosable={false}
      closeOnEsc
      width={560}
      footer={
        <Space>
          <Button theme='borderless' onClick={onCancel}>
            {t('取消')}
          </Button>
          {(phase === 'idle' || phase === 'failed') && (
            <Button theme='solid' type='primary' onClick={startFlow}>
              {phase === 'failed' ? t('重试') : t('开始授权')}
            </Button>
          )}
        </Space>
      }
    >
      <Space vertical spacing='tight' style={{ width: '100%' }}>
        <Banner
          type='info'
          description={t(
            '使用设备码登录你的 MiniMax 账户，访问令牌会在过期前自动刷新。',
          )}
        />

        <div>
          <Text type='tertiary' size='small'>
            {t('账户区域')}
          </Text>
          <RadioGroup
            value={region}
            disabled={phase !== 'idle' && phase !== 'failed'}
            onChange={(e) => setRegion(e.target.value)}
            type='button'
          >
            <Radio value='cn'>{t('MiniMax 国内站 (minimax.cn)')}</Radio>
            <Radio value='global'>{t('MiniMax 海外站 (minimax.io)')}</Radio>
          </RadioGroup>
        </div>

        {phase === 'starting' && (
          <Space>
            <Spin size='small' />
            <Text type='tertiary'>{t('正在初始化授权…')}</Text>
          </Space>
        )}

        {phase === 'authorizing' && (
          <Space vertical spacing='tight' style={{ width: '100%' }}>
            <Space>
              <Spin size='small' />
              <Text type='tertiary'>{t('等待授权完成…')}</Text>
            </Space>
            <Space>
              <Text>{t('验证码：')}</Text>
              <Tag color='blue' size='large'>
                {userCode}
              </Tag>
            </Space>
            {verificationUrl && (
              <Button
                theme='outline'
                onClick={() =>
                  window.open(verificationUrl, '_blank', 'noopener')
                }
              >
                {t('打开授权页面')}
              </Button>
            )}
          </Space>
        )}

        {phase === 'ready' && (
          <Space vertical spacing='tight' style={{ width: '100%' }}>
            <Text type='success'>{t('授权成功，渠道密钥已更新')}</Text>
            {result?.group_id && (
              <Text type='tertiary'>
                {t('套餐分组')}：{result.group_id}
              </Text>
            )}
          </Space>
        )}

        {failure && <Text type='danger'>{failure}</Text>}
      </Space>
    </Modal>
  );
};

export default MiniMaxCodeOAuthModal;
