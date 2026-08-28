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
*/

import { toBoolean } from './boolean';

/**
 * 以页面声明的默认值为基底合并后端选项，供设置页加载时构造表单输入。
 *
 * 必须用默认值兜底而非从空对象拷贝，两个历史事故都源于此：
 * 1. 后端只返回已入库的键（新键首次保存前不存在），从空对象起步会让
 *    开关字段整体消失，渲染成"关"，且 compareObjects 要求键同时存在于
 *    新旧两个对象，缺键导致任何切换都被判为"无修改"。
 * 2. 接口统一返回字符串，"false" 因字符串真值会被显示为"开"，
 *    布尔型默认值的键必须经 toBoolean 还原。
 *
 * @param {Record<string, any>} options 后端选项映射（键 -> 字符串值）
 * @param {Record<string, any>} defaults 页面声明的默认值（含类型信息）
 * @returns {Record<string, any>} 合并后的表单输入
 */
export function mergeOptionInputs(options, defaults) {
  const result = structuredClone(defaults);
  if (!options) return result;
  for (const key of Object.keys(result)) {
    if (!(key in options)) continue;
    result[key] =
      typeof defaults[key] === 'boolean'
        ? toBoolean(options[key])
        : options[key];
  }
  return result;
}
