import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ACCOUNT_DISABLED_DIALOG_EVENT,
  type AccountDisabledDialogPayload,
} from '@/lib/account-disabled-dialog'
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

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-hidden sm:max-w-2xl'>
        <DialogHeader className='pr-8 text-start'>
          <DialogTitle>{payload?.title || t('Account disabled')}</DialogTitle>
        </DialogHeader>
        <div className='max-h-[min(70dvh,42rem)] overflow-y-auto rounded-lg border bg-muted/20 p-4'>
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
