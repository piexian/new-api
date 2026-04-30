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

const pickStrokeColor = (value) => {
  const percent = clampPercent(value);
  if (percent >= 95) return '#ef4444';
  if (percent >= 80) return '#f59e0b';
  return '#3b82f6';
};

const toNumber = (value) => {
  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : null;
};

const normalizeEpochMs = (value) => {
  const numericValue = toNumber(value);
  if (numericValue == null || numericValue <= 0) return null;
  return numericValue < 1e12 ? numericValue * 1000 : numericValue;
};

const formatDateTime = (value) => {
  const epochMs = normalizeEpochMs(value);
  if (epochMs == null) return '-';
  try {
    return new Date(epochMs).toLocaleString();
  } catch (error) {
    return String(value);
  }
};

const formatDurationMs = (value, t) => {
  const numericValue = toNumber(value);
  if (numericValue == null || numericValue <= 0) return '-';

  const totalSeconds = Math.floor(numericValue / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) return `${hours}${t('小时')} ${minutes}${t('分钟')}`;
  if (minutes > 0) return `${minutes}${t('分钟')} ${seconds}${t('秒')}`;
  return `${seconds}${t('秒')}`;
};

const formatCount = (value) => {
  const numericValue = toNumber(value);
  if (numericValue == null) return '-';
  return numericValue.toLocaleString();
};

const formatPercent = (value) => {
  return `${Math.floor(clampPercent(value))}%`;
};

const toIntegerPercent = (value) => {
  return Math.floor(clampPercent(value));
};

const resolveCurrentWindowLabel = (startTime, endTime, t) => {
  if (startTime == null || endTime == null || endTime <= startTime) {
    return t('当前窗口');
  }

  const durationMs = endTime - startTime;
  const fiveHoursMs = 5 * 60 * 60 * 1000;
  const oneDayMs = 24 * 60 * 60 * 1000;

  if (Math.abs(durationMs - fiveHoursMs) <= 30 * 60 * 1000) {
    return t('5小时窗口');
  }
  if (Math.abs(durationMs - oneDayMs) <= 2 * 60 * 60 * 1000) {
    return t('日额度');
  }
  return t('当前窗口');
};

const resolveWindowQuota = ({ total, remaining, upstreamUsageCount }) => {
  const totalValue = toNumber(total);
  const usedCount = toNumber(upstreamUsageCount);
  const remainingCount = toNumber(remaining);
  const usedValue =
    usedCount ??
    (totalValue != null && remainingCount != null
      ? Math.max(totalValue - remainingCount, 0)
      : null);
  const remainingValue =
    remainingCount ??
    (totalValue != null && usedCount != null
      ? Math.max(totalValue - usedCount, 0)
      : null);

  return {
    total: totalValue,
    used: usedValue,
    remaining: remainingValue,
  };
};

const buildWindow = ({
  key,
  label,
  total,
  used,
  remaining,
  remainsTime,
  startTime,
  endTime,
}) => {
  if (
    total == null &&
    used == null &&
    remaining == null &&
    remainsTime == null &&
    startTime == null &&
    endTime == null
  ) {
    return null;
  }

  const remainValue =
    remaining != null
      ? remaining
      : total != null && used != null
        ? Math.max(total - used, 0)
        : null;
  const hasPositiveQuota =
    (total != null && total > 0) ||
    (used != null && used > 0) ||
    (remainValue != null && remainValue > 0);
  if (!hasPositiveQuota) {
    return null;
  }

  const percent =
    total != null && total > 0 && used != null
      ? toIntegerPercent((used / total) * 100)
      : 0;

  return {
    key,
    label,
    total,
    used,
    remaining: remainValue,
    percent,
    remainsTime,
    startTime,
    endTime,
  };
};

