import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  buildRequestRuleExpr,
  createEmptyRuleGroup,
  createEmptyTimeCondition,
} from '@/features/pricing/lib/billing-expr'
import {
  generateExprFromVisualConfig,
  normalizeVisualConfig,
  normalizeVisualTier,
} from '@/features/pricing/lib/tier-expr'

import {
  editorRowId,
  withEditorRowId,
  withRuleRowIds,
  withVisualRowIds,
} from './tiered-editor-state'

test('tier identity survives editing and deletion without changing the billing payload', () => {
  const parsed = {
    tiers: [
      normalizeVisualTier({
        label: 'short',
        input_unit_cost: 2,
        conditions: [{ var: 'len', op: '<', value: 200000 }],
      }),
      normalizeVisualTier({ label: 'long', input_unit_cost: 4 }),
    ],
  }
  const config = withVisualRowIds(parsed)
  const first = config.tiers[0]
  const second = config.tiers[1]
  assert.ok(editorRowId(first))
  assert.notEqual(editorRowId(first), editorRowId(second))
  assert.equal(JSON.stringify(config), JSON.stringify(parsed))
  assert.equal(
    generateExprFromVisualConfig(config),
    generateExprFromVisualConfig(parsed)
  )

  const edited = withVisualRowIds(
    normalizeVisualConfig({
      ...config,
      tiers: [{ ...first, label: 'renamed' }, second],
    })
  )
  assert.equal(editorRowId(edited.tiers[0]), editorRowId(first))
  const remaining = withVisualRowIds({
    ...edited,
    tiers: edited.tiers.slice(1),
  })
  assert.equal(editorRowId(remaining.tiers[0]), editorRowId(second))
})

test('switching a rule condition source preserves its row identity', () => {
  const original = [createEmptyRuleGroup(), createEmptyRuleGroup()]
  const groups = withRuleRowIds(original)
  assert.equal(JSON.stringify(groups), JSON.stringify(original))
  assert.equal(buildRequestRuleExpr(groups), buildRequestRuleExpr(original))
  const condition = groups[0].conditions[0]
  const replacement = withEditorRowId(createEmptyTimeCondition(), condition)
  assert.equal(editorRowId(replacement), editorRowId(condition))
  const remaining = withRuleRowIds(groups.slice(1))
  assert.equal(editorRowId(remaining[0]), editorRowId(groups[1]))
})
