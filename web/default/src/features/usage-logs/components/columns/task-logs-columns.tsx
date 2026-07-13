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
import type { ColumnDef } from '@tanstack/react-table'
import { Music, Settings2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { TASK_ACTIONS, TASK_STATUS } from '../../constants'
import {
  buildGenerationParamRows,
  generationParamsSummary,
  taskGenerationParams,
} from '../../lib/generation-params'
import { taskActionMapper, taskStatusMapper } from '../../lib/mappers'
import type { TaskLog } from '../../types'
import {
  AudioPreviewDialog,
  type AudioClip,
} from '../dialogs/audio-preview-dialog'
import { FailReasonDialog } from '../dialogs/fail-reason-dialog'
import { useUsageLogsContext } from '../usage-logs-provider'
import {
  createDurationColumn,
  createChannelColumn,
  createProgressColumn,
} from './column-helpers'

function parseTaskData(data: unknown): unknown[] {
  if (Array.isArray(data)) return data
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }
  return []
}

function TaskDetailRow(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[5.25rem_minmax(0,1fr)] gap-2 text-sm sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'max-w-full min-w-0 text-xs break-all sm:break-words',
          props.mono && 'font-mono'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function TaskDetailSection(props: {
  icon?: React.ReactNode
  label: string
  children: React.ReactNode
}) {
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label className='flex items-center gap-1.5 text-xs font-semibold'>
        {props.icon}
        {props.label}
      </Label>
      <div className='bg-muted/30 min-w-0 space-y-1 overflow-hidden rounded-md border p-2.5 max-sm:p-2'>
        {props.children}
      </div>
    </div>
  )
}

