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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Input,
  InputNumber,
  Select,
  Table,
  Tabs,
  TabPane,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  API,
  createCardProPagination,
  renderQuota,
  showError,
  timestamp2string,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Title, Text } = Typography;

// 枚举值 -> 展示文案（Chinese key，由经典主题 i18n 翻译）
const LOAN_TYPE_LABELS = {
  borrow: '借款',
  repay: '还款',
};
const LOAN_SOURCE_LABELS = {
  manual: '手动',
  checkin: '签到',
  ai: 'AI 业务员',
};
const LOAN_TOPIC_LABELS = {
  credit: '提额',
  rate: '降息',
  grace: '宽限',
  other: '其他',
};
const LOAN_STATUS_LABELS = {
  open: '进行中',
  closed: '已结案',
};

const LOAN_TYPE_TAG_COLORS = { borrow: 'orange', repay: 'green' };
const LOAN_TOPIC_TAG_COLORS = {
  credit: 'blue',
  rate: 'green',
  grace: 'orange',
};
const LOAN_STATUS_TAG_COLORS = { open: 'blue', closed: 'grey' };

// 日利率展示为百分比（与用户端一致），最多两位小数
const formatDailyRate = (rate) => {
  if (typeof rate !== 'number' || Number.isNaN(rate)) return '-';
  return Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 2,
  }).format(rate);
};

