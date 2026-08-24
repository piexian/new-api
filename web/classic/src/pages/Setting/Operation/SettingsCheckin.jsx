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
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsCheckin(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'checkin_setting.enabled': false,
    'checkin_setting.min_quota': 1000,
    'checkin_setting.max_quota': 10000,
    'checkin_setting.special_enabled': false,
    'checkin_setting.special_weekday': '7',
    'checkin_setting.special_quota': 20000,
    'checkin_setting.client_check_enabled': false,
    'checkin_setting.decay_enabled': false,
    'checkin_setting.decay_rate': 0.85,
    'checkin_setting.decay_floor': 0,
    'checkin_setting.usage_boost_enabled': false,
    'checkin_setting.usage_boost_days': 3,
    'checkin_setting.high_reward_threshold': 0.8,
    'checkin_setting.base_high_probability': 0.05,
    'checkin_setting.boost_max_probability': 0.8,
    'checkin_setting.makeup_enabled': false,
    'checkin_setting.makeup_max_days': 3,
    'checkin_setting.makeup_counts_toward_progress': false,
    'checkin_setting.risk_watch_enabled': false,
    'checkin_setting.risk_watch_days': 14,
    'checkin_setting.risk_min_daily_calls': 1,
    'checkin_setting.risk_min_daily_quota': 100,
    'checkin_setting.expire_enabled': false,
    'checkin_setting.expire_mode': 'unused',
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((inputs) => ({ ...inputs, [fieldName]: value }));
    };
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = String(inputs[item.key]);
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
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
  }, [props.options]);

  const enabled = inputs['checkin_setting.enabled'];
  const specialEnabled = enabled && inputs['checkin_setting.special_enabled'];
  const decayEnabled = enabled && inputs['checkin_setting.decay_enabled'];
  const boostEnabled = enabled && inputs['checkin_setting.usage_boost_enabled'];
  const makeupEnabled = enabled && inputs['checkin_setting.makeup_enabled'];
  const riskWatchEnabled =
    enabled && inputs['checkin_setting.risk_watch_enabled'];
  const expireEnabled = enabled && inputs['checkin_setting.expire_enabled'];

  const weekdayOptions = [1, 2, 3, 4, 5, 6, 7].map((day) => ({
    value: String(day),
    label: t('星期') + t(['一', '二', '三', '四', '五', '六', '日'][day - 1]),
  }));

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('签到设置')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('签到功能允许用户每日签到获取随机额度奖励')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.enabled'}
                  label={t('启用签到功能')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('checkin_setting.enabled')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.min_quota'}
                  label={t('签到最小额度')}
                  placeholder={t('签到奖励的最小额度')}
                  onChange={handleFieldChange('checkin_setting.min_quota')}
                  min={0}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.max_quota'}
                  label={t('签到最大额度')}
                  placeholder={t('签到奖励的最大额度')}
                  onChange={handleFieldChange('checkin_setting.max_quota')}
                  min={0}
                  disabled={!enabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('特殊星期奖励')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('指定星期几签到时发放固定额度奖励')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.special_enabled'}
                  label={t('启用特殊星期奖励')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'checkin_setting.special_enabled',
                  )}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  field={'checkin_setting.special_weekday'}
                  label={t('特殊星期')}
                  optionList={weekdayOptions}
                  onChange={handleFieldChange(
                    'checkin_setting.special_weekday',
                  )}
                  disabled={!specialEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.special_quota'}
                  label={t('特殊签到额度')}
                  placeholder={t('特殊星期发放的固定额度')}
                  onChange={handleFieldChange('checkin_setting.special_quota')}
                  min={0}
                  disabled={!specialEnabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('反脚本检测')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('开启后签到时进行客户端环境检测，防止脚本自动签到')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.client_check_enabled'}
                  label={t('启用客户端检测')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'checkin_setting.client_check_enabled',
                  )}
                  disabled={!enabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('保底衰减')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('连续签到时保底额度按比例逐日衰减')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.decay_enabled'}
                  label={t('启用保底衰减')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('checkin_setting.decay_enabled')}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.decay_rate'}
                  label={t('衰减比例')}
                  placeholder={t('每日保底乘以该比例，范围 0-1')}
                  onChange={handleFieldChange('checkin_setting.decay_rate')}
                  min={0}
                  max={1}
                  step={0.01}
                  disabled={!decayEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.decay_floor'}
                  label={t('衰减下限')}
                  placeholder={t('保底额度下限，0 表示取签到最小额度')}
                  onChange={handleFieldChange('checkin_setting.decay_floor')}
                  min={0}
                  disabled={!decayEnabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('活跃度加成')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('根据用户近期调用量提高高额度奖励的出现概率')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.usage_boost_enabled'}
                  label={t('启用活跃度加成')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'checkin_setting.usage_boost_enabled',
                  )}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.usage_boost_days'}
                  label={t('统计天数')}
                  placeholder={t('统计最近几天的调用量')}
                  onChange={handleFieldChange(
                    'checkin_setting.usage_boost_days',
                  )}
                  min={1}
                  disabled={!boostEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.high_reward_threshold'}
                  label={t('高奖励阈值')}
                  placeholder={t('额度超过最大额度该比例视为高奖励，范围 0-1')}
                  onChange={handleFieldChange(
                    'checkin_setting.high_reward_threshold',
                  )}
                  min={0}
                  max={1}
                  step={0.01}
                  disabled={!boostEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.base_high_probability'}
                  label={t('基础高奖励概率')}
                  placeholder={t('未加成时的高奖励概率，范围 0-1')}
                  onChange={handleFieldChange(
                    'checkin_setting.base_high_probability',
                  )}
                  min={0}
                  max={1}
                  step={0.01}
                  disabled={!boostEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.boost_max_probability'}
                  label={t('加成概率上限')}
                  placeholder={t('活跃度加成后的概率上限，范围 0-1')}
                  onChange={handleFieldChange(
                    'checkin_setting.boost_max_probability',
                  )}
                  min={0}
                  max={1}
                  step={0.01}
                  disabled={!boostEnabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('补签')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('允许用户补签最近漏签的日期')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.makeup_enabled'}
                  label={t('启用补签')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('checkin_setting.makeup_enabled')}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.makeup_max_days'}
                  label={t('最大补签天数')}
                  placeholder={t('允许补签最近几天的漏签')}
                  onChange={handleFieldChange(
                    'checkin_setting.makeup_max_days',
                  )}
                  min={1}
                  disabled={!makeupEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.makeup_counts_toward_progress'}
                  label={t('补签计入连续天数')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'checkin_setting.makeup_counts_toward_progress',
                  )}
                  disabled={!makeupEnabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('风控联动')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('对连续签到但调用量过低的账号进行风控观察')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.risk_watch_enabled'}
                  label={t('启用风控观察')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'checkin_setting.risk_watch_enabled',
                  )}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.risk_watch_days'}
                  label={t('观察天数')}
                  placeholder={t('统计最近几天的行为')}
                  onChange={handleFieldChange(
                    'checkin_setting.risk_watch_days',
                  )}
                  min={1}
                  disabled={!riskWatchEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.risk_min_daily_calls'}
                  label={t('日均最低调用次数')}
                  onChange={handleFieldChange(
                    'checkin_setting.risk_min_daily_calls',
                  )}
                  min={0}
                  disabled={!riskWatchEnabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.risk_min_daily_quota'}
                  label={t('日均最低消费额度')}
                  onChange={handleFieldChange(
                    'checkin_setting.risk_min_daily_quota',
                  )}
                  min={0}
                  disabled={!riskWatchEnabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('额度过期')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('签到获得的额度可设置为过期回收')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.expire_enabled'}
                  label={t('启用额度过期')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('checkin_setting.expire_enabled')}
                  disabled={!enabled}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  field={'checkin_setting.expire_mode'}
                  label={t('过期模式')}
                  optionList={[
                    { value: 'unused', label: t('仅回收未使用部分') },
                    { value: 'all', label: t('回收全部签到额度') },
                  ]}
                  onChange={handleFieldChange('checkin_setting.expire_mode')}
                  disabled={!expireEnabled}
                />
              </Col>
            </Row>
          </Form.Section>

          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存签到设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