function TaskDetailsDialog(props: {
  log: TaskLog
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const params = taskGenerationParams(props.log)
  const rows = buildGenerationParamRows(params, t)
  const actionLabel = taskActionMapper.getLabel(props.log.action)
  const platformLabel = params?.provider === 'xai' ? 'xai' : props.log.platform

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='min-w-0 overflow-hidden max-sm:max-h-[calc(100dvh-1.5rem)] max-sm:w-[calc(100vw-1.5rem)] max-sm:max-w-[calc(100vw-1.5rem)] max-sm:p-4 sm:max-w-lg'>
        <DialogHeader className='max-sm:gap-1'>
          <DialogTitle className='flex items-center gap-2 text-base'>
            {t('Task Details')}
            <StatusBadge
              label={t(taskStatusMapper.getLabel(props.log.status))}
              variant={taskStatusMapper.getVariant(props.log.status)}
              size='sm'
              copyable={false}
            />
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('View the complete details for this task')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[70vh] min-w-0 overflow-hidden pr-2 max-sm:max-h-[calc(100dvh-7rem)] sm:pr-4'>
          <div className='w-full max-w-full min-w-0 space-y-2.5 overflow-hidden py-1 sm:space-y-3'>
            <div className='min-w-0 space-y-1'>
              <TaskDetailRow
                label={t('Task ID')}
                value={props.log.task_id}
                mono
              />
              <TaskDetailRow
                label={t('Platform')}
                value={t(platformLabel)}
                mono
              />
              <TaskDetailRow label={t('Action')} value={t(actionLabel)} />
            </div>

            {rows.length > 0 && (
              <TaskDetailSection
                icon={<Settings2 className='size-3.5' aria-hidden='true' />}
                label={t('Generation Parameters')}
              >
                {rows.map((row) => (
                  <TaskDetailRow
                    key={row.key}
                    label={row.label}
                    value={row.value}
                    mono={row.mono}
                  />
                ))}
              </TaskDetailSection>
            )}

            {props.log.fail_reason && (
              <TaskDetailSection label={t('Fail Reason')}>
                <p className='text-xs break-words'>{props.log.fail_reason}</p>
              </TaskDetailSection>
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

function AudioPreviewCell({ log }: { log: TaskLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const clips = useMemo(() => {
    const data = parseTaskData(log.data)
    return data.filter(
      (c) =>
        c && typeof c === 'object' && (c as Record<string, unknown>).audio_url
    )
  }, [log.data])

  if (clips.length === 0) return null

  return (
    <>
      <button
        type='button'
        className='group flex items-center gap-1 text-left text-xs'
        onClick={() => setOpen(true)}
      >
        <Music className='text-muted-foreground size-3' />
        <span className='text-foreground leading-snug group-hover:underline'>
          {t('Click to preview audio')}
        </span>
      </button>
      <AudioPreviewDialog
        open={open}
        onOpenChange={setOpen}
        clips={clips as AudioClip[]}
      />
    </>
  )
}

export function useTaskLogsColumns(isAdmin: boolean): ColumnDef<TaskLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<TaskLog>[] = [
    {
      accessorKey: 'submit_time',
      header: t('Submit Time'),
      cell: ({ row }) => {
        const log = row.original
        const submitTime = row.getValue('submit_time') as number

        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(submitTime, 'seconds')}
            </span>
            {log.finish_time ? (
              <span className='text-muted-foreground/60 truncate font-mono text-[11px] tabular-nums'>
                {formatTimestampToDate(log.finish_time, 'seconds')}
              </span>
            ) : (
              <span className='text-muted-foreground/50 text-[11px]'>-</span>
            )}
          </div>
        )
      },
      size: 180,
    },
  ]

  if (isAdmin) {
    columns.push(createChannelColumn<TaskLog>({ headerLabel: t('Channel') }), {
      id: 'user',
      header: t('User'),
      accessorFn: (row) => row.username || row.user_id,
      cell: function UserCell({ row }) {
        const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
          useUsageLogsContext()
        const log = row.original
        const displayName = log.username || String(log.user_id || '?')

        return (
          <button
            type='button'
            className='flex items-center gap-1.5 text-left'
            onClick={(e) => {
              e.stopPropagation()
              setSelectedUserId(log.user_id)
              setUserInfoDialogOpen(true)
            }}
          >
            <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
              <AvatarFallback
                className={cn(
                  'text-[11px] font-semibold',
                  !sensitiveVisible && 'bg-muted text-muted-foreground'
                )}
                style={
                  sensitiveVisible ? getUserAvatarStyle(displayName) : undefined
                }
              >
                {sensitiveVisible ? getUserAvatarFallback(displayName) : '•'}
              </AvatarFallback>
            </Avatar>
            <span className='text-muted-foreground truncate text-sm hover:underline'>
              {sensitiveVisible ? displayName : '••••'}
            </span>
          </button>
        )
      },
    })
  }

  columns.push(
    {
      accessorKey: 'task_id',
      header: t('Task ID'),
      cell: ({ row }) => {
        const log = row.original
        const taskId = row.getValue('task_id') as string
        const params = taskGenerationParams(log)
        const platformLabel = params?.provider === 'xai' ? 'xai' : log.platform
        if (!taskId) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }
        return (
          <div className='flex max-w-[170px] flex-col gap-0.5'>
            <StatusBadge
              label={taskId}
              copyText={taskId}
              variant='neutral'
              size='sm'
              className='border-border/60 bg-muted/30 !text-foreground max-w-full truncate rounded-md border px-1.5 py-0.5 font-mono'
            />
            <span className='text-muted-foreground/60 truncate text-[11px]'>
              {t(platformLabel)} · {t(taskActionMapper.getLabel(log.action))}
            </span>
          </div>
        )
      },
      meta: { mobileTitle: true },
    },
    createDurationColumn<TaskLog>({
      submitTimeKey: 'submit_time',
      finishTimeKey: 'finish_time',
      unit: 'seconds',
      headerLabel: t('Duration'),
      warningThresholdSec: 300,
    }),
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <StatusBadge
            label={t(taskStatusMapper.getLabel(status, status || 'Submitting'))}
            variant={taskStatusMapper.getVariant(status)}
            size='sm'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
    },
    createProgressColumn<TaskLog>({ headerLabel: t('Progress') }),
    {
      accessorKey: 'fail_reason',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const log = row.original
        const failReason = row.getValue('fail_reason') as string
        const status = log.status
        const [dialogOpen, setDialogOpen] = useState(false)
        const paramRows = buildGenerationParamRows(taskGenerationParams(log), t)
        const hasParams = paramRows.length > 0
        const paramSummary = generationParamsSummary(paramRows, t)

        const isSunoSuccess =
          log.platform === 'suno' && status === TASK_STATUS.SUCCESS
        if (isSunoSuccess) {
          const data = parseTaskData(log.data)
          if (
            data.some(
              (c) =>
                c &&
                typeof c === 'object' &&
                (c as Record<string, unknown>).audio_url
            )
          ) {
            return <AudioPreviewCell log={log} />
          }
        }

        const isVideoTask =
          log.action === TASK_ACTIONS.GENERATE ||
          log.action === TASK_ACTIONS.TEXT_GENERATE ||
          log.action === TASK_ACTIONS.FIRST_TAIL_GENERATE ||
          log.action === TASK_ACTIONS.REFERENCE_GENERATE ||
          log.action === TASK_ACTIONS.REMIX_GENERATE ||
          log.action === TASK_ACTIONS.VIDEO_EDIT ||
          log.action === TASK_ACTIONS.VIDEO_EXTEND
        const isSuccess = status === TASK_STATUS.SUCCESS
        const resultUrl = log.result_url || failReason
        const isUrl = resultUrl?.startsWith('http')

        if (isSuccess && isVideoTask && isUrl) {
          const videoUrl = `/v1/videos/${log.task_id}/content`
          return (
            <>
              <div className='flex max-w-[220px] flex-col gap-1'>
                <a
                  href={videoUrl}
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-foreground text-xs hover:underline'
                >
                  {t('Click to preview video')}
                </a>
                {hasParams && (
                  <button
                    type='button'
                    className='text-muted-foreground text-left text-xs hover:underline'
                    onClick={() => setDialogOpen(true)}
                  >
                    {paramSummary || t('View parameters')}
                  </button>
                )}
              </div>
              <TaskDetailsDialog
                log={log}
                open={dialogOpen}
                onOpenChange={setDialogOpen}
              />
            </>
          )
        }

        if (!failReason && !hasParams) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }

        return (
          <>
            <button
              type='button'
              className='group flex max-w-[200px] items-center gap-1 text-left text-xs'
              onClick={() => setDialogOpen(true)}
              title={
                failReason
                  ? t('Click to view full error message')
                  : t('Click to view full details')
              }
            >
              <span
                className={cn(
                  'truncate leading-snug group-hover:underline',
                  failReason
                    ? 'text-red-600 dark:text-red-400'
                    : 'text-foreground'
                )}
              >
                {failReason || paramSummary || t('View parameters')}
              </span>
            </button>
            {hasParams ? (
              <TaskDetailsDialog
                log={log}
                open={dialogOpen}
                onOpenChange={setDialogOpen}
              />
            ) : (
              <FailReasonDialog
                failReason={failReason}
                open={dialogOpen}
                onOpenChange={setDialogOpen}
              />
            )}
          </>
        )
      },
      size: 200,
      maxSize: 220,
    }
  )

  return columns
}
