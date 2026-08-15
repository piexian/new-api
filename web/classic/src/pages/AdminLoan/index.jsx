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

// 借贷市场枚举值 -> 展示文案（Chinese key，由经典主题 i18n 翻译）
const OFFER_MODE_LABELS = { pool: '资金池', ai: 'AI', order: '订单' };
const OFFER_STATUS_LABELS = {
  active: '生效',
  paused: '已暂停',
  closed: '已关闭',
};
const OFFER_STATUS_TAG_COLORS = {
  active: 'green',
  paused: 'orange',
  closed: 'grey',
};
const FUNDING_STATUS_LABELS = {
  active: '生效',
  overdue: '逾期',
  repaid: '已结清',
  written_off: '已核销',
};
const FUNDING_STATUS_TAG_COLORS = {
  active: 'blue',
  overdue: 'red',
  repaid: 'green',
  written_off: 'grey',
};
const FUNDING_SOURCE_LABELS = {
  platform: '平台',
  pool: '资金池',
  ai: 'AI',
  order: '订单',
};
const REPAY_PLAN_LABELS = {
  full: '全额（复利）',
  no_penalty: '免罚息',
  interest_freeze: '停止计息',
  principal_only: '只还本金',
};

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
        pagination={false}
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
        pagination={false}
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
        pagination={false}
      />
      <div className='mt-4'>{pagination}</div>
    </Card>
  );
};

// 借贷市场总览：冻结闲置 / 在贷本金 / 累计利息 / 逾期笔数 / 在售挂单
const MarketOverviewTab = () => {
  const { t } = useTranslation();
  const [overview, setOverview] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchOverview = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/user/loan/admin/market_overview');
      const { success, message, data } = res.data;
      if (success) {
        setOverview(data);
      } else {
        setError(message || t('加载失败'));
      }
    } catch (err) {
      setError(err.message || t('加载失败'));
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchOverview();
  }, []);

  const stats = overview
    ? [
        {
          label: t('冻结闲置'),
          value: renderQuota(overview.frozen_idle || 0),
        },
        {
          label: t('在贷本金'),
          value: renderQuota(overview.in_loan_principal || 0),
        },
        {
          label: t('累计利息'),
          value: renderQuota(overview.total_interest_earned || 0),
        },
        {
          label: t('逾期笔数'),
          value: (overview.overdue_fundings || 0).toLocaleString(),
        },
        {
          label: t('在售挂单'),
          value: (overview.active_offers || 0).toLocaleString(),
        },
      ]
    : [];

  return (
    <Card className='!rounded-2xl'>
      {loading && !overview ? (
        <div className='grid gap-3 grid-cols-2 md:grid-cols-5'>
          {['a', 'b', 'c', 'd', 'e'].map((slot) => (
            <div
              key={slot}
              className='h-20 bg-gray-100 dark:bg-gray-800 rounded-xl animate-pulse'
            />
          ))}
        </div>
      ) : error && !overview ? (
        <div className='py-8 text-center text-sm text-gray-500'>
          {error}
          <div className='mt-3'>
            <Button
              type='primary'
              theme='solid'
              size='small'
              onClick={fetchOverview}
            >
              {t('重试')}
            </Button>
          </div>
        </div>
      ) : (
        <>
          <div className='grid gap-3 grid-cols-2 md:grid-cols-5'>
            {stats.map((item) => (
              <div
                key={item.label}
                className='rounded-xl border border-gray-200 dark:border-gray-700 p-3'
              >
                <div className='text-xs text-gray-500 dark:text-gray-400'>
                  {item.label}
                </div>
                <div className='mt-1.5 font-mono font-semibold text-base sm:text-lg tabular-nums break-all'>
                  {item.value}
                </div>
              </div>
            ))}
          </div>
          {overview &&
          Object.keys(overview.offers_by_status || {}).length > 0 ? (
            <div className='mt-3 text-sm text-gray-500 dark:text-gray-400'>
              {t('各状态挂单数')}:{' '}
              {Object.entries(overview.offers_by_status)
                .map(
                  ([status, count]) =>
                    `${t(OFFER_STATUS_LABELS[status] || status)}: ${Number(count).toLocaleString()}`,
                )
                .join(' · ')}
            </div>
          ) : null}
        </>
      )}
    </Card>
  );
};

