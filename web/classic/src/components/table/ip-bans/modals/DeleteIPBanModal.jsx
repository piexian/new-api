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
import { Modal } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';

const DeleteIPBanModal = ({ visible, onCancel, record, refresh, t }) => {
  const [loading, setLoading] = useState(false);

  const handleConfirm = async () => {
    if (!record?.id) return;
    setLoading(true);
    try {
      const res = await API.delete(`/api/ip_ban/${record.id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('删除成功'));
        await refresh();
        onCancel();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={t('确定删除此IP封禁规则？')}
      visible={visible}
      onCancel={onCancel}
      onOk={handleConfirm}
      confirmLoading={loading}
      type='warning'
    >
      {record?.target ? `${t('目标')}: ${record.target}` : t('此修改将不可逆')}
    </Modal>
  );
};

export default DeleteIPBanModal;
