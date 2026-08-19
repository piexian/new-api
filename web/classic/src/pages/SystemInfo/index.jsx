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
import React, { useEffect, useState, useCallback } from 'react';
import {
  Banner,
  Button,
  Card,
  Descriptions,
  Popconfirm,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  RefreshCw,
  ServerCog,
  ListChecks,
  Trash2,
} from 'lucide-react';
import { API, showError, showSuccess, isRoot } from '../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

// 格式化字节
function formatBytes(bytes) {
  if (typeof bytes !== 'number' || isNaN(bytes)) return '-';
  if (bytes === 0) return '0 B';
  if (bytes < 0) return '-' + formatBytes(-bytes);
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatPercent(value) {
  if (typeof value !== 'number' || isNaN(value)) return '-';
  return `${value.toFixed(1)}%`;
}

function formatTimestamp(ts) {
  if (!ts) return '-';
  return new Date(ts * 1000).toLocaleString();
}

function formatRelative(ts) {
  if (!ts) return '-';
  const diff = Math.floor(Date.now() / 1000 - ts);
  if (diff < 60) return `${diff}s 前`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m 前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h 前`;
  return `${Math.floor(diff / 86400)}d 前`;
}

// 实例面板
function SystemInstancesPanel() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [instances, setInstances] = useState([]);
  const [deleting, setDeleting] = useState(null);

  const fetchInstances = useCallback(async () => {
    setRefreshing(true);
    try {
      const res = await API.get('/api/system-info/instances');
      if (res.data.success) {
        setInstances(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('获取系统实例失败'));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [t]);

  useEffect(() => {
    fetchInstances();
    const timer = setInterval(fetchInstances, 30000);
    return () => clearInterval(timer);
  }, [fetchInstances]);

  const staleInstances = instances.filter((i) => i.status === 'stale');

  const deleteStaleInstance = async (nodeName) => {
    setDeleting(nodeName);
    try {
      const res = await API.delete(
        `/api/system-info/instances/${encodeURIComponent(nodeName)}`,
      );
      if (res.data.success) {
        showSuccess(t('已删除过期实例'));
        fetchInstances();
      } else {
        showError(res.data.message || t('删除失败'));
      }
    } catch {
      showError(t('删除失败'));
    } finally {
      setDeleting(null);
    }
  };

  const deleteAllStale = async () => {
    try {
      const res = await API.delete('/api/system-info/stale-instances');
      if (res.data.success) {
        showSuccess(
          t('已删除 {{count}} 个过期实例', {
            count: res.data.data?.deleted_count || 0,
          }),
        );
        fetchInstances();
      } else {
        showError(res.data.message || t('删除失败'));
      }
    } catch {
      showError(t('删除失败'));
    }
  };

  const columns = [
    {
      title: t('实例'),
      dataIndex: 'node_name',
      key: 'node_name',
      render: (value, record) => {
        const name = record.info?.node?.name || value;
        const hostname = record.info?.host?.hostname || '-';
        return (
          <div>
            <Text strong>{name}</Text>
            <br />
            <Text type='tertiary' size='small'>
              {hostname}
            </Text>
          </div>
        );
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const color = status === 'online' ? 'green' : 'orange';
        return <Tag color={color}>{t(status)}</Tag>;
      },
    },
    {
      title: t('角色'),
      key: 'role',
      render: (_, record) =>
        record.info?.role?.is_master ? t('主节点') : t('工作节点'),
    },
    {
      title: 'CPU',
      key: 'cpu',
      render: (_, record) =>
        formatPercent(record.info?.resources?.cpu?.usage_percent),
    },
    {
      title: t('内存'),
      key: 'memory',
      render: (_, record) =>
        formatPercent(record.info?.resources?.memory?.usage_percent),
    },
    {
      title: t('磁盘'),
      key: 'storage',
      render: (_, record) => {
        const storage = record.info?.resources?.storage;
        if (!storage) return '-';
        return `${formatPercent(storage.used_percent)} (${formatBytes(storage.used_bytes)}/${formatBytes(storage.total_bytes)})`;
      },
    },
    {
      title: t('版本'),
      key: 'version',
      render: (_, record) =>
        record.info?.runtime?.version || '-',
    },
    {
      title: t('运行环境'),
      key: 'runtime',
      render: (_, record) => {
        const rt = record.info?.runtime;
        if (!rt?.goos && !rt?.goarch) return '-';
        return [rt.goos, rt.goarch].filter(Boolean).join('/');
      },
    },
    {
      title: t('启动时间'),
      dataIndex: 'started_at',
      key: 'started_at',
      render: (ts) => formatTimestamp(ts),
    },
    {
      title: t('最后心跳'),
      dataIndex: 'last_seen_at',
      key: 'last_seen_at',
      render: (ts) => (
        <span title={formatTimestamp(ts)}>{formatRelative(ts)}</span>
      ),
    },
    {
      title: t('操作'),
      key: 'action',
      render: (_, record) =>
        record.status === 'stale' ? (
          <Popconfirm
            title={t('确认删除过期实例？')}
            content={t('如果该实例已恢复在线，则不会被删除')}
            onConfirm={() => deleteStaleInstance(record.node_name)}
          >
            <Button
              type='danger'
              size='small'
              icon={<Trash2 size={14} />}
              loading={deleting === record.node_name}
            >
              {t('删除')}
            </Button>
          </Popconfirm>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <Card
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ServerCog size={18} />
          <span>{t('系统实例')}</span>
        </div>
      }
      extra={
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          {staleInstances.length > 0 && (
            <Popconfirm
              title={t('确认删除所有过期实例？')}
              content={t('在线实例不会被删除')}
              onConfirm={deleteAllStale}
            >
              <Button type='danger' size='small' icon={<Trash2 size={14} />}>
                {t('删除所有过期')}
              </Button>
            </Popconfirm>
          )}
          <Button
            size='small'
            icon={<RefreshCw size={14} className={refreshing ? 'spin' : ''} />}
            onClick={fetchInstances}
            loading={refreshing}
          >
            {t('刷新')}
          </Button>
          <Text type='tertiary' size='small'>
            {t('每 30 秒自动刷新')}
          </Text>
        </div>
      }
    >
      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={instances}
          rowKey='node_name'
          pagination={false}
          empty={t('暂无实例上报')}
        />
      </Spin>
    </Card>
  );
}

// 任务面板
function SystemTasksPanel() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [tasks, setTasks] = useState([]);

  const fetchTasks = useCallback(async () => {
    setRefreshing(true);
    try {
      const res = await API.get('/api/system-task/list?limit=20');
      if (res.data.success) {
        setTasks(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('获取系统任务失败'));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [t]);

  useEffect(() => {
    fetchTasks();
    const hasActive = tasks.some(
      (task) => task.status === 'pending' || task.status === 'running',
    );
    const interval = hasActive ? 8000 : 30000;
    const timer = setInterval(fetchTasks, interval);
    return () => clearInterval(timer);
  }, [fetchTasks, tasks]);

  const typeLabels = {
    log_cleanup: t('日志清理'),
    channel_test: t('批量渠道测试'),
    model_update: t('批量上游模型更新'),
    midjourney_poll: t('绘图任务轮询'),
    async_task_poll: t('异步任务轮询'),
  };

  const statusColors = {
    pending: 'orange',
    running: 'blue',
    succeeded: 'green',
    failed: 'red',
  };

  const columns = [
    {
      title: t('类型'),
      dataIndex: 'type',
      key: 'type',
      render: (type) => (
        <div>
          <Text strong>{typeLabels[type] || type}</Text>
          <br />
          <Text type='tertiary' size='small'>
            {type}
          </Text>
        </div>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={statusColors[status] || 'grey'}>{t(status)}</Tag>
      ),
    },
    {
      title: t('进度'),
      key: 'progress',
      render: (_, record) => {
        const progress = record.state?.progress;
        if (typeof progress !== 'number' || isNaN(progress)) return '-';
        return `${Math.min(100, Math.max(0, progress))}%`;
      },
    },
    {
      title: t('执行者'),
      dataIndex: 'locked_by',
      key: 'locked_by',
      render: (v) => v || '-',
    },
    {
      title: t('更新时间'),
      dataIndex: 'updated_at',
      key: 'updated_at',
      render: (ts) => (
        <span title={formatTimestamp(ts)}>{formatRelative(ts)}</span>
      ),
    },
    {
      title: t('详情'),
      dataIndex: 'error',
      key: 'error',
      render: (err) => (err ? <Text type='danger'>{err}</Text> : '-'),
    },
  ];

  const activeTasks = tasks.filter(
    (task) => task.status === 'pending' || task.status === 'running',
  );
  const historyTasks = tasks.filter(
    (task) => task.status !== 'pending' && task.status !== 'running',
  );

  return (
    <Card
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ListChecks size={18} />
          <span>{t('系统任务')}</span>
        </div>
      }
      extra={
        <Button
          size='small'
          icon={<RefreshCw size={14} className={refreshing ? 'spin' : ''} />}
          onClick={fetchTasks}
          loading={refreshing}
        >
          {t('刷新')}
        </Button>
      }
    >
      <Spin spinning={loading}>
        <div style={{ marginBottom: 16 }}>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>
            {t('活跃任务')} ({activeTasks.length})
          </Text>
          <Table
            columns={columns}
            dataSource={activeTasks}
            rowKey='task_id'
            pagination={false}
            empty={t('暂无活跃任务')}
          />
        </div>
        <div>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>
            {t('历史任务')} ({historyTasks.length})
          </Text>
          <Table
            columns={columns}
            dataSource={historyTasks}
            rowKey='task_id'
            pagination={false}
            empty={t('暂无历史任务')}
          />
        </div>
      </Spin>
    </Card>
  );
}

export default function SystemInfo() {
  const { t } = useTranslation();

  if (!isRoot()) {
    return (
      <Banner
        type='danger'
        description={t('仅超级管理员可访问此页面')}
      />
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <SystemInstancesPanel />
      <SystemTasksPanel />
    </div>
  );
}
