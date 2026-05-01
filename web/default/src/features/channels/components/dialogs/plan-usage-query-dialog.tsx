import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import {
  getMiniMaxTokenPlanUsage,
  getZhipuCodingPlanUsage,
  type ChannelPlanUsageResponse,
} from '../../api'
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

  const kind: ChannelPlanUsageKind =
    currentRow?.type === 35 ? 'minimax' : 'zhipu'

  const fetchUsage = useCallback(
    async (keyIndex: number) => {
      const row = currentRow
      if (!row) return

      setIsQuerying(true)
      try {
        const res =
          kind === 'minimax'
            ? await getMiniMaxTokenPlanUsage(row.id, keyIndex)
            : await getZhipuCodingPlanUsage(row.id, keyIndex)
        const resolvedKeyIndex = Number(res.key_index ?? keyIndex)
        setCurrentKeyIndex(
          Number.isFinite(resolvedKeyIndex) ? resolvedKeyIndex : 0
        )
        setResponse(res)
        if (!res.success) {
          toast.error(
            res.message ||
              (kind === 'minimax'
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
