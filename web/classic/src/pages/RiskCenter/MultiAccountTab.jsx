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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Input,
  Modal,
  Select,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Ban, Eye, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import CardTable from '../../components/common/ui/CardTable';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text, Title } = Typography;
const PAGE_SIZE = 10;

const MultiAccountTab = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [items, setItems] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [keywordInput, setKeywordInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [selectedCluster, setSelectedCluster] = useState(null);
  const [banTarget, setBanTarget] = useState(null);
  const [banReason, setBanReason] = useState('');
  const [banDuration, setBanDuration] = useState(0);
  const [banLoading, setBanLoading] = useState(false);

  const fetchClusters = useCallback(
    async (targetPage = page, targetKeyword = keyword) => {
      setLoading(true);
      try {
        const response = await API.get('/api/risk/multi-account', {
          params: {
            p: targetPage,
            page_size: PAGE_SIZE,
            keyword: targetKeyword || undefined,
          },
        });
        if (!response.data.success) {
          showError(response.data.message);
          return;
        }
        const data = response.data.data;
        setItems(data.items || []);
        setStats(data.stats || null);
        setTotal(data.total || 0);
        setPage(data.page || targetPage);
      } catch (error) {
        showError(error);
      } finally {
        setLoading(false);
      }
    },
    [keyword, page],
  );

  useEffect(() => {
    fetchClusters(1, '');
  }, []);

  const riskLabel = (level) => {
    if (level === 'high') return t('High Risk');
    if (level === 'medium') return t('Medium Risk');
    return t('Low Risk');
  };

  const riskColor = (level) => {
    if (level === 'high') return 'red';
    if (level === 'medium') return 'orange';
    return 'grey';
  };

  const evidenceLabel = (evidence) =>
    evidence.type === 'github_email_conflict'
      ? t('GitHub Email Conflict')
      : t('Shared IP and Browser');

  const accountStatus = (account) => {
    if (account.deleted) return t('Deleted');
    return account.status === 1 ? t('Enabled') : t('Disabled');
  };

  const roleLabel = (role) => {
    if (role === 100) return t('Root User');
    if (role === 10) return t('Administrator');
    return t('Common User');
  };

  const accountDetailFields = (account) => [
    [t('Email'), account.email || '-', 'email'],
    [t('GitHub ID'), account.github_id || '-', 'github'],
    ...(account.oauth_identities || [])
      .filter((identity) => identity.provider_key !== 'github')
      .map((identity) => [
        `${identity.provider_name} ID`,
        identity.provider_user_id,
        identity.provider_key,
      ]),
    [t('Role'), roleLabel(account.role), 'role'],
    [t('Status'), accountStatus(account), 'status'],
    [
      t('Created At'),
      account.created_at ? timestamp2string(account.created_at) : '-',
      'created_at',
    ],
    [
      t('Last Login'),
      account.last_login_at ? timestamp2string(account.last_login_at) : '-',
      'last_login_at',
    ],
    [
      t('Disabled Until'),
      account.disabled_until ? timestamp2string(account.disabled_until) : '-',
      'disabled_until',
    ],
    [t('Ban Reason'), account.disable_reason || '-', 'disable_reason'],
  ];

  const applySearch = () => {
    const value = keywordInput.trim();
    setKeyword(value);
    fetchClusters(1, value);
  };

  const openBan = (account) => {
    setBanTarget(account);
    setBanReason(t('Administrator confirmed multi-account abuse'));
    setBanDuration(0);
  };

  const confirmBan = async () => {
    if (!banTarget || !banReason.trim()) return;
    setBanLoading(true);
    try {
      const response = await API.post(
        `/api/risk/multi-account/users/${banTarget.id}/ban`,
        {
          reason: banReason.trim(),
          duration_minutes: Number(banDuration),
        },
      );
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      const updatedAccount = response.data.data;
      showSuccess(t('Account banned successfully'));
      if (updatedAccount) {
        setSelectedCluster((current) =>
          current
            ? {
                ...current,
                accounts: current.accounts.map((account) =>
                  account.id === updatedAccount.id ? updatedAccount : account,
                ),
              }
            : null,
        );
      }
      setBanTarget(null);
      fetchClusters(page, keyword);
    } catch (error) {
      showError(error);
    } finally {
      setBanLoading(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('Rank'),
        dataIndex: 'rank',
        width: 70,
        render: (value) => <Text strong>#{value}</Text>,
      },
      {
        title: t('Risk Score'),
        width: 120,
        render: (_, record) => (
          <Space vertical align='start' spacing={2}>
            <Title heading={5}>{record.risk_score}</Title>
            <Tag color={riskColor(record.risk_level)}>
              {riskLabel(record.risk_level)}
            </Tag>
          </Space>
        ),
      },
      {
        title: t('Related Accounts'),
        render: (_, record) => (
          <Space
            vertical
            align='end'
            spacing={4}
            className='min-w-0 max-w-full'
            style={{ width: '100%' }}
          >
            {(record.accounts || []).map((account) => (
              <div key={account.id} className='min-w-0 max-w-full text-right'>
                <Text strong style={{ wordBreak: 'break-all' }}>
                  #{account.id} {account.username}
                </Text>
                <br />
                <Text type='tertiary' style={{ wordBreak: 'break-all' }}>
                  {account.email || '-'}
                </Text>
              </div>
            ))}
          </Space>
        ),
      },
      {
        title: t('Evidence'),
        width: 240,
        render: (_, record) => (
          <Space wrap className='max-w-full justify-end'>
            {(record.evidence || []).map((evidence, index) => (
              <Tag key={`${evidence.type}-${evidence.last_seen_at}-${index}`}>
                {evidenceLabel(evidence)} x{evidence.hit_count}
              </Tag>
            ))}
          </Space>
        ),
      },
      {
        title: t('Last Seen'),
        dataIndex: 'last_seen_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('Actions'),
        width: 110,
        render: (_, record) => (
          <Button
            size='small'
            icon={<Eye size={16} />}
            onClick={() => setSelectedCluster(record)}
          >
            {t('Review')}
          </Button>
        ),
      },
    ],
    [t],
  );

  const mobileColumns = useMemo(
    () => [
      {
        key: 'mobile-summary',
        title: null,
        render: (_, record) => (
          <div className='flex w-full min-w-0 flex-col gap-4'>
            <div className='flex min-w-0 items-start justify-between gap-3'>
              <div className='min-w-0'>
                <Text type='tertiary' size='small'>
                  {t('Rank')}
                </Text>
                <div className='mt-1 text-base font-semibold'>
                  #{record.rank}
                </div>
              </div>
              <div className='flex shrink-0 flex-col items-end gap-1'>
                <span className='text-lg font-semibold'>
                  {record.risk_score}
                </span>
                <Tag color={riskColor(record.risk_level)}>
                  {riskLabel(record.risk_level)}
                </Tag>
              </div>
            </div>

            <div className='min-w-0'>
              <Text type='tertiary' size='small'>
                {t('Related Accounts')}
              </Text>
              <div className='mt-2 flex min-w-0 flex-col gap-2'>
                {(record.accounts || []).map((account) => (
                  <div key={account.id} className='min-w-0'>
                    <div className='break-all font-medium'>
                      #{account.id} {account.username}
                    </div>
                    <div className='break-all text-sm text-semi-color-text-2'>
                      {account.email || '-'}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className='min-w-0'>
              <Text type='tertiary' size='small'>
                {t('Evidence')}
              </Text>
              <div className='mt-2 flex min-w-0 flex-wrap gap-1.5'>
                {(record.evidence || []).map((evidence, index) => (
                  <Tag
                    key={`${evidence.type}-${evidence.last_seen_at}-${index}`}
                    className='!h-auto max-w-full !whitespace-normal'
                  >
                    {evidenceLabel(evidence)} x{evidence.hit_count}
                  </Tag>
                ))}
              </div>
            </div>

            <div className='flex min-w-0 items-start justify-between gap-3 text-sm'>
              <Text type='tertiary'>{t('Last Seen')}</Text>
              <span className='min-w-0 text-right break-words'>
                {record.last_seen_at
                  ? timestamp2string(record.last_seen_at)
                  : '-'}
              </span>
            </div>

            <Button
              block
              icon={<Eye size={16} />}
              onClick={() => setSelectedCluster(record)}
            >
              {t('Review')}
            </Button>
          </div>
        ),
      },
    ],
    [t],
  );

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3'>
        {[
          [t('Multi-account Clusters'), stats?.total_clusters],
          [t('High Risk'), stats?.high_risk_clusters],
          [t('Related Accounts'), stats?.related_accounts],
          [t('GitHub Email Conflicts'), stats?.email_conflicts],
          [t('Shared Environments'), stats?.shared_environments],
        ].map(([label, value]) => (
          <Card key={label} className='min-w-0' bodyStyle={{ padding: 16 }}>
            <Text type='tertiary'>{label}</Text>
            <Title heading={3} style={{ marginTop: 6 }}>
              {value ?? '-'}
            </Title>
          </Card>
        ))}
      </div>

      <Card className='min-w-0' bodyStyle={{ padding: 16 }}>
        <div className='flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center'>
          <Input
            className='min-w-0 flex-1 sm:max-w-[360px]'
            prefix={<Search size={16} />}
            value={keywordInput}
            onChange={setKeywordInput}
            onEnterPress={applySearch}
            placeholder={t('Search account, email, IP, or browser')}
            style={{ width: '100%' }}
          />
          <Button
            type='primary'
            className='w-full sm:w-auto'
            icon={<Search size={16} />}
            onClick={applySearch}
          >
            {t('Search')}
          </Button>
        </div>
      </Card>

      <Card className='min-w-0 overflow-hidden'>
        <CardTable
          columns={isMobile ? mobileColumns : columns}
          dataSource={items}
          loading={loading}
          rowKey='id'
          scroll={{ x: 'max-content' }}
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onChange: (nextPage) => fetchClusters(nextPage, keyword),
          }}
        />
      </Card>

      <Modal
        title={t('Multi-account Review')}
        visible={selectedCluster !== null}
        onCancel={() => setSelectedCluster(null)}
        footer={
          <Button onClick={() => setSelectedCluster(null)}>{t('Close')}</Button>
        }
        width={920}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
        bodyStyle={{ overflowX: 'hidden' }}
      >
        {selectedCluster && (
          <div className='flex min-w-0 max-h-[70vh] flex-col gap-5 overflow-x-hidden overflow-y-auto pr-1 sm:pr-2'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <Tag color={riskColor(selectedCluster.risk_level)}>
                {riskLabel(selectedCluster.risk_level)}
              </Tag>
              <Text>
                {t('Risk Score')}: {selectedCluster.risk_score}
              </Text>
              <Text type='tertiary'>ID: {selectedCluster.id}</Text>
            </div>

            <div>
              <Title heading={5} className='mb-3'>
                {t('Account Details')}
              </Title>
              <div className='flex flex-col gap-3'>
                {selectedCluster.accounts.map((account) => (
                  <div
                    key={account.id}
                    className='min-w-0 rounded-md border border-solid p-3 sm:p-4'
                    style={{ borderColor: 'var(--semi-color-border)' }}
                  >
                    <div className='mb-3 flex min-w-0 flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between'>
                      <Text strong style={{ wordBreak: 'break-all' }}>
                        #{account.id} {account.username}
                      </Text>
                      <div className='flex min-w-0 flex-col items-stretch gap-2 sm:flex-row sm:items-center'>
                        <Tag>{accountStatus(account)}</Tag>
                        <Button
                          type='danger'
                          size='small'
                          className='w-full sm:w-auto'
                          icon={<Ban size={16} />}
                          disabled={!account.can_ban}
                          onClick={() => openBan(account)}
                        >
                          {t('Ban Account')}
                        </Button>
                      </div>
                    </div>
                    <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
                      {accountDetailFields(account).map(
                        ([label, value, fieldKey]) => (
                          <div key={fieldKey} className='min-w-0'>
                            <Text type='tertiary' size='small'>
                              {label}
                            </Text>
                            <div style={{ wordBreak: 'break-all' }}>
                              {value}
                            </div>
                          </div>
                        ),
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <Title heading={5} className='mb-3'>
                {t('Evidence Details')}
              </Title>
              <div className='flex flex-col gap-3'>
                {selectedCluster.evidence.map((evidence, index) => (
                  <div
                    key={`${evidence.type}-${evidence.last_seen_at}-${index}`}
                    className='min-w-0 rounded-md border border-solid p-3 sm:p-4'
                    style={{ borderColor: 'var(--semi-color-border)' }}
                  >
                    <div className='mb-3 flex min-w-0 flex-wrap items-center gap-2'>
                      <Tag>{evidenceLabel(evidence)}</Tag>
                      <Text type='tertiary'>
                        {t('Hit Count')}: {evidence.hit_count}
                      </Text>
                    </div>
                    <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
                      {evidence.email && (
                        <div className='min-w-0'>
                          <Text type='tertiary' size='small'>
                            {t('Full Email')}
                          </Text>
                          <div style={{ wordBreak: 'break-all' }}>
                            {evidence.email}
                          </div>
                        </div>
                      )}
                      {evidence.ip && (
                        <div className='min-w-0'>
                          <Text type='tertiary' size='small'>
                            {t('IP Address')}
                          </Text>
                          <div style={{ wordBreak: 'break-all' }}>
                            {evidence.ip}
                          </div>
                        </div>
                      )}
                      {evidence.user_agent && (
                        <div className='min-w-0 md:col-span-2'>
                          <Text type='tertiary' size='small'>
                            {t('Browser / User Agent')}
                          </Text>
                          <div
                            style={{
                              wordBreak: 'break-all',
                              fontFamily: 'monospace',
                            }}
                          >
                            {evidence.user_agent}
                          </div>
                        </div>
                      )}
                      <div className='min-w-0'>
                        <Text type='tertiary' size='small'>
                          {t('Related User IDs')}
                        </Text>
                        <div>{evidence.user_ids.join(', ')}</div>
                      </div>
                      <div className='min-w-0'>
                        <Text type='tertiary' size='small'>
                          {t('Observed At')}
                        </Text>
                        <div>
                          {timestamp2string(evidence.first_seen_at)} -{' '}
                          {timestamp2string(evidence.last_seen_at)}
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        title={t('Confirm Ban')}
        visible={banTarget !== null}
        onCancel={() => setBanTarget(null)}
        onOk={confirmBan}
        confirmLoading={banLoading}
        okButtonProps={{ disabled: !banReason.trim(), type: 'danger' }}
        okText={t('Confirm Ban')}
        cancelText={t('Cancel')}
        width={520}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
        bodyStyle={{ overflowX: 'hidden' }}
      >
        {banTarget && (
          <div className='flex flex-col gap-4'>
            <Text style={{ wordBreak: 'break-word' }}>
              {t('Ban account confirmation', {
                id: banTarget.id,
                username: banTarget.username,
              })}
            </Text>
            <div>
              <Text type='tertiary'>{t('Ban Reason')}</Text>
              <Input
                value={banReason}
                maxLength={5000}
                onChange={setBanReason}
              />
            </div>
            <div>
              <Text type='tertiary'>{t('Ban Duration')}</Text>
              <Select
                value={banDuration}
                onChange={setBanDuration}
                style={{ width: '100%' }}
                optionList={[
                  { value: 0, label: t('Permanent Ban') },
                  { value: 1440, label: t('1 Day') },
                  { value: 10080, label: t('7 Days') },
                  { value: 43200, label: t('30 Days') },
                ]}
              />
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default MultiAccountTab;
