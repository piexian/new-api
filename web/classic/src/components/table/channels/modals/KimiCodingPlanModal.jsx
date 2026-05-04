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

const formatDurationMs = (ms, t) => {
  const v = toNumber(ms);
  if (v == null || v <= 0) return '-';
  const totalSeconds = Math.floor(v / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}${t('小时')} ${minutes}${t('分')}`;
  if (minutes > 0) return `${minutes}${t('分')} ${seconds}${t('秒')}`;
  return `${seconds}${t('秒')}`;
};

const formatResetHint = (detail, t) => {
  const candidate =
    detail?.resetAt ??
    detail?.reset_at ??
    detail?.resetTime ??
    detail?.reset_time;
  if (candidate != null && candidate !== '') {
    const epochMs = parseResetTime(candidate);
    if (epochMs != null) {
      const delta = epochMs - Date.now();
      if (delta > 0) {
        return `${t('剩余')} ${formatDurationMs(delta, t)}`;
      }
      return t('已重置');
    }
    return `${t('重置时间')}: ${String(candidate)}`;
  }
  for (const key of ['reset_in', 'resetIn', 'ttl']) {
    const seconds = toNumber(detail?.[key]);
    if (seconds != null && seconds > 0) {
      return `${t('剩余')} ${formatDurationMs(seconds * 1000, t)}`;
    }
  }
  return null;
};

const formatLimitLabel = (item, detail, win, index, t) => {
  for (const key of ['name', 'title', 'scope']) {
    const v = item?.[key] ?? detail?.[key];
    if (v != null && String(v).trim() !== '') return String(v);
  }
  const duration = toNumber(win?.duration ?? item?.duration ?? detail?.duration);
  const timeUnit = String(
    win?.timeUnit ?? item?.timeUnit ?? detail?.timeUnit ?? '',
  ).toUpperCase();
  if (duration && duration > 0) {
    if (timeUnit.includes('MINUTE')) {
      if (duration >= 60 && duration % 60 === 0) {
        return `${duration / 60}h ${t('额度')}`;
      }
      return `${duration}m ${t('额度')}`;
    }
    if (timeUnit.includes('HOUR')) return `${duration}h ${t('额度')}`;
    if (timeUnit.includes('DAY')) return `${duration}d ${t('额度')}`;
    return `${duration}s ${t('额度')}`;
  }
  return `${t('额度')} #${index + 1}`;
};

const buildRow = (data, defaultLabel, key, resetHint) => {
  const limit = toNumber(data?.limit) ?? 0;
  let used = toNumber(data?.used);
  if (used == null) {
    const remaining = toNumber(data?.remaining);
    if (remaining != null && limit > 0) {
      used = Math.max(limit - remaining, 0);
    }
  }
  if (used == null && limit <= 0) return null;
  const usedSafe = used ?? 0;
  const remaining = limit > 0 ? Math.max(limit - usedSafe, 0) : 0;
  const percent =
    limit > 0 ? Math.floor(clampPercent((usedSafe / limit) * 100)) : 0;
  const labelRaw = data?.name ?? data?.title;
  const label =
    labelRaw != null && String(labelRaw).trim() !== ''
      ? String(labelRaw)
      : defaultLabel;
  return { key, label, used: usedSafe, limit, remaining, percent, resetHint };
};

const normalizeRows = (payload, t) => {
  const source = payload?.data;
  if (!source || typeof source !== 'object') {
    return { summary: null, limits: [] };
  }
  const summary = source.usage
    ? buildRow(source.usage, t('总额度'), 'summary', formatResetHint(source.usage, t))
    : null;
  const rawLimits = Array.isArray(source.limits) ? source.limits : [];
  const limits = [];
  rawLimits.forEach((item, index) => {
    if (!item || typeof item !== 'object') return;
    const detail = item.detail && typeof item.detail === 'object' ? item.detail : item;
    const win = item.window && typeof item.window === 'object' ? item.window : {};
    const label = formatLimitLabel(item, detail, win, index, t);
    const resetHint = formatResetHint(detail, t);
    const row = buildRow(detail, label, `limit-${index}`, resetHint);
    if (row) {
      row.label =
        detail.name != null && String(detail.name).trim() !== ''
          ? String(detail.name)
          : label;
      limits.push(row);
    }
  });
  return { summary, limits };
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

const resolveKeyStatusMeta = (payload, t) => {
  const status = Number(payload?.key_status);
  if (status === 2) return { color: 'red', label: t('已禁用') };
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
  if (keyCount <= 1) return null;
  const keyMeta = resolveKeyStatusMeta(payload, t);
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
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              onJump();
            }
          }}
        />
        <Button size='small' theme='outline' onClick={onJump} disabled={loading}>
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

