/*
Copyright (C) 2023-2026 QuantumNous

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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { SORT_OPTIONS, type SortOption } from '../constants'
import type { PricingModel } from '../types'
import { filterAndSortModels, normalizeSortOption, sortModels } from './filters'

function model(
  modelName: string,
  overrides: Partial<PricingModel> = {}
): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 2,
    completion_ratio: 100,
    enable_groups: ['default'],
    ...overrides,
  }
}

const models: PricingModel[] = [
  model('z-token', { model_price: 999, created_time: 30 }),
  model('a-request', {
    quota_type: 1,
    model_price: 3,
    model_ratio: 99,
    created_time: 10,
  }),
  model('c-missing-price', {
    quota_type: 1,
    model_ratio: 88,
    created_time: 20,
  }),
  model('d-zero-price', {
    quota_type: 1,
    model_price: 0,
    model_ratio: 9,
    created_time: 40,
  }),
  model('b-token', {
    model_ratio: 0.5,
    model_price: 0,
    created_time: 50,
  }),
  model('e-equal-price', { created_time: 60 }),
]

const expectedOrders: Record<SortOption, string[]> = {
  [SORT_OPTIONS.NAME]: [
    'a-request',
    'b-token',
    'c-missing-price',
    'd-zero-price',
    'e-equal-price',
    'z-token',
  ],
  [SORT_OPTIONS.PRICE_LOW]: [
    'c-missing-price',
    'd-zero-price',
    'b-token',
    'z-token',
    'e-equal-price',
    'a-request',
  ],
  [SORT_OPTIONS.PRICE_HIGH]: [
    'a-request',
    'z-token',
    'e-equal-price',
    'b-token',
    'c-missing-price',
    'd-zero-price',
  ],
  [SORT_OPTIONS.LISTED_NEWEST]: [
    'e-equal-price',
    'b-token',
    'd-zero-price',
    'z-token',
    'c-missing-price',
    'a-request',
  ],
  [SORT_OPTIONS.LISTED_OLDEST]: [
    'a-request',
    'c-missing-price',
    'z-token',
    'd-zero-price',
    'b-token',
    'e-equal-price',
  ],
}

describe('pricing sort contracts', () => {
  for (const sortBy of Object.values(SORT_OPTIONS)) {
    test(`${sortBy} orders models without changing the input`, () => {
      const input = structuredClone(models)
      const before = structuredClone(input)
      input.forEach((entry) => Object.freeze(entry))
      Object.freeze(input)

      const sorted = sortModels(input, sortBy)

      assert.deepEqual(
        sorted.map((entry) => entry.model_name),
        expectedOrders[sortBy]
      )
      assert.notEqual(sorted, input)
      assert.deepEqual(input, before)
    })
  }

  for (const sortBy of [
    SORT_OPTIONS.LISTED_NEWEST,
    SORT_OPTIONS.LISTED_OLDEST,
  ]) {
    test(`${sortBy} puts unknown dates last and breaks ties by name`, () => {
      const input = [
        model('unknown-zero', { created_time: 0 }),
        model('valid-z', { created_time: 20 }),
        model('unknown-nan', { created_time: Number.NaN }),
        model('unknown-missing'),
        model('valid-old', { created_time: 10 }),
        model('unknown-positive-infinity', {
          created_time: Number.POSITIVE_INFINITY,
        }),
        model('valid-a', { created_time: 20 }),
        model('unknown-negative', { created_time: -1 }),
        model('unknown-negative-infinity', {
          created_time: Number.NEGATIVE_INFINITY,
        }),
      ]
      const validNames =
        sortBy === SORT_OPTIONS.LISTED_NEWEST
          ? ['valid-a', 'valid-z', 'valid-old']
          : ['valid-old', 'valid-a', 'valid-z']
      const expected = [
        ...validNames,
        'unknown-missing',
        'unknown-nan',
        'unknown-negative',
        'unknown-negative-infinity',
        'unknown-positive-infinity',
        'unknown-zero',
      ]

      assert.deepEqual(
        sortModels(input, sortBy).map((entry) => entry.model_name),
        expected
      )
      assert.deepEqual(
        sortModels([...input].reverse(), sortBy).map(
          (entry) => entry.model_name
        ),
        expected
      )
    })
  }

  test('normalizes invalid sort sources to name and retains every valid option', () => {
    for (const value of [undefined, null, '', 'invalid', 'NAME', 0, {}]) {
      assert.equal(normalizeSortOption(value), SORT_OPTIONS.NAME)
    }
    for (const value of Object.values(SORT_OPTIONS)) {
      assert.equal(normalizeSortOption(value), value)
    }
    assert.deepEqual(
      sortModels(models, 'invalid').map((entry) => entry.model_name),
      expectedOrders[SORT_OPTIONS.NAME]
    )
  })
})

describe('pricing filtering followed by sorting', () => {
  const matching = {
    vendor_name: 'Acme',
    enable_groups: ['pro'],
    supported_endpoint_types: ['openai'],
    tags: 'featured,fast',
  }
  const input = [
    model('keep-z', { ...matching, created_time: 10 }),
    model('keep-b', { ...matching, created_time: 20, model_ratio: 0.5 }),
    model('keep-e', { ...matching, created_time: 30 }),
    model('excluded-search', matching),
    model('keep-excluded-vendor', { ...matching, vendor_name: 'Other' }),
    model('keep-excluded-group', { ...matching, enable_groups: ['default'] }),
    model('keep-excluded-quota', { ...matching, quota_type: 1 }),
    model('keep-excluded-endpoint', {
      ...matching,
      supported_endpoint_types: ['anthropic'],
    }),
    model('keep-excluded-tag', { ...matching, tags: 'other' }),
  ]
  const expected: Record<SortOption, string[]> = {
    [SORT_OPTIONS.NAME]: ['keep-b', 'keep-e', 'keep-z'],
    [SORT_OPTIONS.PRICE_LOW]: ['keep-b', 'keep-z', 'keep-e'],
    [SORT_OPTIONS.PRICE_HIGH]: ['keep-z', 'keep-e', 'keep-b'],
    [SORT_OPTIONS.LISTED_NEWEST]: ['keep-e', 'keep-b', 'keep-z'],
    [SORT_OPTIONS.LISTED_OLDEST]: ['keep-z', 'keep-b', 'keep-e'],
  }

  for (const sortBy of Object.values(SORT_OPTIONS)) {
    test(`applies all filters before ${sortBy}`, () => {
      const before = structuredClone(input)
      const sorted = filterAndSortModels(input, {
        search: 'KEEP',
        vendor: 'Acme',
        group: 'pro',
        quotaType: 'token',
        endpointType: 'openai',
        tag: 'FEATURED',
        sortBy,
      })

      assert.deepEqual(
        sorted.map((entry) => entry.model_name),
        expected[sortBy]
      )
      assert.deepEqual(input, before)
    })
  }
})
