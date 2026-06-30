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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import {
  Avatar,
  Button,
  Card,
  Col,
  Empty,
  Form,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
  Popconfirm,
  Row,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconCreditCard,
  IconEdit,
  IconKey,
  IconLink,
  IconPlus,
  IconRefresh,
  IconSave,
} from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  getCurrencyConfig,
  getModelCategories,
  renderGroupOption,
  showError,
  showSuccess,
  renderQuota,
  selectFilter,
  timestamp2string,
} from '../../../../helpers';
import {
  displayAmountToQuota,
  quotaToDisplayAmount,
} from '../../../../helpers/quota';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { StatusContext } from '../../../../context/Status';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

const PAGE_SIZE = 10;
const API_KEY_STATUS_ENABLED = 1;
const API_KEY_STATUS_DISABLED = 2;

function formatExpiredTime(expiredTime, t) {
  if (expiredTime === -1 || !expiredTime) {
    return t('永不过期');
  }
  return timestamp2string(expiredTime);
}

function getStatusTag(status, t) {
  if (status === 1) {
    return { color: 'green', text: t('已启用') };
  }
  if (status === 2) {
    return { color: 'red', text: t('已禁用') };
  }
  if (status === 3) {
    return { color: 'yellow', text: t('已过期') };
  }
  if (status === 4) {
    return { color: 'grey', text: t('已耗尽') };
  }
  return { color: 'black', text: t('未知状态') };
}

function getDefaultTokenValues(defaultUseAutoGroup, hasAutoGroup) {
  const useAutoGroup = defaultUseAutoGroup && hasAutoGroup;
  return {
    name: '',
    remain_quota: 0,
    remain_amount: 0,
    expired_time: -1,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: [],
    allow_ips: '',
    group: useAutoGroup ? 'auto' : '',
    cross_group_retry: useAutoGroup,
    tokenCount: 1,
  };
}

function normalizeTokenValues(token, defaultUseAutoGroup, hasAutoGroup) {
  const defaults = getDefaultTokenValues(defaultUseAutoGroup, hasAutoGroup);
  const modelLimits =
    typeof token?.model_limits === 'string' && token.model_limits
      ? token.model_limits.split(',').filter(Boolean)
      : [];

  return {
    ...defaults,
    ...token,
    remain_amount: Number(
      quotaToDisplayAmount(token?.remain_quota || 0).toFixed(6),
    ),
    expired_time:
      token?.expired_time && token.expired_time !== -1
        ? timestamp2string(token.expired_time)
        : -1,
    unlimited_quota: !!token?.unlimited_quota,
    model_limits: modelLimits,
    allow_ips: token?.allow_ips || '',
    group: token?.group || defaults.group,
    cross_group_retry: !!token?.cross_group_retry,
    tokenCount: 1,
  };
}

function generateRandomSuffix() {
  const characters =
    'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < 6; i++) {
    result += characters.charAt(Math.floor(Math.random() * characters.length));
  }
  return result;
}