// 放贷挂单列表：keyword 过滤放贷人（纯数字按用户ID，否则用户名模糊匹配）
const LoanOffersTab = () => {
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
    url: '/api/user/loan/admin/offers',
    buildParams: (filters) => {
      const params = {};
      if (filters.keyword) params.keyword = filters.keyword;
      return params;
    },
  });

  const columns = useMemo(
    () => [
      { title: t('编号'), dataIndex: 'id', width: 80 },
      { title: t('放贷人ID'), dataIndex: 'lender_id', width: 100 },
      { title: t('用户名'), dataIndex: 'username', width: 150 },
      {
        title: t('模式'),
        dataIndex: 'mode',
        render: (v) => (
          <Tag color='grey' size='small'>
            {t(OFFER_MODE_LABELS[v] || v)}
          </Tag>
        ),
        width: 90,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (v) => (
          <Tag color={OFFER_STATUS_TAG_COLORS[v] || 'grey'} size='small'>
            {t(OFFER_STATUS_LABELS[v] || v)}
          </Tag>
        ),
        width: 90,
      },
      {
        title: t('挂出总额'),
        dataIndex: 'amount_total',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('可用额度'),
        dataIndex: 'amount_available',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('利率'),
        dataIndex: 'rate_fixed',
        render: (v, record) =>
          v > 0
            ? formatDailyRate(v)
            : `${formatDailyRate(record.rate_min)}-${formatDailyRate(record.rate_max)}`,
        width: 130,
      },
      {
        title: t('单笔上限'),
        dataIndex: 'per_loan_cap',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('信用门槛'),
        dataIndex: 'min_credit_score',
        render: (v) => (v === -50 ? t('不限') : v),
        width: 100,
      },
      {
        title: t('累计放出'),
        dataIndex: 'total_lent',
        render: (v) => renderQuota(v || 0),
        width: 130,
      },
      {
        title: t('累计利息'),
        dataIndex: 'total_interest_earned',
        render: (v) => renderQuota(v || 0),
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
          placeholder={t('搜索放贷人用户名或ID')}
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
        rowKey='id'
        loading={loading}
        empty={t('暂无数据')}
        scroll={{ x: 'max-content' }}
        pagination={false}
      />
      <div className='mt-4'>{pagination}</div>
    </Card>
  );
};

// 投放记录：可按放贷人ID / 借款人ID / 状态过滤
const LoanFundingsTab = () => {
  const { t } = useTranslation();
  const [lenderId, setLenderId] = useState(null);
  const [loanUserId, setLoanUserId] = useState(null);
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
    url: '/api/user/loan/admin/fundings',
    buildParams: (filters) => {
      const params = {};
      if (filters.lenderId != null && filters.lenderId !== '') {
        params.lender_id = String(filters.lenderId);
      }
      if (filters.loanUserId != null && filters.loanUserId !== '') {
        params.loan_user_id = String(filters.loanUserId);
      }
      if (filters.status) params.status = filters.status;
      return params;
    },
  });

  const userCell = (id, username) => (
    <div className='leading-tight'>
      <div className='text-sm'>{username || '-'}</div>
      <div className='text-xs text-gray-400'>#{id}</div>
    </div>
  );

  const columns = useMemo(
    () => [
      { title: t('编号'), dataIndex: 'id', width: 80 },
      {
        title: t('放贷人'),
        dataIndex: 'lender_id',
        render: (v, record) => userCell(v, record.lender_username),
        width: 150,
      },
      {
        title: t('借款人'),
        dataIndex: 'loan_user_id',
        render: (v, record) => userCell(v, record.borrower_username),
        width: 150,
      },
      {
        title: t('来源'),
        dataIndex: 'source_type',
        render: (v) => (
          <Tag color='grey' size='small'>
            {t(FUNDING_SOURCE_LABELS[v] || v)}
          </Tag>
        ),
        width: 100,
      },
      {
        title: t('金额'),
        dataIndex: 'amount',
        render: (v) => renderQuota(v || 0),
        width: 120,
      },
      {
        title: t('未还本金'),
        dataIndex: 'principal_remaining',
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
        title: t('利率'),
        dataIndex: 'rate',
        render: (v) => formatDailyRate(v),
        width: 100,
      },
      {
        title: t('还款计划'),
        dataIndex: 'repay_plan',
        render: (v) => t(REPAY_PLAN_LABELS[v] || v),
        width: 110,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (v) => (
          <Tag color={FUNDING_STATUS_TAG_COLORS[v] || 'grey'} size='small'>
            {t(FUNDING_STATUS_LABELS[v] || v)}
          </Tag>
        ),
        width: 90,
      },
      {
        title: t('应还日期'),
        dataIndex: 'due_day',
        render: (v) => (v ? formatLoanDay(v) : '-'),
        width: 110,
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
    onPageChange: (page) =>
      load(page, pageSize, { lenderId, loanUserId, status }),
    onPageSizeChange: (size) => {
      localStorage.setItem('page-size', String(size));
      setPageSize(size);
      load(1, size, { lenderId, loanUserId, status });
    },
    isMobile,
    t,
  });

  return (
    <Card className='!rounded-2xl'>
      <div className='flex flex-col md:flex-row gap-2 mb-4'>
        <InputNumber
          value={lenderId}
          placeholder={t('输入放贷人ID')}
          onChange={(v) => setLenderId(v)}
          style={{ maxWidth: 180 }}
        />
        <InputNumber
          value={loanUserId}
          placeholder={t('输入借款人ID')}
          onChange={(v) => setLoanUserId(v)}
          style={{ maxWidth: 180 }}
        />
        <Select
          value={status}
          onChange={(v) => setStatus(v)}
          placeholder={t('全部')}
          optionList={[
            { value: '', label: t('全部') },
            { value: 'active', label: t('生效') },
            { value: 'overdue', label: t('逾期') },
            { value: 'repaid', label: t('已结清') },
            { value: 'written_off', label: t('已核销') },
          ]}
          style={{ width: 130 }}
        />
        <Button
          theme='solid'
          type='primary'
          onClick={() => load(1, pageSize, { lenderId, loanUserId, status })}
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
        pagination={false}
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
        <Text type='secondary'>
          {t('管理词元贷账户、台账、业务员工单与借贷市场')}
        </Text>
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
        <TabPane tab={t('市场总览')} itemKey='overview'>
          <MarketOverviewTab />
        </TabPane>
        <TabPane tab={t('放贷挂单')} itemKey='offers'>
          <LoanOffersTab />
        </TabPane>
        <TabPane tab={t('投放记录')} itemKey='fundings'>
          <LoanFundingsTab />
        </TabPane>
      </Tabs>
    </div>
  );
};

export default AdminLoan;
