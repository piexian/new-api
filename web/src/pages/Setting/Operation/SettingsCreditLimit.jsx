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
import { Button, Col, Form, Row, Spin, Tag } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';

export default function SettingsCreditLimit(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [plansLoading, setPlansLoading] = useState(false);
  const [subscriptionPlanOptions, setSubscriptionPlanOptions] = useState([]);
  const [selectedDefaultPlanIds, setSelectedDefaultPlanIds] = useState([]);
  const [inputs, setInputs] = useState({
    QuotaForNewUser: '',
    DefaultSubscriptionPlans: '[]',
    PreConsumedQuota: '',
    QuotaForInviter: '',
    QuotaForInvitee: '',
    'quota_setting.enable_free_model_pre_consume': true,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  const parseDefaultSubscriptionPlans = (value) => {
    const raw = String(value || '').trim();
    if (!raw) {
      return [];
    }
    try {
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) {
        return [];
      }
      return parsed
        .map((item) => Number(item?.plan_id || 0))
        .filter((planId) => Number.isInteger(planId) && planId > 0);
    } catch {
      return [];
    }
  };

  const serializeDefaultSubscriptionPlans = (planIds) =>
    JSON.stringify(
      Array.from(
        new Set(
          (planIds || [])
            .map((planId) => Number(planId))
            .filter((planId) => Number.isInteger(planId) && planId > 0),
        ),
      ).map((planId) => ({ plan_id: planId })),
    );

  const loadSubscriptionPlans = async () => {
    setPlansLoading(true);
    try {
      const res = await API.get('/api/subscription/admin/plans');
      if (res.data?.success) {
        const optionList = (res.data.data || []).map((item) => {
          const plan = item?.plan || {};
          const priceAmount = Number(plan.price_amount || 0);
          return {
            value: plan.id,
            label:
              plan.title && plan.title.trim()
                ? `${plan.title} (${priceAmount.toFixed(2)} USD)`
                : `#${plan.id} (${priceAmount.toFixed(2)} USD)`,
            disabled: !plan.enabled,
          };
        });
        setSubscriptionPlanOptions(optionList);
      } else {
        showError(res.data?.message || t('订阅套餐加载失败'));
      }
    } catch {
      showError(t('订阅套餐加载失败'));
    } finally {
      setPlansLoading(false);
    }
  };

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
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
    loadSubscriptionPlans();
  }, []);

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    setSelectedDefaultPlanIds(
      parseDefaultSubscriptionPlans(currentInputs.DefaultSubscriptionPlans),
    );
    refForm.current.setValues(currentInputs);
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('额度设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('新用户初始额度')}
                  field={'QuotaForNewUser'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  placeholder={''}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForNewUser: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('请求预扣费额度')}
                  field={'PreConsumedQuota'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={t('请求结束后多退少补')}
                  placeholder={''}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      PreConsumedQuota: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('邀请新用户奖励额度')}
                  field={'QuotaForInviter'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={''}
                  placeholder={t('例如：2000')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForInviter: String(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Col xs={24} sm={12} md={8} lg={8} xl={6}>
                <Form.InputNumber
                  label={t('新用户使用邀请码奖励额度')}
                  field={'QuotaForInvitee'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={''}
                  placeholder={t('例如：1000')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForInvitee: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={24} md={16} lg={16} xl={18}>
                <Form.Select
                  field='DefaultSubscriptionPlans'
                  label={t('新用户默认订阅套餐')}
                  placeholder={t('可选，注册成功后自动发放所选套餐')}
                  optionList={subscriptionPlanOptions}
                  value={selectedDefaultPlanIds}
                  onChange={(value) => {
                    const nextPlanIds = Array.isArray(value) ? value : [];
                    setSelectedDefaultPlanIds(nextPlanIds);
                    setInputs({
                      ...inputs,
                      DefaultSubscriptionPlans:
                        serializeDefaultSubscriptionPlans(nextPlanIds),
                    });
                  }}
                  loading={plansLoading}
                  multiple
                  filter
                  showClear
                  extraText={t(
                    '仅对新创建用户生效，不会给已有用户补发。内部仍以 JSON 数组格式存储。',
                  )}
                />
                {selectedDefaultPlanIds.length > 0 && (
                  <div className='mt-2 flex flex-wrap gap-2'>
                    {selectedDefaultPlanIds.map((planId) => {
                      const matchedPlan = subscriptionPlanOptions.find(
                        (option) => Number(option.value) === Number(planId),
                      );
                      return (
                        <Tag key={planId} color='blue' size='small'>
                          {matchedPlan?.label || `#${planId}`}
                        </Tag>
                      );
                    })}
                  </div>
                )}
              </Col>
            </Row>
            <Row>
              <Col>
                <Form.Switch
                  label={t('对免费模型启用预消耗')}
                  field={'quota_setting.enable_free_model_pre_consume'}
                  extraText={t(
                    '开启后，对免费模型（倍率为0，或者价格为0）的模型也会预消耗额度',
                  )}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'quota_setting.enable_free_model_pre_consume': value,
                    })
                  }
                />
              </Col>
            </Row>

            <Row>
              <Button size='default' onClick={onSubmit}>
                {t('保存额度设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
