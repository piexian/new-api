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
  Radio,
  RadioGroup,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';

const { Title, Text } = Typography;
const PAGE_SIZE = 10;

const RiskStatesTab = () => {
  const { t } = useTranslation();
  const [source, setSource] = useState('probe-guard');
  const [dimension, setDimension] = useState('ip');
  const [keyword, setKeyword] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [items, setItems] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);

  const fetchStates = useCallback(async () => {
    setLoading(true);
    try {
      const endpoint =
        source === 'probe-guard'
          ? `/api/risk/probe-guard/${dimension === 'ip' ? 'ip-offenses' : 'user-offenses'}`
          : `/api/risk/error-ban/${dimension === 'ip' ? 'ip-states' : 'user-states'}`;
      const res = await API.get(endpoint, {
        params: {
          p: page,
          page_size: PAGE_SIZE,
          keyword: searchKeyword || undefined,
        },
      });
      if (res.data.success) {
        setItems(res.data.data.items || []);
        setTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setLoading(false);
    }
  }, [source, dimension, page, searchKeyword]);

  const fetchStats = useCallback(async () => {
    try {
      const res = await API.get(`/api/risk/${source}/stats`);
      if (res.data.success) {
        setStats(res.data.data);
      }
    } catch (err) {
      showError(err);
    }
  }, [source]);

  useEffect(() => {
    fetchStates();
    fetchStats();
  }, [fetchStates, fetchStats]);

  const handleModeChange = (nextSource, nextDimension) => {
    setSource(nextSource);
    setDimension(nextDimension);
    setPage(1);
    setKeyword('');
    setSearchKeyword('');
  };

  const handleAction = useCallback(
    async (record) => {
      try {
        let endpoint;
        if (source === 'probe-guard' && dimension === 'ip') {
          endpoint = `/api/risk/probe-guard/ip-offenses/${record.target_ip}/reset`;
        } else if (source === 'probe-guard') {
          endpoint = `/api/risk/probe-guard/user-offenses/${record.user_id}/unban`;
        } else if (dimension === 'ip') {
          endpoint = `/api/risk/error-ban/ip-states/${record.target_ip}/reset`;
        } else {
          endpoint = `/api/risk/error-ban/user-states/${record.user_id}/reset`;
        }
        const res = await API.post(endpoint);
        if (res.data.success) {
          showSuccess(
            source === 'probe-guard' && dimension === 'user'
              ? t('解封成功')
              : t('重置成功'),
          );
          fetchStates();
          fetchStats();
        } else {
          showError(res.data.message);
        }
      } catch (err) {
        showError(err);
      }
    },
    [source, dimension, fetchStates, fetchStats, t],
  );

  const columns = useMemo(() => {
    const actionColumn = {
      title: t('操作'),
      key: 'action',
      render: (_, record) => (
        <Button
          size='small'
          type={
            source === 'probe-guard' && dimension === 'user'
              ? 'primary'
              : 'danger'
          }
          onClick={() => handleAction(record)}
        >
          {source === 'probe-guard' && dimension === 'user'
            ? t('解封')
            : t('重置')}
        </Button>
      ),
    };

    if (source === 'probe-guard') {
      return [
        {
          title: dimension === 'ip' ? t('IP地址') : t('用户ID'),
          dataIndex: dimension === 'ip' ? 'target_ip' : 'user_id',
          key: 'target',
        },
        {
          title: dimension === 'ip' ? t('最近用户ID') : t('最近IP'),
          dataIndex: dimension === 'ip' ? 'last_user_id' : 'last_ip',
          key: 'context',
        },
        {
          title: t('违规次数'),
          dataIndex: 'offense_count',
          key: 'offense_count',
        },
        { title: t('最近模型'), dataIndex: 'last_models', key: 'last_models' },
        {
          title: t('最近违规'),
          dataIndex: 'last_offense_at',
          key: 'last_offense_at',
          render: (value) => (value ? timestamp2string(value) : '-'),
        },
        actionColumn,
      ];
    }

    return [
      {
        title: dimension === 'ip' ? t('IP地址') : t('用户ID'),
        dataIndex: dimension === 'ip' ? 'target_ip' : 'user_id',
        key: 'target',
      },
      { title: t('规则ID'), dataIndex: 'rule_id', key: 'rule_id' },
      {
        title: t('违规次数'),
        dataIndex: 'offense_count',
        key: 'offense_count',
      },
      {
        title: t('窗口内次数'),
        dataIndex: 'window_count',
        key: 'window_count',
      },
      {
        title: t('窗口开始时间'),
        dataIndex: 'window_start',
        key: 'window_start',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      { title: t('最近错误'), dataIndex: 'last_error', key: 'last_error' },
      {
        title: t('最近违规'),
        dataIndex: 'last_offense_at',
        key: 'last_offense_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      actionColumn,
    ];
  }, [source, dimension, handleAction, t]);

  const fourthStat =
    source === 'probe-guard'
      ? { label: t('近期违规'), value: stats?.recent_offenses }
      : { label: t('活跃规则数'), value: stats?.active_rules };

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <div className='flex flex-col gap-4'>
          <div className='flex flex-wrap items-end justify-between gap-3'>
            <Space wrap>
              <div>
                <Text type='secondary'>{t('来源')}</Text>
                <div className='mt-2'>
                  <RadioGroup
                    type='button'
                    value={source}
                    onChange={(event) =>
                      handleModeChange(event.target.value, dimension)
                    }
                  >
                    <Radio value='probe-guard'>{t('探针防护')}</Radio>
                    <Radio value='error-ban'>{t('错误封禁')}</Radio>
                  </RadioGroup>
                </div>
              </div>
              <div>
                <Text type='secondary'>{t('维度')}</Text>
                <div className='mt-2'>
                  <RadioGroup
                    type='button'
                    value={dimension}
                    onChange={(event) =>
                      handleModeChange(source, event.target.value)
                    }
                  >
                    <Radio value='ip'>IP</Radio>
                    <Radio value='user'>{t('用户')}</Radio>
                  </RadioGroup>
                </div>
              </div>
            </Space>
            <Input
              prefix={<Search size={16} />}
              value={keyword}
              onChange={setKeyword}
              onEnterPress={() => {
                setPage(1);
                setSearchKeyword(keyword.trim());
              }}
              showClear
              placeholder={t('搜索')}
              style={{ width: 280 }}
            />
          </div>

          {stats && (
            <div className='flex flex-wrap gap-4'>
              <Text>
                {t('IP状态数')}: {stats.total_ip_states}
              </Text>
              <Text>
                {t('用户状态数')}: {stats.total_user_states}
              </Text>
              <Text>
                {t('总违规次数')}: {stats.total_offenses}
              </Text>
              <Text>
                {fourthStat.label}: {fourthStat.value}
              </Text>
            </div>
          )}
        </div>
      </Card>

      <Card>
        <Title heading={5} className='mb-4'>
          {t('状态记录')}
        </Title>
        <CardTable
          columns={columns}
          dataSource={items}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onChange: setPage,
          }}
        />
      </Card>
    </div>
  );
};

export default RiskStatesTab;