const resolveModelWindows = (item, t) => {
  const currentStartTime = normalizeEpochMs(item?.start_time);
  const currentEndTime = normalizeEpochMs(item?.end_time);
  const weeklyStartTime = normalizeEpochMs(item?.weekly_start_time);
  const weeklyEndTime = normalizeEpochMs(item?.weekly_end_time);
  const currentIntervalQuota = resolveWindowQuota({
    total: item?.current_interval_total_count,
    remaining: item?.current_interval_remaining_count,
    upstreamUsageCount: item?.current_interval_usage_count,
  });
  const currentWeeklyQuota = resolveWindowQuota({
    total: item?.current_weekly_total_count,
    remaining: item?.current_weekly_remaining_count,
    upstreamUsageCount: item?.current_weekly_usage_count,
  });

  return [
    buildWindow({
      key: 'current_interval',
      label: resolveCurrentWindowLabel(currentStartTime, currentEndTime, t),
      total: currentIntervalQuota.total,
      used: currentIntervalQuota.used,
      remaining: currentIntervalQuota.remaining,
      remainsTime: toNumber(item?.remains_time),
      startTime: currentStartTime,
      endTime: currentEndTime,
    }),
    buildWindow({
      key: 'current_weekly',
      label: t('周额度'),
      total: currentWeeklyQuota.total,
      used: currentWeeklyQuota.used,
      remaining: currentWeeklyQuota.remaining,
      remainsTime: toNumber(item?.weekly_remains_time),
      startTime: weeklyStartTime,
      endTime: weeklyEndTime,
    }),
  ].filter(Boolean);
};

const WindowCard = ({ t, windowInfo }) => {
  return (
    <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='font-medium'>{windowInfo.label}</div>
        <div className='flex flex-wrap items-center gap-2'>
          <Tag color='grey' size='small'>
            {t('已用')} {formatPercent(windowInfo.percent)}
          </Tag>
          <Tag
            color={
              windowInfo.remaining === 0
                ? 'red'
                : windowInfo.remaining == null
                  ? 'grey'
                  : 'blue'
            }
            size='small'
          >
            {t('剩余')} {formatCount(windowInfo.remaining)}
          </Tag>
        </div>
      </div>
      <div className='mt-2'>
        <Progress
          percent={windowInfo.percent}
          stroke={pickStrokeColor(windowInfo.percent)}
          showInfo={false}
        />
      </div>
      <div className='mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-semi-color-text-1'>
        <div>
          {t('已用')} {formatCount(windowInfo.used)}
        </div>
        <div>
          {t('总量')} {formatCount(windowInfo.total)}
        </div>
        <div>
          {t('占比')} {formatPercent(windowInfo.percent)}
        </div>
        <div>
          {t('距重置')} {formatDurationMs(windowInfo.remainsTime, t)}
        </div>
        <div>
          {t('开始')} {formatDateTime(windowInfo.startTime)}
        </div>
        <div>
          {t('结束')} {formatDateTime(windowInfo.endTime)}
        </div>
      </div>
    </div>
  );
};

const MiniMaxModelCard = ({ t, item, windows }) => {
  const modelName = String(item?.model_name || item?.model || t('未命名模型'));

  return (
    <div className='rounded-xl border border-semi-color-border bg-semi-color-bg-1 p-4'>
      <div className='mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='break-all font-semibold text-semi-color-text-0'>
          {modelName}
        </div>
        <Tag color='light-blue'>{t('Token Plan')}</Tag>
      </div>
      {windows.length > 0 ? (
        <div className='grid gap-3 md:grid-cols-2'>
          {windows.map((windowInfo) => (
            <WindowCard key={windowInfo.key} t={t} windowInfo={windowInfo} />
          ))}
        </div>
      ) : null}
    </div>
  );
};

