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
import { ViewIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'

import { getEmailLog } from '../../api'
import { EMAIL_STATUS_MAPPINGS } from '../../constants'
import type { EmailLog } from '../../types'

const previewCsp = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: cid:; style-src 'unsafe-inline'; font-src data:; base-uri 'none'; form-action 'none'">`

function buildPreviewDocument(content: string): string {
  if (/<head(?:\s[^>]*)?>/i.test(content)) {
    return content.replace(
      /<head(?:\s[^>]*)?>/i,
      (head) => `${head}${previewCsp}`
    )
  }
  if (/<html(?:\s[^>]*)?>/i.test(content)) {
    return content.replace(
      /<html(?:\s[^>]*)?>/i,
      (html) => `${html}<head><meta charset="utf-8">${previewCsp}</head>`
    )
  }
  return `<!doctype html><html><head><meta charset="utf-8">${previewCsp}</head><body>${content}</body></html>`
}

function DetailItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-xs'>{label}</dt>
      <dd className='mt-1 min-w-0 text-sm break-words'>{value || '-'}</dd>
    </div>
  )
}

function EmailLogPreviewDialog({
  log,
  open,
  onOpenChange,
}: {
  log: EmailLog
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['email-log', log.id],
    queryFn: async () => {
      const response = await getEmailLog(log.id)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to load email details'))
      }
      return response.data
    },
  })
  const detail = data ?? log
  const statusConfig = EMAIL_STATUS_MAPPINGS[detail.status] ?? {
    label: detail.status || 'Unknown',
    variant: 'neutral' as const,
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Email Log Details')}
      description={detail.subject || t('Details')}
      contentClassName='sm:max-w-5xl'
      contentHeight='72vh'
      bodyClassName='space-y-4'
    >
      {isLoading && (
        <div className='flex min-h-56 items-center justify-center'>
          <Spinner />
        </div>
      )}
      {!isLoading && isError && (
        <div className='text-destructive flex min-h-40 items-center justify-center text-sm'>
          {t('Failed to load email details')}
        </div>
      )}
      {!isLoading && !isError && (
        <>
          <dl className='grid grid-cols-1 gap-3 rounded-md border p-3 sm:grid-cols-2 lg:grid-cols-4'>
            <DetailItem
              label={t('Send Time')}
              value={formatTimestampToDate(detail.created_at, 'seconds')}
            />
            <DetailItem label={t('Receiver')} value={detail.receiver} />
            <DetailItem label={t('Provider')} value={detail.provider} />
            <DetailItem
              label={t('Status')}
              value={
                <StatusBadge
                  label={t(statusConfig.label)}
                  variant={statusConfig.variant as StatusBadgeProps['variant']}
                  copyable={false}
                  size='sm'
                />
              }
            />
            <div className='sm:col-span-2 lg:col-span-4'>
              <DetailItem label={t('Subject')} value={detail.subject} />
            </div>
            {detail.error_message ? (
              <div className='sm:col-span-2 lg:col-span-4'>
                <DetailItem
                  label={t('Error Message')}
                  value={detail.error_message}
                />
              </div>
            ) : null}
          </dl>

          {detail.content ? (
            <Tabs defaultValue='preview' className='min-h-0'>
              <TabsList>
                <TabsTrigger value='preview'>{t('Preview')}</TabsTrigger>
                <TabsTrigger value='source'>{t('HTML Source')}</TabsTrigger>
              </TabsList>
              <TabsContent value='preview'>
                <iframe
                  title={t('Rendered Email')}
                  sandbox=''
                  referrerPolicy='no-referrer'
                  srcDoc={buildPreviewDocument(detail.content)}
                  className='bg-background h-[52vh] min-h-80 w-full rounded-md border'
                />
              </TabsContent>
              <TabsContent value='source'>
                <pre className='bg-muted/40 h-[52vh] min-h-80 overflow-auto rounded-md border p-3 text-xs break-all whitespace-pre-wrap'>
                  {detail.content}
                </pre>
              </TabsContent>
            </Tabs>
          ) : (
            <div className='text-muted-foreground flex min-h-56 items-center justify-center rounded-md border border-dashed px-6 text-center text-sm'>
              {t('Email content is unavailable for older logs')}
            </div>
          )}
        </>
      )}
    </Dialog>
  )
}

export function EmailLogPreviewAction({ log }: { log: EmailLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              aria-label={t('Preview')}
              onClick={(event) => {
                event.stopPropagation()
                setOpen(true)
              }}
            />
          }
        >
          <HugeiconsIcon icon={ViewIcon} strokeWidth={2} />
        </TooltipTrigger>
        <TooltipContent>{t('Preview')}</TooltipContent>
      </Tooltip>
      {open ? (
        <EmailLogPreviewDialog log={log} open={open} onOpenChange={setOpen} />
      ) : null}
    </>
  )
}
