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

import type { UsageLog } from '../data/schema'
import type { LogOtherData } from '../types'
import {
  getLoginAuditCount,
  getLoginAuditMethod,
  getLoginAuditReasonLabel,
  getLoginAuditStageLabel,
  getLoginAuditTarget,
  getLoginAuditUserAgent,
  getLoginAuditWindow,
  getLoginAuthAudit,
  resolveLoginAuditOutcome,
} from './login-audit'

function loginLog(overrides: Partial<UsageLog> = {}): UsageLog {
  return {
    id: 1,
    user_id: 0,
    created_at: 1_700_000_000,
    type: 8,
    content: '',
    username: '',
    token_name: '',
    model_name: '',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 0,
    channel_name: '',
    token_id: 0,
    group: '',
    ip: '',
    user_agent: '',
    other: '',
    request_id: '',
    upstream_request_id: '',
    ...overrides,
  }
}

function otherWith(value: LogOtherData): LogOtherData {
  return value
}

describe('resolveLoginAuditOutcome', () => {
  test('type=7 without auth_audit is a success', () => {
    assert.equal(resolveLoginAuditOutcome(7, null), 'success')
  })

  test('type=7 stays success even with a stray audit payload', () => {
    assert.equal(resolveLoginAuditOutcome(7, { outcome: 'failure' }), 'success')
  })

  test('type=8 without auth_audit is unknown', () => {
    assert.equal(resolveLoginAuditOutcome(8, null), 'unknown')
  })

  test('type=8 never reports success or an invented failure', () => {
    assert.equal(resolveLoginAuditOutcome(8, { outcome: 'success' }), 'unknown')
    assert.equal(resolveLoginAuditOutcome(8, { outcome: '' }), 'unknown')
    assert.equal(resolveLoginAuditOutcome(8, { outcome: 'weird' }), 'unknown')
    assert.equal(resolveLoginAuditOutcome(8, { outcome: 'FAILED' }), 'unknown')
  })

  test('type=8 keeps known non-success outcomes', () => {
    assert.equal(resolveLoginAuditOutcome(8, { outcome: 'blocked' }), 'blocked')
    assert.equal(
      resolveLoginAuditOutcome(8, { outcome: 'challenge' }),
      'challenge'
    )
    assert.equal(resolveLoginAuditOutcome(8, { outcome: 'error' }), 'error')
    assert.equal(
      resolveLoginAuditOutcome(8, { outcome: '  BLOCKED  ' }),
      'blocked'
    )
  })
})

describe('label mapping', () => {
  test('known reason and stage are mapped to readable labels', () => {
    assert.equal(
      getLoginAuditReasonLabel('invalid_credentials'),
      'Invalid credentials'
    )
    assert.equal(
      getLoginAuditStageLabel('two_factor'),
      'Two-factor verification'
    )
  })

  test('unknown snake_case values fall back to a safe label', () => {
    assert.equal(getLoginAuditReasonLabel('brand_new_reason'), 'Unknown reason')
    assert.equal(getLoginAuditStageLabel('brand_new_stage'), 'Unknown stage')
  })

  test('missing values render nothing', () => {
    assert.equal(getLoginAuditReasonLabel(undefined), null)
    assert.equal(getLoginAuditReasonLabel(''), null)
    assert.equal(getLoginAuditStageLabel(undefined), null)
  })
})

describe('getLoginAuditCount', () => {
  test('defaults to a single attempt', () => {
    assert.equal(getLoginAuditCount(null), 1)
    assert.equal(getLoginAuditCount({}), 1)
    assert.equal(getLoginAuditCount({ count: 0 }), 1)
    assert.equal(getLoginAuditCount({ count: -3 }), 1)
    assert.equal(getLoginAuditCount({ count: Number.NaN }), 1)
  })

  test('floors aggregated counts', () => {
    assert.equal(getLoginAuditCount({ count: 5 }), 5)
    assert.equal(getLoginAuditCount({ count: 3.9 }), 3)
  })
})

describe('audit payload access', () => {
  test('getLoginAuthAudit ignores non-object values', () => {
    assert.equal(getLoginAuthAudit(null), null)
    assert.equal(getLoginAuthAudit({}), null)
    assert.deepEqual(
      getLoginAuthAudit({ auth_audit: { outcome: 'blocked' } }),
      { outcome: 'blocked' }
    )
  })

  test('getLoginAuditMethod trims the stored method', () => {
    assert.equal(getLoginAuditMethod({ login_method: '  OAuth  ' }), 'OAuth')
    assert.equal(getLoginAuditMethod({}), '')
    assert.equal(getLoginAuditMethod(null), '')
  })

  test('getLoginAuditWindow drops unusable timestamps', () => {
    assert.deepEqual(
      getLoginAuditWindow({ window_start: 10, window_end: 20 }),
      {
        start: 10,
        end: 20,
      }
    )
    assert.deepEqual(getLoginAuditWindow({ window_start: 0, window_end: -1 }), {
      start: undefined,
      end: undefined,
    })
  })
})

describe('getLoginAuditUserAgent', () => {
  test('prefers the log column and falls back to the audit payload', () => {
    const withTopLevel = loginLog({ user_agent: ' top-level-ua ' })
    assert.equal(
      getLoginAuditUserAgent(
        withTopLevel,
        otherWith({ user_agent: 'audit-ua' })
      ),
      'top-level-ua'
    )

    const emptyTopLevel = loginLog({ user_agent: '' })
    assert.equal(
      getLoginAuditUserAgent(
        emptyTopLevel,
        otherWith({ user_agent: ' audit-ua ' })
      ),
      'audit-ua'
    )
    assert.equal(getLoginAuditUserAgent(emptyTopLevel, otherWith({})), '')
    assert.equal(getLoginAuditUserAgent(loginLog(), null), '')
  })
})

describe('getLoginAuditTarget', () => {
  test('reads the audited account from admin_info', () => {
    assert.deepEqual(
      getLoginAuditTarget({
        admin_info: { target_username: 'alice', target_user_id: 12 },
      }),
      { username: 'alice', userId: '12' }
    )
  })

  test('falls back to the attempted identifier for unknown accounts', () => {
    assert.deepEqual(
      getLoginAuditTarget({
        admin_info: { attempted_identifier: 'ghost@example.com' },
      }),
      { username: 'ghost@example.com', userId: '' }
    )
  })

  test('returns an empty target without audit identity', () => {
    assert.deepEqual(getLoginAuditTarget({}), { username: '', userId: '' })
    assert.deepEqual(getLoginAuditTarget(null), { username: '', userId: '' })
  })
})
