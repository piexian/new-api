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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Space, Table, Typography } from '@douyinfe/semi-ui';
import { RotateCcw, Users } from 'lucide-react';
import {
  API,
  copy,
  getQuotaPerUnit,
  renderQuota,
  showError,
  showSuccess,
} from '../../helpers';
import { UserContext } from '../../context/User';
import InvitationCard from '../topup/InvitationCard';
import TransferModal from '../topup/modals/TransferModal';

const { Text, Title } = Typography;

const InviteRewards = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [affLink, setAffLink] = useState('');
  const [invitedUsers, setInvitedUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(getQuotaPerUnit());
  const [complianceConfirmed, setComplianceConfirmed] = useState(true);

  const columns = useMemo(
    () => [
      {
        title: t('用户ID'),
        dataIndex: 'id',
        width: 100,
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
      },
      {
        title: t('显示名称'),
        dataIndex: 'display_name',
        render: (value) => value || '-',
      },
      {
        title: t('注册时间'),
        dataIndex: 'created_time',
        render: (value) =>
          value ? new Date(value * 1000).toLocaleString() : '-',
      },
    ],
    [t],
  );

  const getUserQuota = async () => {
    const res = await API.get('/api/user/self');
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      setAffLink(`${window.location.origin}/register?aff=${data}`);
    } else {
      showError(message);
    }
  };

  const getInvitedUsers = async () => {
    const res = await API.get('/api/user/aff/invited');
    const { success, message, data } = res.data;
    if (success) {
      setInvitedUsers(Array.isArray(data) ? data : []);
    } else {
      showError(message);
    }
  };

  const getTopupInfo = async () => {
    const res = await API.get('/api/user/topup/info');
    if (res.data?.success) {
      setComplianceConfirmed(
        res.data.data?.payment_compliance_confirmed !== false,
      );
    }
  };

  const refreshData = async () => {
    setLoading(true);
    try {
      await Promise.all([
        getUserQuota(),
        getAffLink(),
        getInvitedUsers(),
        getTopupInfo(),
      ]);
    } finally {
      setLoading(false);
    }
  };

  const resetAffCode = async () => {
    const res = await API.post('/api/user/aff/reset');
    const { success, message, data } = res.data;
    if (success) {
      setAffLink(`${window.location.origin}/register?aff=${data}`);
      showSuccess(t('邀请码已重置'));
    } else {
      showError(message);
    }
  };

  const transfer = async () => {
    if (transferAmount <= 0) {
      showError(t('划转额度必须大于 0'));
      return;
    }
    const res = await API.post('/api/user/aff_transfer', {
      quota: transferAmount,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message);
      setOpenTransfer(false);
      await getUserQuota();
    } else {
      showError(message);
    }
  };

  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('邀请链接已复制到剪切板'));
  };

  useEffect(() => {
    refreshData().then();
  }, []);

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2'>
      <TransferModal
        t={t}
        openTransfer={openTransfer}
        transfer={transfer}
        handleTransferCancel={() => setOpenTransfer(false)}
        userState={userState}
        renderQuota={renderQuota}
        getQuotaPerUnit={getQuotaPerUnit}
        transferAmount={transferAmount}
        setTransferAmount={setTransferAmount}
      />

      <div className='mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <Title heading={3} className='!mb-1'>
            {t('邀请奖励')}
          </Title>
          <Text type='secondary'>
            {t('管理邀请链接、奖励额度和已邀请用户')}
          </Text>
        </div>
        <Button
          icon={<RotateCcw size={14} />}
          theme='outline'
          onClick={resetAffCode}
        >
          {t('重置邀请码')}
        </Button>
      </div>

      <div className='grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.8fr)]'>
        <InvitationCard
          t={t}
          userState={userState}
          renderQuota={renderQuota}
          setOpenTransfer={setOpenTransfer}
          affLink={affLink}
          handleAffLinkClick={handleAffLinkClick}
          complianceConfirmed={complianceConfirmed}
        />

        <Card className='!rounded-2xl shadow-sm border-0'>
          <Space vertical style={{ width: '100%' }}>
            <div className='flex items-center gap-2'>
              <Users size={18} />
              <Typography.Text className='text-lg font-medium'>
                {t('已邀请用户')}
              </Typography.Text>
            </div>
            <Table
              size='small'
              loading={loading}
              columns={columns}
              dataSource={invitedUsers}
              rowKey='id'
              pagination={false}
              empty={t('暂无邀请用户')}
            />
          </Space>
        </Card>
      </div>
    </div>
  );
};

export default InviteRewards;
