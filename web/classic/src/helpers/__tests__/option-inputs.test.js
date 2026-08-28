import { describe, it, expect } from 'bun:test';
import { mergeOptionInputs } from '../option-inputs';
import { compareObjects } from '../compare-objects';

// 回归背景：notify_setting.* 等新键首次保存前后端不返回，设置页曾从空对象
// 拷贝导致开关丢值渲染为"关"，且 compareObjects 因键缺失把任何切换判为
// "无修改"；同时接口返回的字符串 "false" 因字符串真值被显示为"开"。
const DEFAULTS = {
  'a_setting.enabled': false,
  'a_setting.name': 'preset',
  'a_setting.ratio': 0.85,
  'notify_setting.channel_auto_disabled': true,
  'notify_setting.channel_test_result': true,
};

describe('mergeOptionInputs', () => {
  it('后端未返回的键保留页面默认值（新键首次保存前）', () => {
    const merged = mergeOptionInputs(
      { 'a_setting.enabled': 'true', 'a_setting.name': 'db' },
      DEFAULTS,
    );
    expect(merged['notify_setting.channel_auto_disabled']).toBe(true);
    expect(merged['notify_setting.channel_test_result']).toBe(true);
    expect(merged['a_setting.enabled']).toBe(true);
    expect(merged['a_setting.name']).toBe('db');
  });

  it('布尔键还原字符串：还原 "false" 必须为 false（字符串真值回归）', () => {
    const merged = mergeOptionInputs(
      {
        'notify_setting.channel_auto_disabled': 'false',
        'notify_setting.channel_test_result': 'true',
      },
      DEFAULTS,
    );
    expect(merged['notify_setting.channel_auto_disabled']).toBe(false);
    expect(merged['notify_setting.channel_test_result']).toBe(true);
  });

  it('布尔键接受原生布尔（父组件已转换时幂等）', () => {
    const merged = mergeOptionInputs(
      { 'notify_setting.channel_auto_disabled': false },
      DEFAULTS,
    );
    expect(merged['notify_setting.channel_auto_disabled']).toBe(false);
  });

  it('options 为空时整体回退默认值', () => {
    const merged = mergeOptionInputs(undefined, DEFAULTS);
    expect(merged).toEqual(DEFAULTS);
  });

  it('返回值与默认值深隔离，不污染常量表', () => {
    const merged = mergeOptionInputs({}, DEFAULTS);
    merged['a_setting.name'] = 'mutated';
    expect(DEFAULTS['a_setting.name']).toBe('preset');
  });

  it('合并后切换开关能被 compareObjects 检出（保存回归）', () => {
    const inputs = mergeOptionInputs(
      { 'notify_setting.channel_auto_disabled': 'true' },
      DEFAULTS,
    );
    const inputsRow = structuredClone(inputs);
    const toggled = {
      ...inputs,
      'notify_setting.channel_auto_disabled': false,
    };
    const diff = compareObjects(toggled, inputsRow);
    expect(diff).toHaveLength(1);
    expect(diff[0].key).toBe('notify_setting.channel_auto_disabled');
  });
});

describe('compareObjects 契约', () => {
  it('键只存在于单侧时不计为变更（缺键即漏报，调用方须保证双侧同构）', () => {
    const withExtra = { ...DEFAULTS, 'new_setting.unlisted_key': false };
    expect(compareObjects(withExtra, DEFAULTS)).toHaveLength(0);
    expect(compareObjects(DEFAULTS, withExtra)).toHaveLength(0);
  });

  it('严格比较：布尔 true 与字符串 "true" 视为不同', () => {
    const diff = compareObjects(
      { k: true },
      { k: 'true' },
    );
    expect(diff).toHaveLength(1);
  });
});
