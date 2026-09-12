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

import { describe, expect, it } from 'bun:test';
import {
  PRICING_SORT_OPTIONS,
  getPricingSortOptions,
  sortPricingModels,
} from '../pricing-sort';
import { sortModels } from '../../../../default/src/features/pricing/lib/filters';

const models = [
  { model_name: 'delta', quota_type: 0, model_ratio: 3, created_time: 100 },
  { model_name: 'alpha', quota_type: 1, model_price: 2, created_time: 300 },
  { model_name: 'charlie', quota_type: 0, model_ratio: 1, created_time: 200 },
  { model_name: 'bravo', quota_type: 1, model_price: 0.5, created_time: 300 },
];
const names = (items) => items.map((item) => item.model_name);

describe('pricing sorting parity', () => {
  it.each([
    [PRICING_SORT_OPTIONS.NAME, ['alpha', 'bravo', 'charlie', 'delta']],
    [PRICING_SORT_OPTIONS.PRICE_LOW, ['bravo', 'charlie', 'alpha', 'delta']],
    [PRICING_SORT_OPTIONS.PRICE_HIGH, ['delta', 'alpha', 'charlie', 'bravo']],
    [
      PRICING_SORT_OPTIONS.LISTED_NEWEST,
      ['alpha', 'bravo', 'charlie', 'delta'],
    ],
    [
      PRICING_SORT_OPTIONS.LISTED_OLDEST,
      ['delta', 'charlie', 'alpha', 'bravo'],
    ],
  ])('%s matches the default theme', (sortBy, expected) => {
    expect(names(sortPricingModels(models, sortBy))).toEqual(expected);
    expect(sortPricingModels(models, sortBy)).toEqual(
      sortModels(models, sortBy),
    );
  });

  it.each([
    PRICING_SORT_OPTIONS.LISTED_NEWEST,
    PRICING_SORT_OPTIONS.LISTED_OLDEST,
  ])('%s keeps missing and invalid dates last, ordered by name', (sortBy) => {
    const unknown = [
      { model_name: 'missing' },
      { model_name: 'zero', created_time: 0 },
      { model_name: 'negative', created_time: -1 },
      { model_name: 'nan', created_time: NaN },
      { model_name: 'infinite', created_time: Infinity },
      { model_name: 'string', created_time: '900' },
    ];
    const input = [...unknown, ...models];
    const sorted = sortPricingModels(input, sortBy);
    expect(names(sorted.slice(models.length))).toEqual([
      'infinite',
      'missing',
      'nan',
      'negative',
      'string',
      'zero',
    ]);
    expect(sorted).toEqual(sortModels(input, sortBy));
  });

  it.each(Object.values(PRICING_SORT_OPTIONS))(
    '%s leaves cached input unchanged',
    (sortBy) => {
      const input = Object.freeze(
        models.map((model) => Object.freeze({ ...model })),
      );
      const before = [...input];
      const result = sortPricingModels(input, sortBy);
      expect(result).not.toBe(input);
      expect(input).toEqual(before);
    },
  );

  it('sorts the complete filtered set before pagination', () => {
    const filtered = models.filter((model) => model.model_name !== 'bravo');
    const sorted = sortPricingModels(
      filtered,
      PRICING_SORT_OPTIONS.LISTED_NEWEST,
    );
    expect(names(sorted.slice(0, 2))).toEqual(['alpha', 'charlie']);
    expect(names(sorted.slice(2, 4))).toEqual(['delta']);
  });

  it('falls back to name order for an older backend without times', () => {
    const input = [{ model_name: 'z' }, { model_name: 'a' }];
    expect(
      names(sortPricingModels(input, PRICING_SORT_OPTIONS.LISTED_NEWEST)),
    ).toEqual(['a', 'z']);
    expect(
      names(sortPricingModels(input, PRICING_SORT_OPTIONS.LISTED_OLDEST)),
    ).toEqual(['a', 'z']);
  });

  it('offers all five sort modes in the menu', () => {
    expect(
      getPricingSortOptions((key) => key).map((option) => option.value),
    ).toEqual(Object.values(PRICING_SORT_OPTIONS));
  });
});
