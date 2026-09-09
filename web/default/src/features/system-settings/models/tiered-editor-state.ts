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
import { nanoid } from 'nanoid'

import type { RequestRuleGroup } from '@/features/pricing/lib/billing-expr'
import type { VisualConfig } from '@/features/pricing/lib/tier-expr'

// Symbols survive editor object spreads but never enter JSON or billing expressions.
const EDITOR_ROW_ID = Symbol('editor-row-id')

export function editorRowId(row: object): string | undefined {
  return (row as { [EDITOR_ROW_ID]?: string })[EDITOR_ROW_ID]
}

export function withEditorRowId<T extends object>(
  row: T,
  previous: object = row
): T {
  return { ...row, [EDITOR_ROW_ID]: editorRowId(previous) ?? nanoid() }
}

export function withVisualRowIds(config: VisualConfig): VisualConfig {
  return {
    ...config,
    tiers: config.tiers.map((tier) =>
      withEditorRowId({
        ...tier,
        conditions: tier.conditions.map((condition) =>
          withEditorRowId(condition)
        ),
      })
    ),
  }
}

export function withRuleRowIds(groups: RequestRuleGroup[]): RequestRuleGroup[] {
  return groups.map((group) =>
    withEditorRowId({
      ...group,
      conditions: group.conditions.map((condition) =>
        withEditorRowId(condition)
      ),
    })
  )
}
