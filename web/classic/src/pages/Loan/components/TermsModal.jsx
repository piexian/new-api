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

import React, { useState } from 'react';
import { Button, Checkbox, Modal, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';

/**
 * 强制且不可关闭的条款弹窗：启用条款且未同意时展示。
 * 用户必须勾选 18+ 声明后才能点击同意。
 */
const TermsModal = ({ t, visible, termsText, onAgreed }) => {
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const handleAgree = async () => {
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/loan/agree');
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('已同意条款'));
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
      title={t('词元贷条款')}
      visible={visible}
      // 不可跳过：仅在同意成功后由父组件关闭
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
        {t('使用词元贷功能前，请阅读并同意以下条款。')}
      </Typography.Text>
      <div className='mt-3 max-h-64 overflow-y-auto rounded-lg border p-3 text-sm break-words whitespace-pre-wrap bg-slate-50 dark:bg-slate-800'>
        {termsText}
      </div>
      <div className='mt-4'>
        <Checkbox
          checked={confirmed}
          onChange={(e) => setConfirmed(!!e.target.checked)}
        >
          {t('我已年满 18 周岁，并已阅读并同意以上条款。')}
        </Checkbox>
      </div>
    </Modal>
  );
};

export default TermsModal;
