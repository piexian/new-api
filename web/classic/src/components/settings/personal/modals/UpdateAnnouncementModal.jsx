import React from 'react';
import { Modal, Typography, Button, Space } from '@douyinfe/semi-ui';
import { IconEdit, IconLock } from '@douyinfe/semi-icons';

const { Title, Paragraph } = Typography;

const UpdateAnnouncementModal = ({
  t,
  visible,
  onClose,
  hasPassword,
  onChangePassword,
}) => {
  const handleChangePassword = () => {
    onClose();
    if (onChangePassword) {
      onChangePassword();
    }
  };

  return (
    <Modal
      title={t('功能更新提醒')}
      visible={visible}
      onCancel={onClose}
      footer={
        <Button theme="solid" onClick={onClose}>
          {t('我知道了')}
        </Button>
      }
      closeOnEsc
      maskClosable={false}
      style={{ maxWidth: 480 }}
    >
      <div style={{ padding: '8px 0' }}>
        <Paragraph style={{ marginBottom: 20, color: 'var(--semi-color-text-1)' }}>
          {t('本次更新新增了以下功能，您可以在个人设置中使用：')}
        </Paragraph>

        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, marginBottom: 16 }}>
          <IconEdit size="large" style={{ color: 'var(--semi-color-primary)', marginTop: 2, flexShrink: 0 }} />
          <div>
            <Title heading={6} style={{ marginBottom: 4 }}>{t('修改用户名')}</Title>
            <Paragraph type="tertiary" style={{ fontSize: 13 }}>
              {t('现在可以在账户管理中修改您的用户名和密码。')}
            </Paragraph>
          </div>
        </div>

        {!hasPassword && (
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, marginBottom: 16 }}>
            <IconLock size="large" style={{ color: 'var(--semi-color-warning)', marginTop: 2, flexShrink: 0 }} />
            <div>
              <Title heading={6} style={{ marginBottom: 4 }}>{t('设置密码')}</Title>
              <Paragraph type="tertiary" style={{ fontSize: 13 }}>
                {t('您的账户尚未设置密码。设置密码后可使用用户名密码登录，提升账户安全性。')}
              </Paragraph>
              <Button
                size="small"
                theme="light"
                style={{ marginTop: 8 }}
                onClick={handleChangePassword}
              >
                {t('立即设置')}
              </Button>
            </div>
          </div>
        )}
      </div>
    </Modal>
  );
};

export default UpdateAnnouncementModal;
