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

import React, { useEffect } from 'react';
import { Modal, Typography, Input, InputNumber } from '@douyinfe/semi-ui';
import { CreditCard } from 'lucide-react';

const TransferModal = ({
  t,
  openTransfer,
  transfer,
  handleTransferCancel,
  userState,
  renderQuota,
  getQuotaPerUnit,
  transferAmount,
  setTransferAmount,
}) => {
  const availableQuota = Number(userState?.user?.aff_quota || 0);
  const quotaPerUnit = Number(getQuotaPerUnit() || 1);
  const defaultAmount =
    availableQuota > 0 && availableQuota < quotaPerUnit
      ? availableQuota
      : quotaPerUnit;

  useEffect(() => {
    if (openTransfer) {
      setTransferAmount(defaultAmount);
    }
  }, [openTransfer, defaultAmount, setTransferAmount]);

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <CreditCard className='mr-2' size={18} />
          {t('划转邀请额度')}
        </div>
      }
      visible={openTransfer}
      onOk={transfer}
      onCancel={handleTransferCancel}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('可用邀请额度')}
          </Typography.Text>
          <Input
            value={renderQuota(userState?.user?.aff_quota)}
            disabled
            className='!rounded-lg'
          />
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('划转额度')}
          </Typography.Text>
          <InputNumber
            min={1}
            max={availableQuota}
            step={1}
            precision={0}
            value={transferAmount}
            onChange={(value) => setTransferAmount(value)}
            className='w-full !rounded-lg'
          />
          <Typography.Text type='tertiary' size='small'>
            {availableQuota < quotaPerUnit
              ? t('低于原最低额度，已默认选择全部可用奖励')
              : t('可划转任意正数奖励额度')}
          </Typography.Text>
        </div>
      </div>
    </Modal>
  );
};

export default TransferModal;
