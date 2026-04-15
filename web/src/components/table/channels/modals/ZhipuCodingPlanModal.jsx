import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  Button,
  Collapse,
  InputNumber,
  Modal,
  Progress,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const { Text } = Typography;

const ZHIPU_CODING_PLAN_BASE_URL = 'glm-coding-plan';
const ZHIPU_CODING_PLAN_INTERNATIONAL_BASE_URL =
  'glm-coding-plan-international';

const TOOL_NAME_MAP = {
  'search-prime': '联网搜索',
  'web-reader': '网页读取',
  zread: '开源仓库',
};

const clampPercent = (value) => {
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) return 0;
  return Math.max(0, Math.min(100, numericValue));
};

const toNumber = (value) => {
  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : null;
};

const normalizeEpochMs = (value) => {
  const numericValue = toNumber(value);
  if (numericValue == null || numericValue <= 0) return null;
  const absValue = Math.abs(numericValue);
  if (absValue >= 1e18) return Math.floor(numericValue / 1e6);
  if (absValue >= 1e15) return Math.floor(numericValue / 1e3);
  if (absValue >= 1e12) return Math.floor(numericValue);
  return Math.floor(numericValue * 1000);
};

const parseResetTime = (value) => {
  if (value == null || value === '') return null;
  if (typeof value === 'number') {
    return normalizeEpochMs(value);
  }
  const text = String(value).trim();
  if (text === '') return null;
  if (/^-?\d+$/.test(text)) {
    return normalizeEpochMs(text);
  }
  const parsed = Date.parse(text);
  return Number.isNaN(parsed) ? null : parsed;
};

const formatResetTime = (value, t) => {
  const epochMs = parseResetTime(value);
  if (epochMs == null) {
    return value ? String(value) : t('未知');
  }
  try {
    return new Date(epochMs).toLocaleString();
  } catch (error) {
    return String(value);
  }
};

const getProgressStroke = (value) => {
  const percent = clampPercent(value);
  if (percent >= 80) return '#ef4444';
  if (percent >= 50) return '#f59e0b';
  return '#22c55e';
};

const getStatusTagColor = (value) => {
  const percent = clampPercent(value);
  if (percent >= 80) return 'red';
  if (percent >= 50) return 'orange';
  return 'green';
};

const getStatusLabel = (value, t) => {
  const percent = clampPercent(value);
  if (percent >= 80) return t('紧张');
  if (percent >= 50) return t('适中');
  return t('充裕');
};

const getPlanRegionLabel = (record, t) => {
  const baseURL = String(record?.base_url || '').trim();
  if (
    baseURL === ZHIPU_CODING_PLAN_INTERNATIONAL_BASE_URL ||
    baseURL.includes('api.z.ai')
  ) {
    return t('国际版');
  }
  if (
    baseURL === ZHIPU_CODING_PLAN_BASE_URL ||
    baseURL.includes('bigmodel.cn')
  ) {
    return t('国内版');
  }
  return t('未知');
};

const resolveKeyStatusMeta = (payload, t) => {
  const status = Number(payload?.key_status);
  if (status === 2) {
    return { color: 'red', label: t('已禁用') };
  }
  return { color: 'green', label: t('启用') };
};

const KeyPager = ({
  t,
  payload,
  loading,
  currentKeyIndex,
  jumpKeyNumber,
  setJumpKeyNumber,
  onPrev,
  onNext,
  onJump,
}) => {
  const keyCount = Math.max(Number(payload?.key_count || 1), 1);
  if (keyCount <= 1) {
    return null;
  }
  const keyMeta = resolveKeyStatusMeta(payload, t);
  const handleJumpKeyDown = useCallback(
    (event) => {
      if (event.key === 'Enter') {
        event.preventDefault();
        onJump();
      }
    },
    [onJump],
  );

  return (
    <div className='flex flex-col gap-3 rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          size='small'
          theme='outline'
          onClick={onPrev}
          disabled={loading || currentKeyIndex <= 0}
        >
          {t('上一页')}
        </Button>
        <Tag color='blue' type='light'>
          {payload?.key_label || `Key #${currentKeyIndex + 1}`}
        </Tag>
        <Tag color={keyMeta.color} type='light'>
          {keyMeta.label}
        </Tag>
        <Text type='tertiary' size='small'>
          {t('第 {{current}} / {{total}} 个密钥', {
            current: currentKeyIndex + 1,
            total: keyCount,
          })}
        </Text>
        {payload?.disabled_reason && (
          <Text type='danger' size='small'>
            {t('原因')}: {payload.disabled_reason}
          </Text>
        )}
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <InputNumber
          min={1}
          max={keyCount}
          value={jumpKeyNumber}
          size='small'
          disabled={loading}
          style={{ width: 112 }}
          placeholder={t('密钥编号')}
          onChange={(value) => setJumpKeyNumber(value)}
          onKeyDown={handleJumpKeyDown}
        />
        <Button
          size='small'
          theme='outline'
          onClick={onJump}
          disabled={loading}
        >
          {t('跳转')}
        </Button>
        <Button
          size='small'
          theme='outline'
          onClick={onNext}
          disabled={loading || currentKeyIndex >= keyCount - 1}
        >
          {t('下一页')}
        </Button>
      </div>
    </div>
  );
};