// interest_free_until / last_settled_day 为服务器本地日序号（unix/86400），按本地时间展示日期
const formatLoanDay = (dayNumber) => {
  const d = new Date(dayNumber * 86400 * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

// 通用分页数据加载 hook：url 必填，buildParams 接收过滤条件并返回查询参数
const useLoanAdminData = ({ url, buildParams, initialFilters }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);

  const load = async (page, size, filters) => {
    setLoading(true);
    try {
      const res = await API.get(url, {
        params: { ...buildParams(filters), p: page, page_size: size },
      });
      const { success, message, data } = res.data;
      if (success) {
        setItems(data.items || []);
        setTotal(data.total || 0);
        setActivePage(data.page != null ? data.page : page);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  useEffect(() => {
    load(1, ITEMS_PER_PAGE, initialFilters || {});
  }, []);

  return {
    items,
    total,
    loading,
    load,
    activePage,
    pageSize,
    setPageSize,
    isMobile,
    t,
  };
};

// 账户列表：keyword 模糊匹配用户名/用户ID
const LoanAccountsTab = () => {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState('');
  const {
    items,
    total,
    loading,
    load,
    activePage,
    pageSize,
    setPageSize,
    isMobile,
  } = useLoanAdminData({
    url: '/api/user/loan/admin/accounts',
    buildParams: (filters) => {
      const params = {};
      if (filters.keyword) params.keyword = filters.keyword;
      return params;
    },
  });

  const columns = useMemo(
    () => [
      { title: t('用户ID'), dataIndex: 'user_id', width: 90 },
      { title: t('用户名'), dataIndex: 'username', width: 150 },
      {
        title: t('未还本金'),
        dataIndex: 'principal_quota',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('债务总额'),
        dataIndex: 'debt_quota',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('实时债务'),
        dataIndex: 'debt_now',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('当前利息'),
        dataIndex: 'interest_now',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('个人总额上限'),
        dataIndex: 'custom_max_total',
        render: (v) => (v ? renderQuota(v) : t('默认')),
        width: 130,
      },
      {
        title: t('个人日利率'),
        dataIndex: 'custom_daily_rate',
        render: (v) => (v ? formatDailyRate(v) : t('默认')),
        width: 130,
      },
      {
        title: t('免息期'),
        dataIndex: 'interest_free_until',
        render: (v) => (v ? formatLoanDay(v) : t('无')),
        width: 130,
      },
      {
        title: t('条款同意时间'),
        dataIndex: 'terms_agreed_at',
        render: (v) => (v ? timestamp2string(v) : t('未同意')),
        width: 170,
      },
      {
        title: t('累计借款'),
        dataIndex: 'total_borrowed',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('累计还款'),
        dataIndex: 'total_repaid',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('上次结算日'),
        dataIndex: 'last_settled_day',
        render: (v) => (v ? formatLoanDay(v) : '-'),
        width: 130,
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        render: (v) => timestamp2string(v),
        width: 170,
      },
      {
        title: t('更新时间'),
        dataIndex: 'updated_at',
        render: (v) => timestamp2string(v),
        width: 170,
      },
    ],
    [t],
  );

  const pagination = createCardProPagination({
    currentPage: activePage,
    pageSize,
    total,
    onPageChange: (page) => load(page, pageSize, { keyword }),
    onPageSizeChange: (size) => {
      localStorage.setItem('page-size', String(size));
      setPageSize(size);
      load(1, size, { keyword });
    },
    isMobile,
    t,
  });

  return (
    <Card className='!rounded-2xl'>
      <div className='flex flex-col md:flex-row gap-2 mb-4'>
        <Input
          value={keyword}
          placeholder={t('搜索用户名或用户ID')}
          onChange={(v) => setKeyword(v)}
          onEnter={() => load(1, pageSize, { keyword })}
          style={{ maxWidth: 320 }}
        />
        <Button
          theme='solid'
          type='primary'
          onClick={() => load(1, pageSize, { keyword })}
        >
          {t('搜索')}
        </Button>
      </div>
      <Table
        size='small'
        columns={columns}
        dataSource={items}
        rowKey='user_id'
        loading={loading}
        empty={t('暂无数据')}
        scroll={{ x: 'max-content' }}
      />
      <div className='mt-4'>{pagination}</div>
    </Card>
  );
};

// 台账记录：可按 user_id 过滤
const LoanRecordsTab = () => {
  const { t } = useTranslation();
  const [userId, setUserId] = useState(null);
  const {
    items,
    total,
    loading,
    load,
    activePage,
    pageSize,
    setPageSize,
    isMobile,
  } = useLoanAdminData({
    url: '/api/user/loan/admin/records',
    buildParams: (filters) => {
      const params = {};
      if (filters.userId != null && filters.userId !== '') {
        params.user_id = String(filters.userId);
      }
      return params;
    },
  });

  const columns = useMemo(
    () => [
      { title: t('编号'), dataIndex: 'id', width: 80 },
      { title: t('用户ID'), dataIndex: 'user_id', width: 90 },
      { title: t('用户名'), dataIndex: 'username', width: 150 },
      {
        title: t('类型'),
        dataIndex: 'type',
        render: (v) => (
          <Tag color={LOAN_TYPE_TAG_COLORS[v] || 'grey'} size='small'>
            {t(LOAN_TYPE_LABELS[v] || v)}
          </Tag>
        ),
        width: 90,
      },
      {
        title: t('金额'),
        dataIndex: 'amount',
        render: (v) => renderQuota(v || 0),
        width: 120,
      },
      {
        title: t('利息部分'),
        dataIndex: 'interest_part',
        render: (v) => renderQuota(v || 0),
        width: 120,
      },
      {
        title: t('本金部分'),
        dataIndex: 'principal_part',
        render: (v) => renderQuota(v || 0),
        width: 120,
      },
      {
        title: t('手续费'),
        dataIndex: 'fee_part',
        render: (v) => renderQuota(v || 0),
        width: 120,
      },
      {
        title: t('欠款余额'),
        dataIndex: 'debt_after',
        render: (v) => renderQuota(v || 0),
        width: 120,
      },
      {
        title: t('来源'),
        dataIndex: 'source',
        render: (v) => t(LOAN_SOURCE_LABELS[v] || v),
        width: 100,
      },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (v) => timestamp2string(v),
        width: 170,
      },
    ],
    [t],
  );

  const pagination = createCardProPagination({
    currentPage: activePage,
    pageSize,
    total,
    onPageChange: (page) => load(page, pageSize, { userId }),
    onPageSizeChange: (size) => {
      localStorage.setItem('page-size', String(size));
      setPageSize(size);
      load(1, size, { userId });
    },
    isMobile,
    t,
  });

  return (
    <Card className='!rounded-2xl'>
      <div className='flex flex-col md:flex-row gap-2 mb-4'>
        <InputNumber
          value={userId}
          placeholder={t('输入用户ID')}
          onChange={(v) => setUserId(v)}
          style={{ maxWidth: 200 }}
        />
        <Button
          theme='solid'
          type='primary'
          onClick={() => load(1, pageSize, { userId })}
        >
          {t('搜索')}
        </Button>
      </div>
      <Table
        size='small'
        columns={columns}
        dataSource={items}
        rowKey='id'
        loading={loading}
        empty={t('暂无数据')}
        scroll={{ x: 'max-content' }}
      />
      <div className='mt-4'>{pagination}</div>
    </Card>
  );
};

// 业务员工单：可按 user_id / status 过滤
const LoanApplicationsTab = () => {
  const { t } = useTranslation();
  const [userId, setUserId] = useState(null);
  const [status, setStatus] = useState('');
  const {
    items,
    total,
    loading,
    load,
    activePage,
    pageSize,
    setPageSize,
    isMobile,
  } = useLoanAdminData({
    url: '/api/user/loan/admin/applications',
    buildParams: (filters) => {
      const params = {};
      if (filters.userId != null && filters.userId !== '') {
        params.user_id = String(filters.userId);
      }
      if (filters.status) params.status = filters.status;
      return params;
    },
  });

  const columns = useMemo(
    () => [
      { title: t('工单'), dataIndex: 'id', width: 90 },
      { title: t('用户ID'), dataIndex: 'user_id', width: 90 },
      { title: t('用户名'), dataIndex: 'username', width: 150 },
      {
        title: t('主题'),
        dataIndex: 'topic',
        render: (v) => (
          <Tag color={LOAN_TOPIC_TAG_COLORS[v] || 'grey'} size='small'>
            {t(LOAN_TOPIC_LABELS[v] || v)}
          </Tag>
        ),
        width: 100,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (v) => (
          <Tag color={LOAN_STATUS_TAG_COLORS[v] || 'grey'} size='small'>
            {t(LOAN_STATUS_LABELS[v] || v)}
          </Tag>
        ),
        width: 100,
      },
      {
        title: t('评分'),
        dataIndex: 'rating',
        render: (v) => (v ? v : t('未评分')),
        width: 90,
      },
      {
        title: t('评分备注'),
        dataIndex: 'rating_comment',
        render: (v) => v || '-',
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        render: (v) => timestamp2string(v),
        width: 170,
      },
      {
        title: t('更新时间'),
        dataIndex: 'updated_at',
        render: (v) => timestamp2string(v),
        width: 170,
      },
    ],
    [t],
  );

  const pagination = createCardProPagination({
    currentPage: activePage,
    pageSize,
    total,
    onPageChange: (page) => load(page, pageSize, { userId, status }),
    onPageSizeChange: (size) => {
      localStorage.setItem('page-size', String(size));
      setPageSize(size);
      load(1, size, { userId, status });
    },
    isMobile,
    t,
  });

  return (
    <Card className='!rounded-2xl'>
      <div className='flex flex-col md:flex-row gap-2 mb-4'>
        <InputNumber
          value={userId}
          placeholder={t('输入用户ID')}
          onChange={(v) => setUserId(v)}
          style={{ maxWidth: 200 }}
        />
        <Select
          value={status}
          onChange={(v) => setStatus(v)}
          placeholder={t('全部')}
          optionList={[
            { value: '', label: t('全部') },
            { value: 'open', label: t('进行中') },
            { value: 'closed', label: t('已结案') },
          ]}
          style={{ width: 140 }}
        />
        <Button
          theme='solid'
          type='primary'
          onClick={() => load(1, pageSize, { userId, status })}
        >
          {t('搜索')}
        </Button>
      </div>
      <Table
        size='small'
        columns={columns}
        dataSource={items}
        rowKey='id'
        loading={loading}
        empty={t('暂无数据')}
        scroll={{ x: 'max-content' }}
      />
      <div className='mt-4'>{pagination}</div>
    </Card>
  );
};

const AdminLoan = () => {
  const { t } = useTranslation();

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2 pb-6'>
      <div className='mb-6'>
        <Title heading={3} className='!mb-1'>
          {t('词元贷管理')}
        </Title>
        <Text type='secondary'>{t('管理词元贷账户、台账与业务员工单')}</Text>
      </div>

      <Tabs type='line' defaultActiveKey='accounts'>
        <TabPane tab={t('账户列表')} itemKey='accounts'>
          <LoanAccountsTab />
        </TabPane>
        <TabPane tab={t('台账记录')} itemKey='records'>
          <LoanRecordsTab />
        </TabPane>
        <TabPane tab={t('业务员工单')} itemKey='applications'>
          <LoanApplicationsTab />
        </TabPane>
      </Tabs>
    </div>
  );
};

export default AdminLoan;
