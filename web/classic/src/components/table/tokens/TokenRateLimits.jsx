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

// 分组 RPM/并发展示：undefined 表示分组信息未知（显示 -）；0 表示后端明确的不限制（显示 ♾️）
export function renderGroupLimitValue(value) {
  if (typeof value === 'number' && value > 0) {
    return Number.isInteger(value) ? value : value.toFixed(1);
  }
  if (value === undefined) {
    return '-';
  }
  return '♾️';
}

export function renderGroupLimits(info, t) {
  const items = [
    { label: 'RPM', value: info?.rpm },
    { label: t('并发'), value: info?.concurrency },
  ];
  return (
    <span
      className='inline-flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs'
      style={{ color: 'var(--semi-color-text-2)' }}
    >
      {items.map(({ label, value }) => (
        <span key={label} className='inline-flex items-center gap-1'>
          <span>{label}</span>
          <span>{renderGroupLimitValue(value)}</span>
        </span>
      ))}
    </span>
  );
}