const resolveLimitTitle = (item, t) => {
  if (Number(item?.unit) === 6) return t('每周额度');
  switch (item?.type) {
    case 'TOKENS_LIMIT':
      return t('5小时额度');
    case 'TIME_LIMIT':
      return t('MCP 工具额度');
    default:
      return item?.type || t('额度窗口');
  }
};

const getZhipuCodingPlanSource = (payload) => {
  const upstream = payload?.data;
  return upstream && typeof upstream === 'object' && upstream !== null
    ? upstream?.data || upstream
    : null;
};

const formatPlanLevel = (value, t) => {
  const text = String(value || '').trim();
  if (!text) {
    return t('未知');
  }
  if (/^[a-z0-9_-]+$/i.test(text)) {
    return text
      .split(/[_-]+/)
      .filter(Boolean)
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(' ');
  }
  return text;
};

const normalizeLimitCards = (payload, t) => {
  const source = getZhipuCodingPlanSource(payload);
  const limits = Array.isArray(source?.limits) ? source.limits : [];
  const orderWeight = (item) => {
    if (item?.type === 'TOKENS_LIMIT') return 1;
    if (Number(item?.unit) === 6) return 2;
    if (item?.type === 'TIME_LIMIT') return 3;
    return 9;
  };

  return [...limits]
    .sort((left, right) => orderWeight(left) - orderWeight(right))
    .map((item, index) => {
      const currentValue = toNumber(item?.currentValue);
      const totalValue = toNumber(item?.usage);
      const usageLabel =
        currentValue != null && totalValue != null
          ? `${currentValue.toLocaleString()}/${totalValue.toLocaleString()}`
          : null;

      return {
        key: `${item?.type || 'limit'}-${item?.unit || 0}-${index}`,
        title: resolveLimitTitle(item, t),
        percentage: clampPercent(item?.percentage),
        usageLabel,
        nextResetTime: formatResetTime(item?.nextResetTime, t),
        details: Array.isArray(item?.usageDetails)
          ? item.usageDetails.map((detail, detailIndex) => ({
              key: `${detail?.modelCode || 'detail'}-${detailIndex}`,
              name:
                TOOL_NAME_MAP[detail?.modelCode] ||
                detail?.modelCode ||
                t('未知工具'),
              usage: toNumber(detail?.usage),
            }))
          : [],
      };
    });
};

