import React, { useEffect, useState } from 'react';
import {
  Modal,
  Steps,
  Input,
  Typography,
  Button,
  Banner,
} from '@douyinfe/semi-ui';
import { IconLock, IconUser, IconShield, IconTick } from '@douyinfe/semi-icons';
import Turnstile from 'react-turnstile';

const SetupGuideModal = ({
  t,
  visible,
  onClose,
  userState,
  onComplete,
  turnstileEnabled,
  turnstileSiteKey,
  turnstileToken,
  setTurnstileToken,
}) => {
  const [currentStep, setCurrentStep] = useState(0);
  const [username, setUsername] = useState(userState?.user?.username || '');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const hasPassword = userState?.user?.has_password;

  useEffect(() => {
    if (visible && turnstileEnabled) {
      setTurnstileToken('');
    }
  }, [visible, turnstileEnabled, setTurnstileToken]);

  const buildSelfUpdateUrl = () =>
    turnstileToken
      ? `/api/user/self?turnstile=${encodeURIComponent(turnstileToken)}`
      : '/api/user/self';

  const handleSkipAll = async () => {
    await markCompleted();
  };

  const markCompleted = async () => {
    if (onComplete) {
      await onComplete();
    }
    onClose();
  };

  const handleUsernameNext = async () => {
    const trimmed = username.trim();
    if (!trimmed) {
      setError(t('用户名不能为空'));
      return;
    }
    if (trimmed.length > 20) {
      setError(t('用户名长度不能超过20个字符'));
      return;
    }

    // If username changed, save it
    if (trimmed !== userState?.user?.username) {
      setSaving(true);
      setError('');
      try {
        const { API, showSuccess } = await import('../../../../helpers');
        const res = await API.put(buildSelfUpdateUrl(), { username: trimmed });
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('用户名修改成功'));
        } else {
          setError(message);
          setSaving(false);
          return;
        }
      } catch {
        setError(t('操作失败，请重试'));
        setSaving(false);
        return;
      }
      setSaving(false);
    }
    setError('');
    setCurrentStep(1);
  };

  const handlePasswordNext = async () => {
    if (hasPassword) {
      // Already has password, skip
      setCurrentStep(2);
      return;
    }

    if (!password) {
      setError(t('请输入新密码！'));
      return;
    }
    if (password.length < 8) {
      setError(t('密码长度不能少于8位'));
      return;
    }
    if (password !== confirmPassword) {
      setError(t('两次输入的密码不一致！'));
      return;
    }

    setSaving(true);
    setError('');
    try {
      const { API, showSuccess } = await import('../../../../helpers');
      const res = await API.put(buildSelfUpdateUrl(), { password });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('密码设置成功'));
      } else {
        setError(message);
        setSaving(false);
        return;
      }
    } catch {
      setError(t('操作失败，请重试'));
      setSaving(false);
      return;
    }
    setSaving(false);
    setError('');
    setCurrentStep(2);
  };

  const handleFinish = async () => {
    await markCompleted();
  };

  const handleGoTo2FA = async () => {
    await markCompleted();
    // Navigate user to security tab - parent will handle
  };

  const renderStepContent = () => {
    switch (currentStep) {
      case 0:
        return (
          <div className='space-y-4 py-4'>
            <div className='text-center mb-4'>
              <div className='w-16 h-16 rounded-full bg-blue-50 flex items-center justify-center mx-auto mb-3'>
                <IconUser size='extra-large' className='text-blue-500' />
              </div>
              <Typography.Title heading={5}>{t('设置用户名')}</Typography.Title>
              <Typography.Text type='tertiary'>
                {t('设置一个方便记忆的用户名')}
              </Typography.Text>
            </div>

            <div>
              <Typography.Text strong className='block mb-2'>
                {t('用户名')}
              </Typography.Text>
              <Input
                value={username}
                onChange={setUsername}
                placeholder={t('请输入用户名')}
                size='large'
                maxLength={20}
                prefix={<IconUser />}
                className='!rounded-lg'
              />
            </div>

            {error && (
              <Banner type='danger' description={error} closeIcon={null} />
            )}

            {turnstileEnabled && (
              <div className='flex justify-center pt-2'>
                <Turnstile
                  sitekey={turnstileSiteKey}
                  onVerify={(token) => {
                    setTurnstileToken(token);
                  }}
                  onExpire={() => {
                    setTurnstileToken('');
                  }}
                />
              </div>
            )}
          </div>
        );
      case 1:
        return (
          <div className='space-y-4 py-4'>
            <div className='text-center mb-4'>
              <div className='w-16 h-16 rounded-full bg-orange-50 flex items-center justify-center mx-auto mb-3'>
                <IconLock size='extra-large' className='text-orange-500' />
              </div>
              <Typography.Title heading={5}>
                {hasPassword ? t('密码已设置') : t('设置密码')}
              </Typography.Title>
              <Typography.Text type='tertiary'>
                {hasPassword
                  ? t('您已设置密码，可继续下一步')
                  : t('设置密码后可使用用户名密码登录')}
              </Typography.Text>
            </div>

            {hasPassword ? (
              <div className='flex items-center justify-center py-6'>
                <div className='w-12 h-12 rounded-full bg-green-50 flex items-center justify-center'>
                  <IconTick size='extra-large' className='text-green-500' />
                </div>
              </div>
            ) : (
              <>
                <div>
                  <Typography.Text strong className='block mb-2'>
                    {t('新密码')}
                  </Typography.Text>
                  <Input
                    value={password}
                    onChange={setPassword}
                    placeholder={t('请输入新密码')}
                    type='password'
                    size='large'
                    prefix={<IconLock />}
                    className='!rounded-lg'
                  />
                </div>
                <div>
                  <Typography.Text strong className='block mb-2'>
                    {t('确认新密码')}
                  </Typography.Text>
                  <Input
                    value={confirmPassword}
                    onChange={setConfirmPassword}
                    placeholder={t('请再次输入新密码')}
                    type='password'
                    size='large'
                    prefix={<IconLock />}
                    className='!rounded-lg'
                  />
                </div>
              </>
            )}

            {error && (
              <Banner type='danger' description={error} closeIcon={null} />
            )}

            {!hasPassword && turnstileEnabled && (
              <div className='flex justify-center pt-2'>
                <Turnstile
                  sitekey={turnstileSiteKey}
                  onVerify={(token) => {
                    setTurnstileToken(token);
                  }}
                  onExpire={() => {
                    setTurnstileToken('');
                  }}
                />
              </div>
            )}
          </div>
        );
      case 2:
        return (
          <div className='space-y-4 py-4'>
            <div className='text-center mb-4'>
              <div className='w-16 h-16 rounded-full bg-green-50 flex items-center justify-center mx-auto mb-3'>
                <IconShield size='extra-large' className='text-green-500' />
              </div>
              <Typography.Title heading={5}>
                {t('启用两步验证')}
              </Typography.Title>
              <Typography.Text type='tertiary'>
                {t('建议启用两步验证以提高账户安全性')}
              </Typography.Text>
            </div>

            <Banner
              type='info'
              description={t(
                '两步验证可以有效防止他人未经授权访问您的账户，即使密码泄露也能保障安全。此步骤非强制要求，您可以稍后在安全设置中启用。',
              )}
              closeIcon={null}
            />
          </div>
        );
      default:
        return null;
    }
  };

  const renderFooter = () => {
    switch (currentStep) {
      case 0:
        return (
          <div className='flex justify-between w-full'>
            <Button onClick={handleSkipAll} type='tertiary'>
              {t('跳过全部')}
            </Button>
            <div className='flex gap-2'>
              <Button
                onClick={() => {
                  setError('');
                  setCurrentStep(1);
                }}
                type='tertiary'
              >
                {t('跳过')}
              </Button>
              <Button
                theme='solid'
                onClick={handleUsernameNext}
                loading={saving}
              >
                {t('下一步')}
              </Button>
            </div>
          </div>
        );
      case 1:
        return (
          <div className='flex justify-between w-full'>
            <Button
              onClick={() => {
                setError('');
                setCurrentStep(0);
              }}
              type='tertiary'
            >
              {t('上一步')}
            </Button>
            <div className='flex gap-2'>
              {!hasPassword && (
                <Button
                  onClick={() => {
                    setError('');
                    setCurrentStep(2);
                  }}
                  type='tertiary'
                >
                  {t('跳过')}
                </Button>
              )}
              <Button
                theme='solid'
                onClick={handlePasswordNext}
                loading={saving}
              >
                {hasPassword ? t('下一步') : t('设置并继续')}
              </Button>
            </div>
          </div>
        );
      case 2:
        return (
          <div className='flex justify-between w-full'>
            <Button
              onClick={() => {
                setError('');
                setCurrentStep(1);
              }}
              type='tertiary'
            >
              {t('上一步')}
            </Button>
            <div className='flex gap-2'>
              <Button onClick={handleFinish} type='tertiary'>
                {t('稍后再说')}
              </Button>
              <Button theme='solid' onClick={handleGoTo2FA}>
                {t('去设置')}
              </Button>
            </div>
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <IconShield className='mr-2 text-blue-500' />
          {t('欢迎！完成以下设置以保护您的账户')}
        </div>
      }
      visible={visible}
      onCancel={handleSkipAll}
      footer={renderFooter()}
      size='medium'
      centered={true}
      className='modern-modal'
      maskClosable={false}
    >
      <Steps current={currentStep} size='small' className='mb-4'>
        <Steps.Step title={t('用户名')} icon={<IconUser />} />
        <Steps.Step title={t('密码')} icon={<IconLock />} />
        <Steps.Step title={t('安全')} icon={<IconShield />} />
      </Steps>
      {renderStepContent()}
    </Modal>
  );
};

export default SetupGuideModal;