function UserTokenEditor({
  visible,
  user,
  token,
  t,
  models,
  groups,
  optionsLoading,
  defaultUseAutoGroup,
  onCancel,
  onSuccess,
}) {
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [showQuotaInput, setShowQuotaInput] = useState(false);
  const isEdit = !!token?.id;
  const hasAutoGroup = groups.some((group) => group.value === 'auto');

  const resetForm = (values) => {
    if (formApiRef.current) {
      formApiRef.current.setValues(values);
    }
  };

  const setExpiredTime = (month, day, hour, minute) => {
    if (!formApiRef.current) return;
    let timestamp = Date.now() / 1000;
    let seconds = month * 30 * 24 * 60 * 60;
    seconds += day * 24 * 60 * 60;
    seconds += hour * 60 * 60;
    seconds += minute * 60;
    if (seconds === 0) {
      formApiRef.current.setValue('expired_time', -1);
      return;
    }
    timestamp += seconds;
    formApiRef.current.setValue('expired_time', timestamp2string(timestamp));
  };

  const loadToken = async () => {
    if (!user?.id || !token?.id) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/user/${user.id}/tokens/${token.id}`);
      if (res.data?.success) {
        resetForm(
          normalizeTokenValues(
            res.data.data || token,
            defaultUseAutoGroup,
            hasAutoGroup,
          ),
        );
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      formApiRef.current?.reset();
      setShowQuotaInput(false);
      return;
    }
    if (isEdit) {
      loadToken();
    } else {
      resetForm(getDefaultTokenValues(defaultUseAutoGroup, hasAutoGroup));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, token?.id, defaultUseAutoGroup, hasAutoGroup]);

  const normalizePayload = (values, name) => {
    const payload = {
      name,
      remain_quota: values.unlimited_quota
        ? 0
        : displayAmountToQuota(values.remain_amount),
      expired_time: values.expired_time,
      unlimited_quota: !!values.unlimited_quota,
      model_limits_enabled: values.model_limits?.length > 0,
      model_limits: (values.model_limits || []).join(','),
      allow_ips: values.allow_ips || '',
      group: values.group || '',
      cross_group_retry:
        values.group === 'auto' ? !!values.cross_group_retry : false,
    };

    if (!payload.unlimited_quota && payload.remain_quota < 0) {
      showError(t('额度不能为负数'));
      return null;
    }

    if (payload.expired_time !== -1) {
      const time = Date.parse(payload.expired_time);
      if (Number.isNaN(time)) {
        showError(t('过期时间格式错误！'));
        return null;
      }
      payload.expired_time = Math.ceil(time / 1000);
    }

    return payload;
  };

  const submit = async (values) => {
    if (!user?.id) return;
    const baseName = (values.name || '').trim();
    if (!baseName) {
      showError(t('请输入名称'));
      return;
    }

    setLoading(true);
    try {
      if (isEdit) {
        const payload = normalizePayload(values, baseName);
        if (!payload) return;
        const res = await API.put(
          `/api/user/${user.id}/tokens/${token.id}`,
          payload,
        );
        if (res.data?.success) {
          showSuccess(t('令牌更新成功！'));
          onSuccess?.('update');
        } else {
          showError(res.data?.message || t('更新失败'));
        }
        return;
      }

      const count = parseInt(values.tokenCount, 10) || 1;
      let successCount = 0;
      for (let i = 0; i < count; i++) {
        const name =
          i === 0 ? baseName : `${baseName}-${generateRandomSuffix()}`;
        const payload = normalizePayload(values, name);
        if (!payload) break;
        const res = await API.post(`/api/user/${user.id}/tokens`, payload);
        if (res.data?.success) {
          successCount++;
        } else {
          showError(res.data?.message || t('创建失败'));
          break;
        }
      }

      if (successCount > 0) {
        showSuccess(t('已创建 {{count}} 个令牌！', { count: successCount }));
        onSuccess?.('create');
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <SideSheet
      placement={isEdit ? 'right' : 'left'}
      title={
        <Space>
          <Tag color={isEdit ? 'blue' : 'green'} shape='circle'>
            {isEdit ? t('更新') : t('新建')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {isEdit ? t('更新令牌信息') : t('创建新的令牌')}
          </Typography.Title>
        </Space>
      }
      bodyStyle={{ padding: 0 }}
      visible={visible}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
              className='!rounded-lg'
              type='primary'
              onClick={onCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={onCancel}
    >
      <Spin spinning={loading || optionsLoading}>
        <Form
          key={isEdit ? `edit-${token?.id}` : 'create-user-token'}
          initValues={getDefaultTokenValues(defaultUseAutoGroup, hasAutoGroup)}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
        >
          {({ values }) => (
            <div className='p-2'>
              <Card className='!rounded-2xl shadow-sm border-0'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                    <IconKey size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>{t('基本信息')}</Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置令牌的基本信息')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={24}>
                    <Form.Input
                      field='name'
                      label={t('名称')}
                      placeholder={t('请输入名称')}
                      rules={[{ required: true, message: t('请输入名称') }]}
                      showClear
                    />
                  </Col>
                  <Col span={24}>
                    {groups.length > 0 ? (
                      <Form.Select
                        field='group'
                        label={t('令牌分组')}
                        placeholder={t('令牌分组，默认为用户的分组')}
                        optionList={groups}
                        renderOptionItem={renderGroupOption}
                        onChange={(value) => {
                          formApiRef.current?.setValue('group', value || '');
                          if (value !== 'auto') {
                            formApiRef.current?.setValue(
                              'cross_group_retry',
                              false,
                            );
                          }
                        }}
                        filter={(input, option) => {
                          const q = input.toLowerCase();
                          return (
                            option.value?.toLowerCase().includes(q) ||
                            (typeof option.label === 'string' &&
                              option.label.toLowerCase().includes(q)) ||
                            (typeof option.desc === 'string' &&
                              option.desc.toLowerCase().includes(q))
                          );
                        }}
                        showClear
                        style={{ width: '100%' }}
                      />
                    ) : (
                      <Form.Select
                        placeholder={t('管理员未设置用户可选分组')}
                        disabled
                        label={t('令牌分组')}
                        style={{ width: '100%' }}
                      />
                    )}
                  </Col>
                  <Col
                    span={24}
                    style={{
                      display: values.group === 'auto' ? 'block' : 'none',
                    }}
                  >
                    <Form.Switch
                      field='cross_group_retry'
                      label={t('跨分组重试')}
                      size='default'
                      extraText={t(
                        '开启后，当前分组渠道失败时会按顺序尝试下一个分组的渠道',
                      )}
                    />
                  </Col>
                  <Col xs={24} sm={24} md={24} lg={10} xl={10}>
                    <Form.DatePicker
                      field='expired_time'
                      label={t('过期时间')}
                      type='dateTime'
                      placeholder={t('请选择过期时间')}
                      rules={[
                        { required: true, message: t('请选择过期时间') },
                        {
                          validator: (rule, value) => {
                            if (value === -1 || !value) {
                              return Promise.resolve();
                            }
                            const time = Date.parse(value);
                            if (Number.isNaN(time)) {
                              return Promise.reject(t('过期时间格式错误！'));
                            }
                            if (time <= Date.now()) {
                              return Promise.reject(
                                t('过期时间不能早于当前时间！'),
                              );
                            }
                            return Promise.resolve();
                          },
                        },
                      ]}
                      showClear
                      style={{ width: '100%' }}
                    />
                  </Col>
                  <Col xs={24} sm={24} md={24} lg={14} xl={14}>
                    <Form.Slot label={t('过期时间快捷设置')}>
                      <Space wrap>
                        <Button
                          theme='light'
                          type='primary'
                          onClick={() => setExpiredTime(0, 0, 0, 0)}
                        >
                          {t('永不过期')}
                        </Button>
                        <Button
                          theme='light'
                          type='tertiary'
                          onClick={() => setExpiredTime(1, 0, 0, 0)}
                        >
                          {t('一个月')}
                        </Button>
                        <Button
                          theme='light'
                          type='tertiary'
                          onClick={() => setExpiredTime(0, 1, 0, 0)}
                        >
                          {t('一天')}
                        </Button>
                        <Button
                          theme='light'
                          type='tertiary'
                          onClick={() => setExpiredTime(0, 0, 1, 0)}
                        >
                          {t('一小时')}
                        </Button>
                      </Space>
                    </Form.Slot>
                  </Col>
                  {!isEdit && (
                    <Col span={24}>
                      <Form.InputNumber
                        field='tokenCount'
                        label={t('新建数量')}
                        min={1}
                        extraText={t('批量创建时会在名称后自动添加随机后缀')}
                        rules={[
                          { required: true, message: t('请输入新建数量') },
                        ]}
                        style={{ width: '100%' }}
                      />
                    </Col>
                  )}
                </Row>
              </Card>

              <Card className='!rounded-2xl shadow-sm border-0'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='green' className='mr-2 shadow-md'>
                    <IconCreditCard size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>{t('额度设置')}</Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置令牌可用额度和数量')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={24}>
                    <Form.InputNumber
                      field='remain_amount'
                      label={t('金额')}
                      prefix={getCurrencyConfig().symbol}
                      placeholder={t('输入金额')}
                      precision={6}
                      disabled={values.unlimited_quota}
                      min={0}
                      step={0.000001}
                      onChange={(val) => {
                        const amount = val === '' || val == null ? 0 : val;
                        formApiRef.current?.setValue('remain_amount', amount);
                        formApiRef.current?.setValue(
                          'remain_quota',
                          displayAmountToQuota(amount),
                        );
                      }}
                      style={{ width: '100%' }}
                      showClear
                    />
                  </Col>
                  <Col span={24}>
                    <div
                      className='text-xs cursor-pointer mt-1'
                      style={{ color: 'var(--semi-color-text-2)' }}
                      onClick={() => setShowQuotaInput((v) => !v)}
                    >
                      {showQuotaInput
                        ? `▾ ${t('收起原生额度输入')}`
                        : `▸ ${t('使用原生额度输入')}`}
                    </div>
                    <div
                      style={{ display: showQuotaInput ? 'block' : 'none' }}
                      className='mt-2'
                    >
                      <Form.InputNumber
                        field='remain_quota'
                        label={t('额度')}
                        placeholder={t('输入额度')}
                        disabled={values.unlimited_quota}
                        min={0}
                        step={500000}
                        rules={
                          values.unlimited_quota
                            ? []
                            : [{ required: true, message: t('请输入额度') }]
                        }
                        onChange={(val) => {
                          const quota = val === '' || val == null ? 0 : val;
                          formApiRef.current?.setValue('remain_quota', quota);
                          formApiRef.current?.setValue(
                            'remain_amount',
                            Number(quotaToDisplayAmount(quota).toFixed(6)),
                          );
                        }}
                        style={{ width: '100%' }}
                        showClear
                      />
                    </div>
                  </Col>
                  <Col span={24}>
                    <Form.Switch
                      field='unlimited_quota'
                      label={t('无限额度')}
                      size='default'
                      extraText={t(
                        '令牌的额度仅用于限制令牌本身的最大额度使用量，实际的使用受到账户的剩余额度限制',
                      )}
                    />
                  </Col>
                </Row>
              </Card>

              <Card className='!rounded-2xl shadow-sm border-0'>
                <div className='flex items-center mb-2'>
                  <Avatar
                    size='small'
                    color='purple'
                    className='mr-2 shadow-md'
                  >
                    <IconLink size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>{t('访问限制')}</Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置令牌的访问限制')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={24}>
                    <Form.Select
                      field='model_limits'
                      label={t('模型限制列表')}
                      placeholder={t(
                        '请选择该令牌支持的模型，留空支持所有模型',
                      )}
                      multiple
                      optionList={models}
                      extraText={t('非必要，不建议启用模型限制')}
                      filter={selectFilter}
                      autoClearSearchValue={false}
                      searchPosition='dropdown'
                      showClear
                      style={{ width: '100%' }}
                    />
                  </Col>
                  <Col span={24}>
                    <Form.TextArea
                      field='allow_ips'
                      label={t('IP白名单（支持CIDR表达式）')}
                      placeholder={t('允许的IP，一行一个，不填写则不限制')}
                      autosize
                      rows={1}
                      extraText={t(
                        '请勿过度信任此功能，IP可能被伪造，请配合nginx和cdn等网关使用',
                      )}
                      showClear
                      style={{ width: '100%' }}
                    />
                  </Col>
                </Row>
              </Card>
            </div>
          )}
        </Form>
      </Spin>
    </SideSheet>
  );
}

const UserTokensModal = ({ visible, onCancel, user, t, onSuccess }) => {
  const isMobile = useIsMobile();
  const [statusState] = useContext(StatusContext);
  const [loading, setLoading] = useState(false);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [tokens, setTokens] = useState([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [actionLoadingId, setActionLoadingId] = useState(null);
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);
  const [editorVisible, setEditorVisible] = useState(false);
  const [editingToken, setEditingToken] = useState(null);
  const defaultUseAutoGroup =
    statusState?.status?.default_use_auto_group === true;

  const loadTokens = async (page = currentPage) => {
    if (!user?.id) return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/${user.id}/tokens?p=${page}&size=${PAGE_SIZE}`,
      );
      if (res.data?.success) {
        const data = res.data.data || {};
        setTokens(data.items || []);
        setTotal(data.total || 0);
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  const loadTokenOptions = async () => {
    setOptionsLoading(true);
    try {
      const [modelsRes, groupsRes] = await Promise.all([
        API.get(`/api/user/${user.id}/models`),
        API.get(`/api/user/${user.id}/groups`),
      ]);

      if (modelsRes.data?.success) {
        const categories = getModelCategories(t);
        const modelOptions = (modelsRes.data.data || []).map((model) => {
          let icon = null;
          for (const [key, category] of Object.entries(categories)) {
            if (key !== 'all' && category.filter({ model_name: model })) {
              icon = category.icon;
              break;
            }
          }
          return {
            label: (
              <span className='flex items-center gap-1'>
                {icon}
                {model}
              </span>
            ),
            value: model,
          };
        });
        setModels(modelOptions);
      } else {
        showError(modelsRes.data?.message || t('加载模型失败'));
      }

      if (groupsRes.data?.success) {
        let groupOptions = Object.entries(groupsRes.data.data || {}).map(
          ([group, info]) => ({
            label: group,
            desc: info.desc,
            value: group,
            ratio: info.ratio,
          }),
        );
        if (defaultUseAutoGroup) {
          groupOptions = groupOptions.sort((a, b) => {
            if (a.value === 'auto') return -1;
            if (b.value === 'auto') return 1;
            return 0;
          });
        }
        setGroups(groupOptions);
      } else {
        showError(groupsRes.data?.message || t('加载分组失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setOptionsLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      setEditorVisible(false);
      setEditingToken(null);
      return;
    }
    setCurrentPage(1);
    loadTokenOptions();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, user?.id]);

  useEffect(() => {
    if (visible && user?.id) {
      loadTokens();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, currentPage, user?.id]);

  const handlePageChange = (page) => {
    setCurrentPage(page);
  };

  const openCreateEditor = () => {
    setEditingToken(null);
    setEditorVisible(true);
  };

  const openEditEditor = (record) => {
    setEditingToken(record);
    setEditorVisible(true);
  };

  const handleEditorSuccess = (action) => {
    setEditorVisible(false);
    setEditingToken(null);
    if (action === 'create' && currentPage !== 1) {
      setCurrentPage(1);
    } else {
      loadTokens();
    }
    onSuccess?.();
  };

  const handleToggleStatus = async (record) => {
    setActionLoadingId(record.id);
    try {
      const nextStatus =
        record.status === API_KEY_STATUS_ENABLED
          ? API_KEY_STATUS_DISABLED
          : API_KEY_STATUS_ENABLED;
      const res = await API.put(
        `/api/user/${user.id}/tokens/${record.id}?status_only=true`,
        { status: nextStatus },
      );
      if (res.data?.success) {
        showSuccess(
          record.status === API_KEY_STATUS_ENABLED
            ? t('令牌已禁用')
            : t('令牌已启用'),
        );
        loadTokens();
      } else {
        showError(res.data?.message || t('操作失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setActionLoadingId(null);
    }
  };

  const handleDelete = async (record) => {
    setActionLoadingId(record.id);
    try {
      const res = await API.delete(`/api/user/${user.id}/tokens/${record.id}`);
      if (res.data?.success) {
        showSuccess(t('令牌已删除'));
        // 如果当前页只剩一条且不是第一页，回退一页
        if (tokens.length === 1 && currentPage > 1) {
          setCurrentPage(currentPage - 1);
        } else {
          loadTokens();
        }
        onSuccess?.();
      } else {
        showError(res.data?.message || t('删除失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setActionLoadingId(null);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('名称'),
        dataIndex: 'name',
        width: 160,
        render: (text, record) => (
          <Space>
            <Text strong>{text || '-'}</Text>
            <Tag color={getStatusTag(record.status, t).color} size='small'>
              {getStatusTag(record.status, t).text}
            </Tag>
          </Space>
        ),
      },
      {
        title: 'Key',
        dataIndex: 'key',
        width: 200,
        render: (text) => (
          <Text
            copyable
            ellipsis={{ showTooltip: true }}
            style={{ maxWidth: 180 }}
          >
            {text ? `sk-${text}` : '-'}
          </Text>
        ),
      },
      {
        title: t('分组'),
        dataIndex: 'group',
        width: 100,
        render: (text) => text || t('默认'),
      },
      {
        title: t('剩余额度'),
        dataIndex: 'remain_quota',
        width: 120,
        render: (val, record) =>
          record.unlimited_quota ? t('无限') : renderQuota(val),
      },
      {
        title: t('已用额度'),
        dataIndex: 'used_quota',
        width: 120,
        render: (val) => renderQuota(val),
      },
      {
        title: t('过期时间'),
        dataIndex: 'expired_time',
        width: 160,
        render: (val) => formatExpiredTime(val, t),
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_time',
        width: 160,
        render: (val) => (val ? timestamp2string(val) : '-'),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        fixed: 'right',
        width: 210,
        render: (_, record) => (
          <Space>
            <Popconfirm
              title={t('确认操作')}
              content={
                record.status === API_KEY_STATUS_ENABLED
                  ? t('确定要禁用此令牌吗？')
                  : t('确定要启用此令牌吗？')
              }
              onConfirm={() => handleToggleStatus(record)}
            >
              <Button
                type={
                  record.status === API_KEY_STATUS_ENABLED
                    ? 'danger'
                    : 'primary'
                }
                size='small'
                loading={actionLoadingId === record.id}
              >
                {record.status === API_KEY_STATUS_ENABLED
                  ? t('禁用')
                  : t('启用')}
              </Button>
            </Popconfirm>
            <Button
              type='tertiary'
              size='small'
              icon={<IconEdit />}
              onClick={() => openEditEditor(record)}
            >
              {t('编辑')}
            </Button>
            <Popconfirm
              title={t('确认删除')}
              content={t('确定要删除此令牌吗？此操作不可撤销。')}
              onConfirm={() => handleDelete(record)}
            >
              <Button
                type='danger'
                size='small'
                loading={actionLoadingId === record.id}
              >
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [tokens, actionLoadingId, currentPage, t],
  );

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 1100}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('管理')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {t('用户令牌管理')}
          </Typography.Title>
          <Text type='tertiary' className='ml-2'>
            {user?.username || '-'} (ID: {user?.id || '-'})
          </Text>
        </Space>
      }
    >
      <div className='p-4'>
        <div className='mb-3 flex justify-between items-center gap-2'>
          <Text type='tertiary'>
            {t('总计')}: {total}
          </Text>
          <Space>
            <Button
              theme='light'
              type='tertiary'
              icon={<IconRefresh />}
              loading={loading}
              onClick={() => loadTokens()}
            >
              {t('刷新')}
            </Button>
            <Button
              theme='solid'
              type='primary'
              icon={<IconPlus />}
              onClick={openCreateEditor}
            >
              {t('添加令牌')}
            </Button>
          </Space>
        </div>
        <CardTable
          columns={columns}
          dataSource={tokens}
          rowKey='id'
          loading={loading}
          scroll={{ x: 'max-content' }}
          hidePagination={false}
          pagination={{
            currentPage,
            pageSize: PAGE_SIZE,
            total,
            pageSizeOpts: [10, 20, 50],
            showSizeChanger: false,
            onPageChange: handlePageChange,
          }}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无令牌')}
              style={{ padding: 30 }}
              footer={
                <Button
                  theme='solid'
                  type='primary'
                  icon={<IconPlus />}
                  onClick={openCreateEditor}
                >
                  {t('添加令牌')}
                </Button>
              }
            />
          }
          size='middle'
        />
      </div>
      <UserTokenEditor
        visible={editorVisible}
        user={user}
        token={editingToken}
        t={t}
        models={models}
        groups={groups}
        optionsLoading={optionsLoading}
        defaultUseAutoGroup={defaultUseAutoGroup}
        onCancel={() => {
          setEditorVisible(false);
          setEditingToken(null);
        }}
        onSuccess={handleEditorSuccess}
      />
    </SideSheet>
  );
};

export default UserTokensModal;
