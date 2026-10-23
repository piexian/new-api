/*
Copyright (C) 2023-2026 QuantumNous

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

import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Checkbox,
  DatePicker,
  Input,
  Modal,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import dayjs from 'dayjs';
import { Ban, Eye, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import CardTable from '../../components/common/ui/CardTable';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text, Title } = Typography;
const PAGE_SIZE = 10;
const KINDS = ['session', 'cache'];
const defaultDateRange = () => [
  dayjs().subtract(29, 'day').startOf('day').toDate(),
  dayjs().endOf('day').toDate(),
];
const toUnix = (date) =>
  date ? Math.floor(new Date(date).getTime() / 1000) : undefined;

const SessionRiskTab = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [kind, setKind] = useState('session');
  const [dateRange, setDateRange] = useState(defaultDateRange);
  const [recordingEnabled, setRecordingEnabled] = useState(false);
  const [recordingLoading, setRecordingLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [window, setWindow] = useState({ start: 0, end: 0 });
  const [keywordInput, setKeywordInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [selectedItem, setSelectedItem] = useState(null);
  const [selectedUserIds, setSelectedUserIds] = useState([]);
  const [banReason, setBanReason] = useState('');
  const [banDuration, setBanDuration] = useState(0);
  const [banLoading, setBanLoading] = useState(false);

  const kindLabel = (value) =>
    value === 'cache' ? t('Cache Key') : t('Session ID');

  const fetchConfig = useCallback(async () => {
    try {
      const response = await API.get('/api/risk/session/config');
      if (response.data.success) {
        setRecordingEnabled(
          Boolean(response.data.data?.session_recording_enabled),
        );
      }
    } catch (error) {
      showError(error);
    }
  }, []);

  const toggleRecording = async (checked) => {
    setRecordingLoading(true);
    try {
      const response = await API.put('/api/risk/session/config', {
        session_recording_enabled: checked,
      });
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      setRecordingEnabled(checked);
      showSuccess(t('Settings saved'));
    } catch (error) {
      showError(error);
    } finally {
      setRecordingLoading(false);
    }
  };

  const fetchShared = useCallback(
    async (targetPage = page, targetKeyword = keyword, overrides = {}) => {
      const currentKind = overrides.kind ?? kind;
      const [rangeStart, rangeEnd] = overrides.dateRange ?? dateRange;
      const start = toUnix(rangeStart);
      const end = toUnix(rangeEnd);
      setLoading(true);
      try {
        const response = await API.get('/api/risk/session/shared', {
          params: {
            kind: currentKind,
            p: targetPage,
            page_size: PAGE_SIZE,
            keyword: targetKeyword || undefined,
            start_timestamp: start,
            end_timestamp: end,
          },
        });
        if (!response.data.success) {
          showError(response.data.message);
          return;
        }
        const data = response.data.data;
        setItems(data?.items || []);
        setTotal(data?.total || 0);
        setPage(data?.page || targetPage);
        setWindow({ start: data?.start || start, end: data?.end || end });
      } catch (error) {
        showError(error);
      } finally {
        setLoading(false);
      }
    },
    [kind, keyword, page, dateRange],
  );

  useEffect(() => {
    fetchConfig();
    fetchShared(1, '');
  }, []);

  const applySearch = () => {
    const value = keywordInput.trim();
    setKeyword(value);
    fetchShared(1, value);
  };

  const changeKind = (value) => {
    setKind(value);
    fetchShared(1, keyword, { kind: value });
  };

  const changeDateRange = (value) => {
    const next = Array.isArray(value) && value.length === 2 ? value : [];
    setDateRange(next);
    fetchShared(1, keyword, { dateRange: next });
  };

  const openReview = (item) => {
    setSelectedItem(item);
    setSelectedUserIds(item.users.map((user) => user.user_id));
    setBanReason(
      kind === 'session'
        ? t('Administrator confirmed shared session ID abuse')
        : t('Administrator confirmed shared cache key abuse'),
    );
    setBanDuration(0);
  };

  const closeReview = () => {
    if (banLoading) return;
    setSelectedItem(null);
  };

  const toggleUser = (userId, checked) => {
    setSelectedUserIds((current) =>
      checked
        ? [...new Set([...current, userId])]
        : current.filter((id) => id !== userId),
    );
  };

  const confirmBan = async () => {
    if (!selectedItem || !selectedUserIds.length || !banReason.trim()) return;
    setBanLoading(true);
    try {
      const response = await API.post('/api/risk/session/shared/ban-users', {
        kind,
        identifier: selectedItem.identifier,
        start: window.start,
        end: window.end,
        user_ids: selectedUserIds,
        reason: banReason.trim(),
        duration_minutes: Number(banDuration),
      });
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      const result = response.data.data;
      if (result?.failures?.length) {
        Modal.error({
          title: t('Ban completed with failures'),
          content: result.failures.map((failure) => (
            <div key={failure.id}>
              #{failure.id}: {failure.message}
            </div>
          )),
        });
      } else {
        showSuccess(t('Account banned successfully'));
      }
      setSelectedItem(null);
      await fetchShared(page, keyword);
    } catch (error) {
      showError(error);
    } finally {
      setBanLoading(false);
    }
  };

  const labelTags = (values, render) => (
    <Space wrap>
      {(values || []).slice(0, 3).map((value, index) => (
        <Tag key={index}>{render(value)}</Tag>
      ))}
      {(values || []).length > 3 && <Tag>+{values.length - 3}</Tag>}
    </Space>
  );

  const columns = [
    {
      title: kindLabel(kind),
      dataIndex: 'identifier',
      width: 240,
      render: (value, record) => (
        <div className='min-w-0'>
          <Typography.Text
            style={{ maxWidth: 220 }}
            ellipsis={{ showTooltip: true }}
            copyable
          >
            {value}
          </Typography.Text>
          <div className='mt-1'>{labelTags(record.sources, (v) => v)}</div>
        </div>
      ),
    },
    {
      title: t('Related Accounts'),
      dataIndex: 'users',
      width: 240,
      render: (values) =>
        labelTags(values, (user) => `#${user.user_id} ${user.username}`),
    },
    {
      title: t('Related Tokens'),
      dataIndex: 'tokens',
      width: 240,
      render: (values) =>
        labelTags(values, (token) => `#${token.token_id} ${token.token_name}`),
    },
    { title: t('Users'), dataIndex: 'user_count', width: 90 },
    { title: t('Requests'), dataIndex: 'request_count', width: 100 },
    { title: t('Errors'), dataIndex: 'error_count', width: 90 },
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
          onClick={() => openReview(record)}
        >
          {t('Review')}
        </Button>
      ),
    },
  ];

  const mobileColumns = [
    {
      key: 'mobile-summary',
      title: null,
      render: (_, record) => (
        <div className='flex w-full min-w-0 flex-col gap-3'>
          <div className='min-w-0'>
            <Text type='tertiary' size='small'>
              {kindLabel(record.kind)}
            </Text>
            <div className='mt-1 break-all font-mono text-sm'>
              {record.identifier}
            </div>
          </div>
          <div className='flex min-w-0 flex-wrap gap-1.5'>
            {(record.users || []).map((user) => (
              <Tag key={user.user_id}>
                #{user.user_id} {user.username}
              </Tag>
            ))}
          </div>
          <div className='flex min-w-0 items-start justify-between gap-3 text-sm'>
            <Text type='tertiary'>{t('Requests')}</Text>
            <span>
              {record.request_count} ({t('Errors')}: {record.error_count})
            </span>
          </div>
          <div className='flex min-w-0 items-start justify-between gap-3 text-sm'>
            <Text type='tertiary'>{t('Last Seen')}</Text>
            <span>{timestamp2string(record.last_seen_at)}</span>
          </div>
          <Button
            block
            icon={<Eye size={16} />}
            onClick={() => openReview(record)}
          >
            {t('Review')}
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <Card className='min-w-0' bodyStyle={{ padding: 16 }}>
        <Space wrap>
          <Text strong>{t('Record Session Identifiers')}</Text>
          <Switch
            aria-label={t('Record Session Identifiers')}
            checked={recordingEnabled}
            loading={recordingLoading}
            onChange={toggleRecording}
          />
          <Text type='tertiary'>
            {recordingEnabled
              ? t('Recording enabled')
              : t('Recording disabled')}
          </Text>
        </Space>
        <div className='mt-2 text-semi-color-text-2'>
          {t(
            'When enabled, session IDs and cache affinity identifiers from original client requests are recorded in logs for shared-session risk analysis. Existing logs are not backfilled.',
          )}
        </div>
      </Card>

      <Card className='min-w-0' bodyStyle={{ padding: 16 }}>
        <div className='flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center'>
          <Select
            value={kind}
            onChange={changeKind}
            style={{ width: isMobile ? '100%' : 160 }}
            optionList={KINDS.map((value) => ({
              value,
              label: kindLabel(value),
            }))}
          />
          <DatePicker
            type='dateTimeRange'
            value={dateRange}
            onChange={changeDateRange}
            placeholder={[t('开始时间'), t('结束时间')]}
            showClear
            style={{ width: isMobile ? '100%' : 360 }}
            presets={DATE_RANGE_PRESETS.map((preset) => ({
              text: t(preset.text),
              start: preset.start(),
              end: preset.end(),
            }))}
          />
          <Input
            className='min-w-0 flex-1 sm:max-w-[360px]'
            prefix={<Search size={16} />}
            value={keywordInput}
            onChange={setKeywordInput}
            onEnterPress={applySearch}
            placeholder={t('Search identifier, account, token, or IP')}
            style={{ width: '100%' }}
          />
          <Button
            type='primary'
            className='w-full sm:w-auto'
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
          rowKey='identifier'
          scroll={{ x: 'max-content' }}
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onChange: (nextPage) => fetchShared(nextPage, keyword),
          }}
        />
      </Card>

      <Modal
        title={t('Shared Identifier Review')}
        visible={selectedItem !== null}
        onCancel={closeReview}
        footer={
          <div className='flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
            <Button
              block={isMobile}
              disabled={banLoading}
              onClick={closeReview}
            >
              {t('Cancel')}
            </Button>
            <Button
              block={isMobile}
              type='danger'
              icon={<Ban size={16} />}
              loading={banLoading}
              disabled={!selectedUserIds.length || !banReason.trim()}
              onClick={confirmBan}
            >
              {t('Ban Selected Users', { count: selectedUserIds.length })}
            </Button>
          </div>
        }
        width={920}
        fullScreen={isMobile}
        footerFill={isMobile}
        style={isMobile ? undefined : { maxWidth: 'calc(100vw - 24px)' }}
        bodyStyle={{
          overflowX: 'hidden',
          overflowY: isMobile ? 'auto' : undefined,
          padding: isMobile ? '12px 16px' : undefined,
        }}
      >
        {selectedItem ? (
          <div className='flex min-w-0 flex-col gap-5 overflow-x-hidden pr-1 sm:max-h-[70vh] sm:overflow-y-auto sm:pr-2'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <Tag color='red'>
                {t('Users')}: {selectedItem.user_count}
              </Tag>
              <Text copyable className='break-all'>
                {selectedItem.identifier}
              </Text>
            </div>

            <div>
              <Title heading={5} className='mb-3'>
                {t('Related Accounts')}
              </Title>
              <div className='flex flex-col gap-2'>
                {selectedItem.users.map((user) => (
                  <div
                    key={user.user_id}
                    className='flex min-w-0 items-center gap-3 rounded-md border border-solid p-3'
                    style={{ borderColor: 'var(--semi-color-border)' }}
                  >
                    <Checkbox
                      checked={selectedUserIds.includes(user.user_id)}
                      onChange={(event) =>
                        toggleUser(user.user_id, event.target.checked)
                      }
                    />
                    <Text
                      strong
                      className='flex-1'
                      style={{ wordBreak: 'break-all' }}
                    >
                      #{user.user_id} {user.username}
                    </Text>
                    <Text type='tertiary'>
                      {t('Requests')}: {user.request_count}
                    </Text>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <Title heading={5} className='mb-3'>
                {t('Related Tokens')}
              </Title>
              <Space wrap>
                {(selectedItem.tokens || []).map((token) => (
                  <Tag key={token.token_id}>
                    #{token.token_id} {token.token_name} ({token.username})
                  </Tag>
                ))}
              </Space>
            </div>

            <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
              <div className='min-w-0'>
                <Text type='tertiary' size='small'>
                  {t('IP Addresses')}
                </Text>
                <div
                  style={{ wordBreak: 'break-all', fontFamily: 'monospace' }}
                >
                  {(selectedItem.ips || []).join(', ') || '-'}
                </div>
              </div>
              <div className='min-w-0'>
                <Text type='tertiary' size='small'>
                  {t('Observed At')}
                </Text>
                <div>
                  {timestamp2string(selectedItem.first_seen_at)} -{' '}
                  {timestamp2string(selectedItem.last_seen_at)}
                </div>
              </div>
            </div>

            <div className='flex min-w-0 flex-col gap-1.5'>
              <Text type='tertiary'>{t('Ban Reason')}</Text>
              <Input
                value={banReason}
                maxLength={5000}
                onChange={setBanReason}
              />
            </div>
            <div className='flex min-w-0 flex-col gap-1.5'>
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
        ) : null}
      </Modal>
    </div>
  );
};

export default SessionRiskTab;
