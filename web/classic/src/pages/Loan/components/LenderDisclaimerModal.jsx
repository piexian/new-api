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

import React, { useEffect, useState } from 'react';
import { Button, Checkbox, Modal, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';

/**
 * 放贷免责声明弹窗：首次创建挂单前必须确认（未同意时后端会拒绝创建）。
 * 与词元贷 18+ 声明弹窗同构：勾选确认后调用 disclaimer 接口，成功后由父组件关闭。
 */
const LenderDisclaimerModal = ({ t, visible, onAgreed, onCancel }) => {
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!visible) {
      setConfirmed(false);
      setSubmitting(false);
    }
  }, [visible]);

  const handleAgree = async () => {
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/loan/market/disclaimer');
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('已同意放贷免责声明'));
        onAgreed?.();
      } else {
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={t('放贷免责声明')}
      visible={visible}
      onCancel={onCancel}
      closable={false}
      maskClosable={false}
      closeOnEsc={false}
      footer={
        <Button
          type='primary'
          theme='solid'
          onClick={handleAgree}
          loading={submitting}
          disabled={!confirmed || submitting}
        >
          {submitting ? t('提交中...') : t('我同意')}
        </Button>
      }
    >
      <Typography.Text type='secondary' size='small'>
        {t('开始放贷前，请阅读并同意以下免责声明。')}
      </Typography.Text>
      <div className='mt-3 max-h-64 overflow-y-auto rounded-lg border p-3 text-sm break-words whitespace-pre-wrap bg-slate-50 dark:bg-slate-800 space-y-2'>
        <p>{t('在词元贷市场放贷纯属娱乐玩法。')}</p>
        <p>{t('并非真实金融。借款人违约时，出借额度可能全部损失。')}</p>
        <p>{t('平台不兜底、不代替放贷人追偿。')}</p>
      </div>
      <div className='mt-4'>
        <Checkbox
          checked={confirmed}
          onChange={(e) => setConfirmed(!!e.target.checked)}
        >
          {t('我已阅读并同意上述免责声明，知晓相关风险并自愿承担。')}
        </Checkbox>
      </div>
    </Modal>
  );
};

export default LenderDisclaimerModal;