const ZhipuLimitCard = ({ t, card }) => {
  return (
    <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='font-medium'>{card.title}</div>
        <Tag color={getStatusTagColor(card.percentage)} size='small'>
          {getStatusLabel(card.percentage, t)}
        </Tag>
      </div>
      <div className='mt-3 flex items-baseline justify-between gap-3'>
        <div
          className='text-2xl font-semibold'
          style={{ color: getProgressStroke(card.percentage) }}
        >
          {`${Math.floor(card.percentage)}%`}
        </div>
        {card.usageLabel && (
          <Text type='tertiary' size='small'>
            {t('当前用量')} {card.usageLabel}
          </Text>
        )}
      </div>
      <Progress
        percent={Math.floor(card.percentage)}
        showInfo={false}
        stroke={getProgressStroke(card.percentage)}
        size='small'
        style={{ marginTop: 8 }}
      />
      <div className='mt-2 text-xs text-semi-color-text-2'>
        {t('重置时间')}: {card.nextResetTime}
      </div>
      {card.details.length > 0 && (
        <div className='mt-3 border-t border-semi-color-border pt-2'>
          {card.details.map((detail) => (
            <div
              key={detail.key}
              className='flex items-center justify-between py-1 text-xs'
            >
              <Text type='tertiary' size='small'>
                {detail.name}
              </Text>
              <span>
                {detail.usage == null ? '-' : detail.usage.toLocaleString()}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

const ZhipuCodingPlanUsageView = ({ t, record, payload, onRefresh }) => {
  const cards = useMemo(() => normalizeLimitCards(payload, t), [payload, t]);
  const source = useMemo(() => getZhipuCodingPlanSource(payload), [payload]);
  const rawJSON = useMemo(() => {
    const rawData = payload?.data;
    if (rawData && typeof rawData === 'object') {
      return JSON.stringify(rawData, null, 2);
    }
    return rawData ? String(rawData) : '';
  }, [payload]);
  const hasWeeklyLimit = cards.some((card) => card.title === t('每周额度'));
  const planLevelLabel = useMemo(
    () => formatPlanLevel(source?.level, t),
    [source?.level, t],
  );

  if (!payload?.success) {
    return (
      <div className='flex flex-col gap-3'>
        <Text type='danger'>
          {payload?.message || t('获取智谱 Coding Plan 额度失败')}
        </Text>
        <div className='flex justify-end'>
          <Button
            size='small'
            type='primary'
            theme='outline'
            onClick={onRefresh}
          >
            {t('刷新')}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className='flex flex-col gap-4'>
      <div className='rounded-lg border border-semi-color-border bg-semi-color-fill-0 p-4'>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex flex-wrap items-center gap-2'>
            <div className='text-base font-semibold'>
              {t('智谱 Coding Plan 额度')}
            </div>
            <Tag color='blue' type='light'>
              {getPlanRegionLabel(record, t)}
            </Tag>
            <Tag color='cyan' type='light'>
              {t('套餐等级')}: {planLevelLabel}
            </Tag>
            <Tag color={hasWeeklyLimit ? 'green' : 'orange'} type='light'>
              {hasWeeklyLimit ? t('新套餐') : t('旧套餐')}
            </Tag>
          </div>
          <Button
            size='small'
            type='primary'
            theme='outline'
            onClick={onRefresh}
          >
            {t('刷新')}
          </Button>
        </div>
        <div className='mt-2 text-xs text-semi-color-text-2'>
          {t('请求地址')}: {payload?.request_url || '-'}
        </div>
        <div className='mt-1 text-xs text-semi-color-text-2'>
          {t('上游状态')}: {payload?.upstream_status ?? '-'}
        </div>
      </div>

      {cards.length > 0 ? (
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
          {cards.map((card) => (
            <ZhipuLimitCard key={card.key} t={t} card={card} />
          ))}
        </div>
      ) : (
        <div className='rounded-lg border border-dashed border-semi-color-border bg-semi-color-fill-0 p-4 text-sm text-semi-color-text-2'>
          {t(
            '当前未解析到可用额度窗口，可能不是 Coding Plan Key，或上游接口字段已变化。',
          )}
        </div>
      )}

      <Collapse>
        <Collapse.Panel header={t('原始响应')} itemKey='raw'>
          <pre className='max-h-80 overflow-auto rounded bg-semi-color-fill-0 p-3 text-xs leading-5'>
            {rawJSON || '-'}
          </pre>
        </Collapse.Panel>
      </Collapse>
    </div>
  );
};

const ZhipuCodingPlanUsageLoader = ({ t, record, initialPayload }) => {
  const [loading, setLoading] = useState(!initialPayload);
  const [payload, setPayload] = useState(initialPayload ?? null);
  const [currentKeyIndex, setCurrentKeyIndex] = useState(
    Number(initialPayload?.key_index ?? 0),
  );
  const [jumpKeyNumber, setJumpKeyNumber] = useState(
    Number(initialPayload?.key_index ?? 0) + 1,
  );
  const hasShownErrorRef = useRef(false);
  const mountedRef = useRef(true);
  const recordId = record?.id;

  const fetchUsage = useCallback(
    async (requestedKeyIndex) => {
      if (!recordId) {
        if (mountedRef.current) setPayload(null);
        return;
      }

      if (mountedRef.current) setLoading(true);
      try {
        const query = `?key_index=${Math.max(Number(requestedKeyIndex || 0), 0)}`;
        const res = await API.get(
          `/api/channel/${recordId}/zhipu/coding_plan/usage${query}`,
          {
            skipErrorHandler: true,
          },
        );
        if (!mountedRef.current) return;
        setPayload(res?.data ?? null);
        const resolvedKeyIndex = Number(
          res?.data?.key_index ?? requestedKeyIndex ?? 0,
        );
        setCurrentKeyIndex(resolvedKeyIndex);
        setJumpKeyNumber(resolvedKeyIndex + 1);
        if (!res?.data?.success && !hasShownErrorRef.current) {
          hasShownErrorRef.current = true;
          showError(t('获取智谱 Coding Plan 额度失败'));
        }
      } catch (error) {
        if (!mountedRef.current) return;
        if (!hasShownErrorRef.current) {
          hasShownErrorRef.current = true;
          showError(t('获取智谱 Coding Plan 额度失败'));
        }
        setPayload({ success: false, message: String(error) });
      } finally {
        if (mountedRef.current) setLoading(false);
      }
    },
    [recordId, t],
  );

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    const initialIndex = Number(initialPayload?.key_index ?? 0);
    setCurrentKeyIndex(initialIndex);
    setJumpKeyNumber(initialIndex + 1);
    if (initialPayload) {
      setPayload(initialPayload);
      setLoading(false);
    } else {
      setPayload(null);
    }
    hasShownErrorRef.current = false;
  }, [initialPayload, recordId]);

  useEffect(() => {
    if (
      initialPayload &&
      Number(initialPayload?.key_index ?? 0) === currentKeyIndex
    ) {
      return;
    }
    fetchUsage(currentKeyIndex).catch(() => {});
  }, [currentKeyIndex, fetchUsage, initialPayload]);

  const handlePrev = useCallback(() => {
    if (loading) {
      return;
    }
    setCurrentKeyIndex((value) => Math.max(value - 1, 0));
  }, [loading]);

  const handleNext = useCallback(() => {
    if (loading) {
      return;
    }
    const keyCount = Math.max(Number(payload?.key_count || 1), 1);
    setCurrentKeyIndex((value) => Math.min(value + 1, keyCount - 1));
  }, [loading, payload?.key_count]);

  const handleJump = useCallback(() => {
    if (loading) {
      return;
    }
    const keyCount = Math.max(Number(payload?.key_count || 1), 1);
    const requested = Number(jumpKeyNumber);
    if (!Number.isFinite(requested) || requested < 1) {
      return;
    }
    const targetIndex =
      Math.min(Math.max(Math.floor(requested), 1), keyCount) - 1;
    setCurrentKeyIndex(targetIndex);
  }, [jumpKeyNumber, loading, payload?.key_count]);

  if (loading) {
    return (
      <div className='flex items-center justify-center py-10'>
        <Spin spinning={true} size='large' tip={t('加载中...')} />
      </div>
    );
  }

  if (!payload) {
    return (
      <div className='flex flex-col gap-3'>
        <Text type='danger'>{t('获取智谱 Coding Plan 额度失败')}</Text>
        <div className='flex justify-end'>
          <Button
            size='small'
            type='primary'
            theme='outline'
            onClick={() => fetchUsage(currentKeyIndex)}
          >
            {t('刷新')}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className='flex flex-col gap-4'>
      <KeyPager
        t={t}
        payload={payload}
        loading={loading}
        currentKeyIndex={currentKeyIndex}
        jumpKeyNumber={jumpKeyNumber}
        setJumpKeyNumber={setJumpKeyNumber}
        onPrev={handlePrev}
        onNext={handleNext}
        onJump={handleJump}
      />
      <ZhipuCodingPlanUsageView
        t={t}
        record={record}
        payload={payload}
        onRefresh={() => fetchUsage(currentKeyIndex)}
      />
    </div>
  );
};

export const openZhipuCodingPlanUsageModal = ({ t, record, payload }) => {
  Modal.info({
    title: t('智谱 Coding Plan 额度'),
    centered: false,
    width: 960,
    style: { maxWidth: '95vw', top: 12 },
    bodyStyle: {
      maxHeight: 'calc(100vh - 160px)',
      overflowY: 'auto',
      WebkitOverflowScrolling: 'touch',
    },
    content: (
      <ZhipuCodingPlanUsageLoader
        t={t}
        record={record}
        initialPayload={payload}
      />
    ),
    footer: (
      <div className='flex justify-end gap-2'>
        <Button type='primary' theme='solid' onClick={() => Modal.destroyAll()}>
          {t('关闭')}
        </Button>
      </div>
    ),
  });
};
