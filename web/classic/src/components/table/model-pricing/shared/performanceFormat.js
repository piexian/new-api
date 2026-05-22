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

export const EMPTY_VALUE = '—';

export const toNumber = (value) => {
  if (value === null || value === undefined || value === '') {
    return null;
  }
  const num = Number(value);
  return Number.isFinite(num) ? num : null;
};

export const average = (values, { includeZero = false } = {}) => {
  const validValues = values.filter(
    (value) => value !== null && (includeZero || value > 0),
  );
  if (validValues.length === 0) {
    return null;
  }
  return (
    validValues.reduce((sum, value) => sum + value, 0) / validValues.length
  );
};

export const formatThroughput = (value) => {
  const num = toNumber(value);
  if (num === null || num <= 0) {
    return EMPTY_VALUE;
  }
  if (num >= 1000) {
    return `${(num / 1000).toFixed(2)}K t/s`;
  }
  return `${num.toFixed(num < 10 ? 2 : 1)} t/s`;
};

export const formatCompactThroughput = (value) =>
  formatThroughput(value).replace(' t/s', 'tps');

export const formatLatency = (value) => {
  const num = toNumber(value);
  if (num === null || num <= 0) {
    return EMPTY_VALUE;
  }
  if (num >= 1000) {
    return `${(num / 1000).toFixed(2)}s`;
  }
  return `${Math.round(num)}ms`;
};

export const formatPercent = (value) => {
  const num = toNumber(value);
  if (num === null || num < 0) {
    return EMPTY_VALUE;
  }
  return `${num.toFixed(2)}%`;
};

export const clampPercent = (value) => {
  const num = toNumber(value);
  if (num === null) {
    return 0;
  }
  return Math.max(0, Math.min(100, num));
};

export const successColor = (value) => {
  const rate = toNumber(value);
  if (rate === null) {
    return '#94a3b8';
  }
  if (rate >= 99.9) {
    return '#16a34a';
  }
  if (rate >= 99) {
    return '#65a30d';
  }
  if (rate >= 95) {
    return '#d97706';
  }
  return '#dc2626';
};
