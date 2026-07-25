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

import React, { useContext, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  updateAPI,
  setUserData,
} from '../../helpers';
import { UserContext } from '../../context/User';
import Loading from '../common/ui/Loading';
import { LOGIN_FEATURE_UPDATE_PROMPT_KEY } from '../../constants/common.constant';
import { Button, Card, Input, Typography } from '@douyinfe/semi-ui';

const { Title, Text } = Typography;

const OAuthOwnershipTransferPanel = ({
  transfer,
  error,
  sending,
  confirming,
  onSend,
  onConfirm,
  onReturn,
  t,
}) => {
  const [code, setCode] = useState('');
  const [remainingSeconds, setRemainingSeconds] = useState(0);

  useEffect(() => {
    if (!transfer?.code_sent || !transfer?.expires_at) {
      setRemainingSeconds(0);
      return;
    }
    const updateRemaining = () => {
      setRemainingSeconds(
        Math.max(0, transfer.expires_at - Math.floor(Date.now() / 1000)),
      );
    };
    updateRemaining();
    const timer = window.setInterval(updateRemaining, 1000);
    return () => window.clearInterval(timer);
  }, [transfer?.code_sent, transfer?.expires_at]);

  const countdown = `${String(Math.floor(remainingSeconds / 60)).padStart(2, '0')}:${String(
    remainingSeconds % 60,
  ).padStart(2, '0')}`;
  const expired =
    transfer?.code_sent &&
    transfer?.expires_at > 0 &&
    Math.floor(Date.now() / 1000) >= transfer.expires_at;
  const closed = transfer?.closed || expired;

  return (
    <div className='flex min-h-screen items-center justify-center px-4 py-10'>
      <Card className='w-full max-w-lg'>
        <div className='space-y-5'>
          <div className='text-center'>
            <Title heading={3}>{t('OAuth 邮箱归属验证')}</Title>
            <Text type='tertiary'>
              {t('邮箱 {{email}} 已绑定其他账号', {
                email: transfer?.email || '***',
              })}
            </Text>
          </div>

          <div className='rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-100'>
            {t(
              '这是当前 {{provider}} 账号与该邮箱唯一的一次验证机会。验证成功会永久封禁旧账号并将邮箱迁移到当前账号；验证码只发送一次，倒计时结束或连续错误 5 次后永久关闭，之后不会再次开放。',
              { provider: transfer?.provider || 'OAuth' },
            )}
          </div>

          {error ? (
            <div className='rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-200'>
              {error}
            </div>
          ) : null}

          {closed ? (
            <div className='space-y-4 text-center'>
              <Text type='danger'>
                {t('本次归属验证已永久关闭，之后不会再次开放。')}
              </Text>
              <Button block onClick={onReturn}>
                {t('返回登录')}
              </Button>
            </div>
          ) : transfer?.code_sent ? (
            <div className='space-y-4'>
              <Input
                value={code}
                maxLength={6}
                size='large'
                placeholder={t('请输入 6 位验证码')}
                onChange={setCode}
                disabled={confirming}
              />
              <Text type='tertiary' size='small'>
                {t('剩余时间：{{countdown}}；还可错误 {{remaining}} 次。', {
                  countdown,
                  remaining: transfer.attempts_remaining,
                })}
              </Text>
              <Button
                block
                theme='solid'
                type='danger'
                loading={confirming}
                disabled={code.length !== 6 || expired}
                onClick={() => onConfirm(code)}
              >
                {t('确认迁移并永久封禁旧账号')}
              </Button>
            </div>
          ) : (
            <Button
              block
              theme='solid'
              type='danger'
              loading={sending}
              onClick={onSend}
            >
              {t('我已了解，发送唯一验证码')}
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
};

const OAuth2Callback = (props) => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();
  const [ownershipTransfer, setOwnershipTransfer] = useState(null);
  const [ownershipError, setOwnershipError] = useState('');
  const [sendingOwnershipCode, setSendingOwnershipCode] = useState(false);
  const [confirmingOwnership, setConfirmingOwnership] = useState(false);

  // 防止 React 18 Strict Mode 下重复执行
  const hasExecuted = useRef(false);

  // 最大重试次数
  const MAX_RETRIES = 3;

  const sendCode = async (code, state, isSteam = false, retry = 0) => {
    try {
      // Steam 使用 OpenID 2.0，回调不含 code，仅有 openid.* 参数，
      // 将原始 query string（含 state + openid.*）透传给后端校验。
      const url = isSteam
        ? `/api/oauth/${props.type}${window.location.search}`
        : `/api/oauth/${props.type}?code=${code}&state=${state}`;
      const { data: resData } = await API.get(url);

      const { success, message, data } = resData;

      if (!success) {
        if (resData.code === 'oauth_ownership_transfer_required' && data) {
          setOwnershipTransfer(data);
          setOwnershipError('');
          return;
        }
        // 业务错误不重试，直接显示错误
        showError(message || t('授权失败'));
        return;
      }

      if (data?.action === 'bind') {
        showSuccess(t('绑定成功！'));
        navigate('/console/personal');
      } else {
        sessionStorage.setItem(LOGIN_FEATURE_UPDATE_PROMPT_KEY, '1');
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        setUserData(data);
        updateAPI();
        showSuccess(t('登录成功！'));
        navigate('/console/token');
      }
    } catch (error) {
      // 网络错误等可重试
      if (retry < MAX_RETRIES) {
        // 递增的退避等待
        await new Promise((resolve) => setTimeout(resolve, (retry + 1) * 2000));
        return sendCode(code, state, isSteam, retry + 1);
      }

      // 重试次数耗尽，提示错误并返回设置页面
      showError(error.message || t('授权失败'));
      navigate('/console/personal');
    }
  };

  useEffect(() => {
    // 防止 React 18 Strict Mode 下重复执行
    if (hasExecuted.current) {
      return;
    }
    hasExecuted.current = true;

    const initialize = async () => {
      try {
        const { data: statusResponse } = await API.get(
          '/api/oauth/ownership/status',
        );
        if (statusResponse?.data?.active || statusResponse?.data?.closed) {
          setOwnershipTransfer(statusResponse.data);
          setOwnershipError(statusResponse.message || '');
          return;
        }
      } catch (_error) {
        // Continue with the OAuth callback when there is no resumable challenge.
      }

      const isSteam = props.type === 'steam';
      const code = searchParams.get('code');
      const state = searchParams.get('state');

      // 参数缺失直接返回（Steam 使用 OpenID，回调无 code）
      if (!isSteam && !code) {
        showError(t('未获取到授权码'));
        navigate('/console/personal');
        return;
      }

      sendCode(code, state, isSteam);
    };

    initialize();
  }, []);

  const sendOwnershipCode = async () => {
    setSendingOwnershipCode(true);
    setOwnershipError('');
    try {
      const { data: response } = await API.post(
        '/api/oauth/ownership/send',
        {},
      );
      if (response?.data) setOwnershipTransfer(response.data);
      if (!response?.success)
        setOwnershipError(response?.message || t('发送失败'));
    } catch (error) {
      setOwnershipError(error.message || t('发送失败'));
    } finally {
      setSendingOwnershipCode(false);
    }
  };

  const confirmOwnership = async (code) => {
    setConfirmingOwnership(true);
    setOwnershipError('');
    try {
      const { data: response } = await API.post(
        '/api/oauth/ownership/confirm',
        { code },
      );
      if (!response?.success) {
        if (response?.data) setOwnershipTransfer(response.data);
        setOwnershipError(response?.message || t('验证失败'));
        return;
      }

      const data = response?.data;
      if (data?.action === 'bind') {
        showSuccess(t('邮箱归属迁移完成'));
        navigate('/console/personal');
        return;
      }
      sessionStorage.setItem(LOGIN_FEATURE_UPDATE_PROMPT_KEY, '1');
      userDispatch({ type: 'login', payload: data });
      localStorage.setItem('user', JSON.stringify(data));
      setUserData(data);
      updateAPI();
      showSuccess(t('邮箱归属迁移完成'));
      navigate('/console/token');
    } catch (error) {
      setOwnershipError(error.message || t('验证失败'));
    } finally {
      setConfirmingOwnership(false);
    }
  };

  if (ownershipTransfer) {
    return (
      <OAuthOwnershipTransferPanel
        transfer={ownershipTransfer}
        error={ownershipError}
        sending={sendingOwnershipCode}
        confirming={confirmingOwnership}
        onSend={sendOwnershipCode}
        onConfirm={confirmOwnership}
        onReturn={() => navigate('/login')}
        t={t}
      />
    );
  }

  return <Loading />;
};

export default OAuth2Callback;