const MetaBlock = ({ label, value }) => {
  return (
    <div className='min-w-0 rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
      <div className='mb-1 text-xs text-semi-color-text-2'>{label}</div>
      <div className='min-w-0 break-all text-sm text-semi-color-text-0'>
        {value}
      </div>
    </div>
  );
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

const MiniMaxUsageView = ({
  t,
  record,
  payload,
  onRefresh,
  compact = false,
}) => {
  const upstreamData =
    payload?.data && typeof payload.data === 'object' ? payload.data : null;
  const modelRemains = Array.isArray(upstreamData?.model_remains)
    ? upstreamData.model_remains
    : [];
  const parsedModels = useMemo(
    () =>
      modelRemains.map((item) => ({
        item,
        windows: resolveModelWindows(item, t),
      })),
    [modelRemains, t],
  );
  const activeModels = parsedModels.filter((entry) => entry.windows.length > 0);
  const unavailableModels = parsedModels
    .filter((entry) => entry.windows.length === 0)
    .map((entry) => String(entry.item?.model_name || entry.item?.model || ''))
    .filter(Boolean);
  const baseResp = upstreamData?.base_resp ?? null;
  const rawJSON = useMemo(
    () => JSON.stringify(payload?.data ?? payload ?? {}, null, 2),
    [payload],
  );

  return (
    <div className='flex flex-col gap-4 pr-1'>
      {!compact && (
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div className='min-w-0 break-all text-xs text-semi-color-text-2'>
            {t('渠道：')}
            {record?.name || '-'} ({t('编号：')}
            {record?.id || '-'})
          </div>
          <Button
            type='primary'
            theme='outline'
            size='small'
            onClick={onRefresh}
            className='self-start'
          >
            {t('刷新')}
          </Button>
        </div>
      )}

      {!payload?.success && (
        <div className='rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600'>
          {payload?.message || t('查询失败')}
        </div>
      )}

      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        <MetaBlock
          label={t('上游状态')}
          value={payload?.upstream_status ?? '-'}
        />
        <MetaBlock label={t('请求地址')} value={payload?.request_url || '-'} />
        <MetaBlock
          label={t('业务状态')}
          value={
            baseResp?.status_code === 0
              ? t('正常')
              : baseResp?.status_code != null
                ? `${baseResp.status_code}`
                : t('未知')
          }
        />
        <MetaBlock
          label={t('业务消息')}
          value={baseResp?.status_msg || payload?.message || '-'}
        />
      </div>

      <div className='flex flex-col gap-3'>
        {activeModels.length > 0 ? (
          activeModels.map(({ item, windows }, index) => (
            <MiniMaxModelCard
              key={`${item?.model_name || item?.model || 'model'}-${index}`}
              t={t}
              item={item}
              windows={windows}
            />
          ))
        ) : (
          <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 px-4 py-6 text-sm text-semi-color-text-2'>
            {t(
              '当前未解析到可用额度窗口，可能不是 Token Plan Key，或上游接口字段已变化。',
            )}
          </div>
        )}
        {unavailableModels.length > 0 && (
          <div className='rounded-xl border border-dashed border-semi-color-border bg-semi-color-bg-0 p-4'>
            <div className='mb-2 text-sm font-medium text-semi-color-text-0'>
              {t('当前套餐未开放或未包含的模型')}
            </div>
            <div className='flex flex-wrap gap-2'>
              {unavailableModels.map((modelName) => (
                <Tag key={modelName} color='grey' type='ghost'>
                  {modelName}
                </Tag>
              ))}
            </div>
          </div>
        )}
      </div>

      <Collapse>
        <Collapse.Panel header={t('原始响应')} itemKey='raw'>
          <pre className='max-h-80 overflow-auto rounded bg-semi-color-fill-0 p-3 text-xs leading-5'>
            {rawJSON}
          </pre>
        </Collapse.Panel>
      </Collapse>
    </div>
  );
};

const MiniMaxUsageLoader = ({ t, record, initialPayload }) => {
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
          `/api/channel/${recordId}/minimax/usage${query}`,
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
          showError(t('获取 MiniMax Token Plan 用量失败'));
        }
      } catch (error) {
        if (!mountedRef.current) return;
        if (!hasShownErrorRef.current) {
          hasShownErrorRef.current = true;
          showError(t('获取 MiniMax Token Plan 用量失败'));
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
        <Text type='danger'>{t('获取 MiniMax Token Plan 用量失败')}</Text>
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
      <MiniMaxUsageView
        t={t}
        record={record}
        payload={payload}
        onRefresh={() => fetchUsage(currentKeyIndex)}
      />
    </div>
  );
};

export const openMiniMaxUsageModal = ({ t, record, payload }) => {
  Modal.info({
    title: t('MiniMax Token Plan 用量'),
    centered: false,
    width: 960,
    style: { maxWidth: '95vw', top: 12 },
    bodyStyle: {
      maxHeight: 'calc(100vh - 160px)',
      overflowY: 'auto',
      WebkitOverflowScrolling: 'touch',
    },
    content: (
      <MiniMaxUsageLoader t={t} record={record} initialPayload={payload} />
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
