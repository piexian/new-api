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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  getKimiCodingPlanUsage,
  getMiniMaxTokenPlanUsage,
  getQwenTokenPlanUsage,
  getZhipuCodingPlanUsage,
  type ChannelPlanUsageResponse,
} from '../../api'
import {
  isKimiCodingPlanChannel,
  isQwenTokenPlanChannel,
} from '../../lib/plan-usage-utils'
import { useChannels } from '../channels-provider'
import {
  ChannelPlanUsageDialog,
  type ChannelPlanUsageKind,
} from './channel-plan-usage-dialog'

type PlanUsageQueryDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function PlanUsageQueryDialog({
  open,
  onOpenChange,
}: PlanUsageQueryDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const [isQuerying, setIsQuerying] = useState(false)
  const [response, setResponse] = useState<ChannelPlanUsageResponse | null>(
    null
  )
  const [currentKeyIndex, setCurrentKeyIndex] = useState(0)

  let kind: ChannelPlanUsageKind = 'zhipu'
  if (currentRow?.type === 35) {
    kind = 'minimax'
  } else if (currentRow && isKimiCodingPlanChannel(currentRow)) {
    kind = 'kimi'
  } else if (currentRow && isQwenTokenPlanChannel(currentRow)) {
    kind = 'qwen'
  }

  const fetchUsage = useCallback(
    async (keyIndex: number) => {
      const row = currentRow
      if (!row) return

      setIsQuerying(true)
      try {
        let res: ChannelPlanUsageResponse
        if (kind === 'minimax') {
          res = await getMiniMaxTokenPlanUsage(row.id, keyIndex)
        } else if (kind === 'kimi') {
          res = await getKimiCodingPlanUsage(row.id, keyIndex)
        } else if (kind === 'qwen') {
          res = await getQwenTokenPlanUsage(row.id, keyIndex)
        } else {
          res = await getZhipuCodingPlanUsage(row.id, keyIndex)
        }
        const resolvedKeyIndex = Number(res.key_index ?? keyIndex)
        setCurrentKeyIndex(
          Number.isFinite(resolvedKeyIndex) ? resolvedKeyIndex : 0
        )
        setResponse(res)
        if (!res.success) {
          toast.error(
            res.message ||
              (kind === 'minimax' || kind === 'qwen'
                ? t('Failed to fetch Token Plan usage')
                : t('Failed to fetch Coding Plan usage'))
          )
        }
      } catch (error) {
        setResponse({
          success: false,
          message:
            error instanceof Error ? error.message : t('Failed to fetch usage'),
        })
        toast.error(
          error instanceof Error ? error.message : t('Failed to fetch usage')
        )
      } finally {
        setIsQuerying(false)
      }
    },
    [currentRow, kind, t]
  )

  useEffect(() => {
    if (!open) {
      setResponse(null)
      setCurrentKeyIndex(0)
      return
    }

    void fetchUsage(0)
  }, [open, fetchUsage])

  if (!currentRow) return null

  return (
    <ChannelPlanUsageDialog
      open={open}
      onOpenChange={onOpenChange}
      kind={kind}
      channel={currentRow}
      response={response}
      currentKeyIndex={currentKeyIndex}
      onKeyIndexChange={(keyIndex) => {
        setCurrentKeyIndex(keyIndex)
        void fetchUsage(keyIndex)
      }}
      onRefresh={(keyIndex) => {
        void fetchUsage(keyIndex)
      }}
      isRefreshing={isQuerying || (open && !response)}
    />
  )
}
