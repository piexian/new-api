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

import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

// 以 quota 存储、界面按美元编辑的字段
const USD_QUOTA_FIELDS = ['max_total', 'max_per_borrow', 'ai_max_limit'];

// ai_models 后端以 JSON 字符串存储，解析失败时回退为空列表
function parseAiModels(raw) {
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((item) => item && typeof item === 'object' && 'model' in item)
      .map((item) => ({
        model: String(item.model ?? ''),
        context_window: Number(item.context_window) || 0,
      }));
  } catch {
    return [];
  }
}

export default function SettingsLoan(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'loan_setting.enabled': false,
    'loan_setting.max_total': '2500000',
    'loan_setting.daily_rate': '0.001',
    'loan_setting.repay_fee_rate': '0.0001',
    'loan_setting.min_register_days': '0',
    'loan_setting.max_per_borrow': '0',
    'loan_setting.checkin_repay_enabled': true,
    'loan_setting.terms_enabled': true,
    'loan_setting.terms_text': '',
    'loan_setting.ai_enabled': false,
    'loan_setting.ai_models': '[]',
    'loan_setting.ai_max_limit': '10000000',
    'loan_setting.ai_min_rate': '0.0005',
    'loan_setting.ai_max_grace_days': '30',
    'loan_setting.ai_max_active_applications': '1',
    'loan_setting.ai_daily_limit': '3',
    'loan_setting.ai_max_rounds': '10',
    'loan_setting.ai_max_output': '2048',
    'loan_setting.ai_prompt': '',
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);
  // USD 金额输入的本地状态，保存时换算回 quota
  const [usdInputs, setUsdInputs] = useState({
    max_total: 0,
    max_per_borrow: 0,
    ai_max_limit: 0,
  });
  const [aiModelRows, setAiModelRows] = useState([]);

  // quota_per_unit 取自 localStorage，可能缺失；缺失时禁用 USD 输入并提示，
  // 避免按错误汇率换算出错误 quota（选择禁用换算输入，而非回退为 quota 直填）
  const rawQuotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  const quotaPerUnit =
    Number.isFinite(rawQuotaPerUnit) && rawQuotaPerUnit > 0
      ? rawQuotaPerUnit
      : null;

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((inputs) => ({ ...inputs, [fieldName]: value }));
    };
  }

  function handleUsdChange(name) {
    return (value) => {
      setUsdInputs((prev) => ({ ...prev, [name]: value }));
    };
  }

  function syncAiModelRows(rows) {
    setAiModelRows(rows);
    setInputs((inputs) => ({
      ...inputs,
      // 序列化时过滤 model 为空的行（trim 后），model 名 trim 存储，
      // context_window 取整；与 default 主题行为一致
      'loan_setting.ai_models': JSON.stringify(
        rows
          .filter((row) => row.model && row.model.trim() !== '')
          .map((row) => ({
            model: row.model.trim(),
            context_window: Math.round(Number(row.context_window)) || 0,
          })),
      ),
    }));
  }

  function handleAiModelChange(index, field, value) {
    const rows = aiModelRows.map((row, i) =>
      i === index ? { ...row, [field]: value } : row,
    );
    syncAiModelRows(rows);
  }

  function renderUsdInput(name, description) {
    const quotaHint =
      quotaPerUnit !== null
        ? t('约合额度 {{quota}}', {
            quota: Math.round(
              (Number(usdInputs[name]) || 0) * quotaPerUnit,
            ).toLocaleString(),
          })
        : '';
    return (
      <>
        <InputNumber
          value={usdInputs[name]}
          onChange={handleUsdChange(name)}
          min={0}
          step='any'
          style={{ width: '100%' }}
          disabled={!inputs['loan_setting.enabled'] || quotaPerUnit === null}
        />
        <Typography.Text type='tertiary' size='small'>
          {description}
          {quotaHint ? `（${quotaHint}）` : ''}
        </Typography.Text>
      </>
    );
  }

  function onSubmit() {
    // 保存期校验：AI 可批准的最低日利率不得高于全局日利率，否则审批边界自相矛盾
    const dailyRate = parseFloat(inputs['loan_setting.daily_rate']);
    const aiMinRate = parseFloat(inputs['loan_setting.ai_min_rate']);
    if (
      Number.isFinite(dailyRate) &&
      Number.isFinite(aiMinRate) &&
      aiMinRate > dailyRate
    ) {
      return showError(t('AI 最低日利率不能高于全局日利率'));
    }
    // 空行（model trim 后为空）已在序列化时过滤，此处只校验保留行的
    // context_window 必须为大于 0 的整数
    const validRows = aiModelRows.filter(
      (row) => row.model && row.model.trim() !== '',
    );
    for (const row of validRows) {
      const contextWindow = Number(row.context_window);
      if (!Number.isInteger(contextWindow) || contextWindow <= 0) {
        return showError(t('上下文窗口必须为大于 0 的整数'));
      }
    }

    const updateArray = compareObjects(inputs, inputsRow);
    const requestQueue = updateArray.map((item) =>
      API.put('/api/option/', {
        key: item.key,
        value: String(inputs[item.key]),
      }),
    );

    // USD 输入换算回 quota 后单独与原始值比较
    if (quotaPerUnit !== null) {
      USD_QUOTA_FIELDS.forEach((name) => {
        const key = `loan_setting.${name}`;
        const quota = String(
          Math.round((Number(usdInputs[name]) || 0) * quotaPerUnit),
        );
        if (quota !== String(inputsRow[key])) {
          requestQueue.push(API.put('/api/option/', { key, value: quota }));
        }
      });
    }

    if (!requestQueue.length) return showWarning(t('你似乎并没有修改什么'));
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
    setAiModelRows(parseAiModels(currentInputs['loan_setting.ai_models']));
    if (quotaPerUnit !== null) {
      const nextUsd = {};
      USD_QUOTA_FIELDS.forEach((name) => {
        nextUsd[name] =
          (Number(currentInputs[`loan_setting.${name}`]) || 0) / quotaPerUnit;
      });
      setUsdInputs(nextUsd);
    }
  }, [props.options]);

  const enabled = inputs['loan_setting.enabled'];
  const aiEnabled = enabled && inputs['loan_setting.ai_enabled'];
  const termsEnabled = enabled && inputs['loan_setting.terms_enabled'];

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('词元贷设置')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('词元贷允许用户借用额度并在之后归还，可配置利率与 AI 信贷员')}
            </Typography.Text>
            {quotaPerUnit === null && (
              <Typography.Text
                type='danger'
                style={{ marginBottom: 16, display: 'block' }}
              >
                {t(
                  '未获取到额度汇率（quota_per_unit），美元金额输入已禁用，请先在通用设置中完成配置',
                )}
              </Typography.Text>
            )}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'loan_setting.enabled'}
                  label={t('启用词元贷')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('loan_setting.enabled')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'loan_setting.checkin_repay_enabled'}
                  label={t('签到自动还款')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'loan_setting.checkin_repay_enabled',
                  )}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'loan_setting.terms_enabled'}
                  label={t('要求确认借款条款')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('loan_setting.terms_enabled')}
                  disabled={!enabled}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Typography.Text strong>
                  {t('借款总额度上限（美元）')}
                </Typography.Text>
                {renderUsdInput('max_total', t('用户未偿还借款的总额度上限'))}
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Typography.Text strong>
                  {t('单次借款上限（美元）')}
                </Typography.Text>
                {renderUsdInput(
                  'max_per_borrow',
                  t('单次借款额度上限，填 0 表示跟随借款总额度上限'),
                )}
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'loan_setting.daily_rate'}
                  label={t('日利率')}
                  placeholder={'0.001'}
                  onChange={handleFieldChange('loan_setting.daily_rate')}
                  min={0}
                  step={0.0001}
                  disabled={!enabled}
                  extraText={t('每日复利利率，例如 0.001 表示每天 0.1%')}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'loan_setting.repay_fee_rate'}
                  label={t('提前还款手续费率')}
                  placeholder={'0.0001'}
                  onChange={handleFieldChange('loan_setting.repay_fee_rate')}
                  min={0}
                  step={0.0001}
                  disabled={!enabled}
                  extraText={t(
                    '手动提前还款按抵本部分收取的手续费率，签到自动还款不收取',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'loan_setting.min_register_days'}
                  label={t('最小注册天数')}
                  placeholder={'0'}
                  onChange={handleFieldChange('loan_setting.min_register_days')}
                  min={0}
                  precision={0}
                  disabled={!enabled}
                  extraText={t('账号注册满该天数后才可借款')}
                />
              </Col>
            </Row>
            {/* 条件字段保持挂载、仅隐藏，否则异步选项加载后后挂载的字段拿不到表单值 */}
            <div style={termsEnabled ? undefined : { display: 'none' }}>
              <Row gutter={16}>
                <Col span={24}>
                  <Form.TextArea
                    field={'loan_setting.terms_text'}
                    label={t('借款条款内容')}
                    placeholder={t('用户首次借款前展示的条款内容')}
                    rows={5}
                    onChange={handleFieldChange('loan_setting.terms_text')}
                  />
                </Col>
              </Row>
            </div>
          </Form.Section>

          <Form.Section text={t('AI 信贷员')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('允许用户与 AI 信贷员协商额度、利率与免息期')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'loan_setting.ai_enabled'}
                  label={t('启用 AI 信贷员')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('loan_setting.ai_enabled')}
                  disabled={!enabled}
                />
              </Col>
            </Row>
            {/* 同上：AI 字段保持挂载，仅按开关隐藏 */}
            <div style={aiEnabled ? undefined : { display: 'none' }}>
              <Row gutter={16} style={{ marginTop: 8 }}>
                <Col span={24}>
                  <Typography.Text strong>{t('AI 信贷员模型')}</Typography.Text>
                  <div style={{ marginTop: 8 }}>
                    {aiModelRows.map((row, index) => (
                      <div
                        key={index}
                        style={{
                          display: 'flex',
                          gap: 8,
                          marginBottom: 8,
                          alignItems: 'center',
                        }}
                      >
                        <Input
                          value={row.model}
                          placeholder={t('模型名称')}
                          style={{ flex: 1 }}
                          onChange={(value) =>
                            handleAiModelChange(index, 'model', value)
                          }
                        />
                        <InputNumber
                          value={row.context_window}
                          placeholder={t('上下文窗口')}
                          min={1}
                          precision={0}
                          style={{ width: 160 }}
                          onChange={(value) =>
                            handleAiModelChange(index, 'context_window', value)
                          }
                        />
                        <Button
                          type='danger'
                          theme='borderless'
                          icon={<IconDelete />}
                          onClick={() =>
                            syncAiModelRows(
                              aiModelRows.filter((_, i) => i !== index),
                            )
                          }
                        />
                      </div>
                    ))}
                    <Button
                      icon={<IconPlus />}
                      onClick={() =>
                        syncAiModelRows([
                          ...aiModelRows,
                          { model: '', context_window: 0 },
                        ])
                      }
                    >
                      {t('添加模型')}
                    </Button>
                    <div style={{ marginTop: 4 }}>
                      <Typography.Text type='tertiary' size='small'>
                        {t('可供 AI 信贷员使用的模型，上下文窗口单位为 tokens')}
                      </Typography.Text>
                    </div>
                  </div>
                </Col>
              </Row>
              <Row gutter={16} style={{ marginTop: 8 }}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Typography.Text strong>
                    {t('AI 可批准额度上限（美元）')}
                  </Typography.Text>
                  {renderUsdInput(
                    'ai_max_limit',
                    t('AI 信贷员可批准的最高额度'),
                  )}
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    field={'loan_setting.ai_min_rate'}
                    label={t('AI 最低日利率')}
                    placeholder={'0.0005'}
                    onChange={handleFieldChange('loan_setting.ai_min_rate')}
                    min={0}
                    step={0.0001}
                    extraText={t('AI 信贷员可批准的最低日利率')}
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    field={'loan_setting.ai_max_grace_days'}
                    label={t('AI 最长免息天数')}
                    placeholder={'30'}
                    onChange={handleFieldChange(
                      'loan_setting.ai_max_grace_days',
                    )}
                    min={0}
                    precision={0}
                    extraText={t('AI 信贷员可授予的最长免息期')}
                  />
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    field={'loan_setting.ai_max_active_applications'}
                    label={t('每用户最大进行中申请数')}
                    placeholder={'1'}
                    onChange={handleFieldChange(
                      'loan_setting.ai_max_active_applications',
                    )}
                    min={0}
                    precision={0}
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    field={'loan_setting.ai_daily_limit'}
                    label={t('每用户每日申请上限')}
                    placeholder={'3'}
                    onChange={handleFieldChange('loan_setting.ai_daily_limit')}
                    min={0}
                    precision={0}
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    field={'loan_setting.ai_max_rounds'}
                    label={t('最大对话轮数')}
                    placeholder={'10'}
                    onChange={handleFieldChange('loan_setting.ai_max_rounds')}
                    min={0}
                    precision={0}
                  />
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    field={'loan_setting.ai_max_output'}
                    label={t('最大输出 Tokens')}
                    placeholder={'2048'}
                    onChange={handleFieldChange('loan_setting.ai_max_output')}
                    min={0}
                    precision={0}
                    extraText={t('AI 信贷员单次回复的最大输出 tokens')}
                  />
                </Col>
              </Row>
              <Row gutter={16}>
                <Col span={24}>
                  <Form.TextArea
                    field={'loan_setting.ai_prompt'}
                    label={t('AI 信贷员系统提示词')}
                    placeholder={t('系统提示词模板，请保留硬性边界占位符')}
                    rows={8}
                    onChange={handleFieldChange('loan_setting.ai_prompt')}
                  />
                </Col>
              </Row>
            </div>
          </Form.Section>

          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存词元贷设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