const KimiUsageRowCard = ({ t, row }) => {
  return (
    <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='font-medium'>{row.label}</div>
        <Tag color={getStatusTagColor(row.percent)} size='small'>
          {`${row.percent}%`}
        </Tag>
      </div>
      <Progress
        percent={row.percent}
        showInfo={false}
        stroke={getProgressStroke(row.percent)}
        size='small'
        style={{ marginTop: 8 }}
      />
      <div className='mt-2 grid grid-cols-2 gap-1 text-xs text-semi-color-text-2'>
        <div>{t('已用')}: {row.used.toLocaleString()}</div>
        <div>{t('剩余')}: {row.remaining.toLocaleString()}</div>
        <div>{t('总量')}: {row.limit.toLocaleString()}</div>
        {row.resetHint && <div>{row.resetHint}</div>}
      </div>
    </div>
  );
};

const KimiCodingPlanUsageView = ({ t, payload, onRefresh }) => {
  const { summary, limits } = useMemo(() => normalizeRows(payload, t), [payload, t]);
  const rawJSON = useMemo(() => {
    const rawData = payload?.data;
    if (rawData && typeof rawData === 'object') {
      return JSON.stringify(rawData, null, 2);
    }
    return rawData ? String(rawData) : '';
  }, [payload]);

  if (!payload?.success) {
    return (
      <div className='flex flex-col gap-3'>
        <Text type='danger'>
          {payload?.message || t('获取 Kimi Coding Plan 额度失败')}
        </Text>
        <div className='flex justify-end'>
          <Button size='small' type='primary' theme='outline' onClick={onRefresh}>
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
            <div className='text-base font-semibold'>{t('Kimi Coding Plan 额度')}</div>
            <Tag color='cyan' type='light'>{t('Kimi')}</Tag>
          </div>
          <Button size='small' type='primary' theme='outline' onClick={onRefresh}>
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

      {summary || limits.length > 0 ? (
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
          {summary && <KimiUsageRowCard key={summary.key} t={t} row={summary} />}
          {limits.map((row) => (
            <KimiUsageRowCard key={row.key} t={t} row={row} />
          ))}
        </div>
      ) : (
        <div className='rounded-lg border border-dashed border-semi-color-border bg-semi-color-fill-0 p-4 text-sm text-semi-color-text-2'>
          {t('当前未解析到可用额度窗口，可能不是 Coding Plan Key，或上游接口字段已变化。')}
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

const KimiCodingPlanUsageLoader = ({ t, record, initialPayload }) => {
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
          `/api/channel/${recordId}/kimi/coding_plan/usage${query}`,
          { skipErrorHandler: true },
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
          showError(t('获取 Kimi Coding Plan 额度失败'));
        }
      } catch (error) {
        if (!mountedRef.current) return;
        if (!hasShownErrorRef.current) {
          hasShownErrorRef.current = true;
          showError(t('获取 Kimi Coding Plan 额度失败'));
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
    if (loading) return;
    setCurrentKeyIndex((value) => Math.max(value - 1, 0));
  }, [loading]);

  const handleNext = useCallback(() => {
    if (loading) return;
    const keyCount = Math.max(Number(payload?.key_count || 1), 1);
    setCurrentKeyIndex((value) => Math.min(value + 1, keyCount - 1));
  }, [loading, payload?.key_count]);

  const handleJump = useCallback(() => {
    if (loading) return;
    const keyCount = Math.max(Number(payload?.key_count || 1), 1);
    const requested = Number(jumpKeyNumber);
    if (!Number.isFinite(requested) || requested < 1) return;
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
        <Text type='danger'>{t('获取 Kimi Coding Plan 额度失败')}</Text>
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
      <KimiCodingPlanUsageView
        t={t}
        payload={payload}
        onRefresh={() => fetchUsage(currentKeyIndex)}
      />
    </div>
  );
};

export const openKimiCodingPlanUsageModal = ({ t, record, payload }) => {
  Modal.info({
    title: t('Kimi Coding Plan 额度'),
    centered: false,
    width: 960,
    style: { maxWidth: '95vw', top: 12 },
    bodyStyle: {
      maxHeight: 'calc(100vh - 160px)',
      overflowY: 'auto',
      WebkitOverflowScrolling: 'touch',
    },
    content: (
      <KimiCodingPlanUsageLoader
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
