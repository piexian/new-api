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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Markdown } from '@/components/ui/markdown'
import {
  ACCOUNT_DISABLED_DIALOG_EVENT,
  type AccountDisabledDialogPayload,
} from '@/lib/account-disabled-dialog'

function hasContent(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0
}

export function AccountDisabledDialog() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [payload, setPayload] = useState<AccountDisabledDialogPayload | null>(
    null
  )

  useEffect(() => {
    const handleDialogEvent = (event: Event) => {
      const detail = (event as CustomEvent<AccountDisabledDialogPayload>).detail
      setPayload(detail)
      setOpen(true)
    }

    window.addEventListener(ACCOUNT_DISABLED_DIALOG_EVENT, handleDialogEvent)
    return () => {
      window.removeEventListener(
        ACCOUNT_DISABLED_DIALOG_EVENT,
        handleDialogEvent
      )
    }
  }, [])

  const content = useMemo(() => {
    if (hasContent(payload?.reason)) return payload.reason.trim()
    if (hasContent(payload?.message)) return payload.message.trim()
    return t('This account has been disabled.')
  }, [payload, t])

  const accountMeta = useMemo(() => {
    const items: string[] = []
    if (
      payload?.userId !== undefined &&
      payload.userId !== null &&
      String(payload.userId).trim() !== '' &&
      String(payload.userId) !== '0'
    ) {
      items.push(`ID: ${payload.userId}`)
    }
    if (hasContent(payload?.username)) {
      items.push(payload.username.trim())
    }
    return items.join(' · ')
  }, [payload])

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-hidden sm:max-w-2xl'>
        <DialogHeader className='pr-8 text-start'>
          <div className='flex flex-wrap items-center gap-2'>
            {accountMeta && (
              <span className='bg-muted text-muted-foreground inline-flex max-w-full items-center rounded-md border px-2 py-1 text-xs font-medium'>
                <span className='truncate'>{accountMeta}</span>
              </span>
            )}
            <DialogTitle>{payload?.title || t('Account disabled')}</DialogTitle>
          </div>
        </DialogHeader>
        <div className='bg-muted/20 max-h-[min(70dvh,42rem)] overflow-y-auto rounded-lg border p-4'>
          <Markdown className='prose-neutral dark:prose-invert'>
            {content}
          </Markdown>
        </div>
        <DialogFooter>
          <DialogClose render={<Button type='button' />}>
            {t('Close')}
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
