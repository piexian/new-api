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

import React from 'react';
import { Button, Form, Radio, RadioGroup } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';

const LogsFilters = ({
  formInitValues,
  setFormApi,
  refresh,
  setShowColumnSelector,
  formApi,
  setLogType,
  loading,
  isAdminUser,
  logScope,
  handleLogScopeChange,
  t,
}) => {
  const isLoginScope = logScope === 'login';
  const logTypeOptions = isLoginScope
    ? [
        { value: '0', label: t('全部') },
        { value: '7', label: t('登录成功') },
        ...(isAdminUser ? [{ value: '8', label: t('安全事件') }] : []),
      ]
    : [
        { value: '0', label: t('全部') },
        { value: '1', label: t('充值') },
        { value: '2', label: t('消费') },
        { value: '3', label: t('管理') },
        { value: '4', label: t('系统') },
        { value: '5', label: t('错误') },
        { value: '6', label: t('退款') },
        { value: '7', label: t('登录') },
        ...(isAdminUser ? [{ value: '8', label: t('安全事件') }] : []),
      ];
  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => setFormApi(api)}
      onSubmit={refresh}
      allowEmpty={true}
      autoComplete='off'
      layout='vertical'
      trigger='change'
      stopValidateWithError={false}
    >
      <div className='flex flex-col gap-2'>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
          {/* 时间选择器 */}
          <div className='col-span-1 lg:col-span-2'>
            <Form.DatePicker
              field='dateRange'
              className='w-full'
              type='dateTimeRange'
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
              pure
              size='small'
              presets={DATE_RANGE_PRESETS.map((preset) => ({
                text: t(preset.text),
                start: preset.start(),
                end: preset.end(),
              }))}
            />
          </div>

          {!isLoginScope && (
            <>
              {/* 其他搜索字段 */}
              <Form.Input
                field='token_name'
                prefix={<IconSearch />}
                placeholder={t('令牌名称')}
                showClear
                pure
                size='small'
              />

              <Form.Input
                field='model_name'
                prefix={<IconSearch />}
                placeholder={t('模型名称')}
                showClear
                pure
                size='small'
              />

              <Form.Input
                field='group'
                prefix={<IconSearch />}
                placeholder={t('分组')}
                showClear
                pure
                size='small'
              />

              <Form.Input
                field='session_id'
                prefix={<IconSearch />}
                placeholder={t('Session ID')}
                showClear
                pure
                size='small'
              />

              <Form.Input
                field='cache_key'
                prefix={<IconSearch />}
                placeholder={t('Cache Key')}
                showClear
                pure
                size='small'
              />
            </>
          )}

          <Form.Input
            field='request_id'
            prefix={<IconSearch />}
            placeholder={t('Request ID')}
            showClear
            pure
            size='small'
          />

          <Form.Input
            field='ip'
            prefix={<IconSearch />}
            placeholder={t('IP 地址')}
            showClear
            pure
            size='small'
          />

          <Form.Input
            field='user_agent'
            prefix={<IconSearch />}
            placeholder={t('User Agent')}
            showClear
            pure
            size='small'
          />

          {isAdminUser && !isLoginScope && (
            <>
              <Form.Input
                field='channel'
                prefix={<IconSearch />}
                placeholder={t('渠道 ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='username'
                prefix={<IconSearch />}
                placeholder={t('用户名称')}
                showClear
                pure
                size='small'
              />
            </>
          )}

          {isAdminUser && isLoginScope && (
            <Form.Input
              field='username'
              prefix={<IconSearch />}
              placeholder={t('用户名称')}
              showClear
              pure
              size='small'
            />
          )}
        </div>

        {/* 操作按钮区域 */}
        <div className='flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3'>
          {/* 查询范围与日志类型选择器 */}
          <div className='w-full sm:w-auto flex flex-col sm:flex-row gap-2'>
            <RadioGroup
              type='button'
              buttonSize='small'
              value={logScope}
              onChange={(e) => handleLogScopeChange(e.target.value)}
            >
              <Radio value='all'>{t('全部日志')}</Radio>
              <Radio value='login'>{t('登录审计')}</Radio>
            </RadioGroup>
            <Form.Select
              field='logType'
              placeholder={t('日志类型')}
              className='w-full sm:w-auto min-w-[120px]'
              showClear
              pure
              onChange={(value) => {
                // 同步统计开关所需的类型状态，再延迟查询让表单值生效
                setLogType(value ? parseInt(value) : 0);
                setTimeout(() => {
                  refresh();
                }, 0);
              }}
              size='small'
            >
              {logTypeOptions.map((option) => (
                <Form.Select.Option key={option.value} value={option.value}>
                  {option.label}
                </Form.Select.Option>
              ))}
            </Form.Select>
          </div>

          <div className='flex gap-2 w-full sm:w-auto justify-end'>
            <Button
              type='tertiary'
              htmlType='submit'
              loading={loading}
              size='small'
            >
              {t('查询')}
            </Button>
            <Button
              type='tertiary'
              onClick={() => {
                if (formApi) {
                  formApi.reset();
                  setLogType(0);
                  setTimeout(() => {
                    refresh();
                  }, 100);
                }
              }}
              size='small'
            >
              {t('重置')}
            </Button>
            {!isLoginScope && (
              <Button
                type='tertiary'
                onClick={() => setShowColumnSelector(true)}
                size='small'
              >
                {t('列设置')}
              </Button>
            )}
          </div>
        </div>
      </div>
    </Form>
  );
};

export default LogsFilters;
