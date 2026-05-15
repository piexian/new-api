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

import React from 'react';
import { Modal, TextArea, Typography } from '@douyinfe/semi-ui';

const EnableDisableUserModal = ({
  visible,
  onCancel,
  onConfirm,
  user,
  action,
  disableReason,
  setDisableReason,
  t,
}) => {
  const isDisable = action === 'disable';
  const { Text } = Typography;

  return (
    <Modal
      title={isDisable ? t('确定要禁用此用户吗？') : t('确定要启用此用户吗？')}
      visible={visible}
      onCancel={onCancel}
      onOk={onConfirm}
      type='warning'
    >
      {isDisable ? (
        <div>
          <Text>{t('此操作将禁用用户账户')}</Text>
          <TextArea
            value={disableReason}
            onChange={(value) => setDisableReason(value)}
            placeholder={t('请输入禁用原因')}
            autosize
            maxCount={5000}
            showClear
            style={{ marginTop: 12 }}
          />
          <Text type='tertiary' size='small'>
            {t('用户下次登录时将看到该原因')}
          </Text>
        </div>
      ) : (
        t('此操作将启用用户账户')
      )}
    </Modal>
  );
};

export default EnableDisableUserModal;
