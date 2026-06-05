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
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DateTimePicker } from '@/components/datetime-picker'
import { batchCreateIPBans } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import {
  IP_BAN_BATCH_FORM_DEFAULT_VALUES,
  getIPBanBatchFormSchema,
  transformIPBanBatchFormToPayload,
  type IPBanBatchFormValues,
} from '../lib'
import type {
  ApiResponse,
  IPBanBatchResult,
  IPBanConfirmationData,
} from '../types'
import { useIPBans } from './ip-bans-provider'

type IPBansBatchDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function dateToUnixSeconds(date: Date | undefined) {
  return date ? Math.floor(date.getTime() / 1000) : 0
}

function unixSecondsToDate(value: number) {
  return value > 0 ? new Date(value * 1000) : undefined
}

function getSelfLockConfirmation(
  response: ApiResponse<unknown>
): IPBanConfirmationData | null {
  const data = response.data as IPBanConfirmationData | undefined
  return response.success === false && data?.requires_confirmation === true
    ? data
    : null
}

export function IPBansBatchDialog({
  open,
  onOpenChange,
}: IPBansBatchDialogProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useIPBans()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [pendingValues, setPendingValues] =
    useState<IPBanBatchFormValues | null>(null)
  const [confirmationData, setConfirmationData] =
    useState<IPBanConfirmationData | null>(null)
  const [lastResult, setLastResult] = useState<IPBanBatchResult | null>(null)

  const form = useForm<IPBanBatchFormValues>({
    resolver: zodResolver(getIPBanBatchFormSchema(t)),
    defaultValues: IP_BAN_BATCH_FORM_DEFAULT_VALUES,
  })

  const submitValues = async (
    values: IPBanBatchFormValues,
    confirmed = false
  ) => {
    setIsSubmitting(true)
    try {
      const result = await batchCreateIPBans(
        transformIPBanBatchFormToPayload(values, confirmed)
      )

      const confirmation = getSelfLockConfirmation(result)
      if (confirmation) {
        setConfirmationData(confirmation)
        setPendingValues(values)
        setConfirmOpen(true)
        return
      }

      if (!result.success) {
        toast.error(result.message || t('Failed to import IP ban rules'))
        return
      }

      setLastResult(result.data ?? null)
      toast.success(
        t(SUCCESS_MESSAGES.IP_BAN_BATCH_CREATED, {
          created: result.data?.created ?? 0,
          skipped: result.data?.skipped ?? 0,
        })
      )
      triggerRefresh()
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleOpenChange = (value: boolean) => {
    onOpenChange(value)
    if (!value) {
      form.reset(IP_BAN_BATCH_FORM_DEFAULT_VALUES)
      setPendingValues(null)
      setConfirmationData(null)
      setConfirmOpen(false)
      setLastResult(null)
    }
  }

  const handleConfirmSelfLock = async () => {
    if (!pendingValues) return
    setConfirmOpen(false)
    await submitValues(pendingValues, true)
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className='flex max-h-[90dvh] flex-col overflow-hidden sm:max-w-[720px]'>
          <DialogHeader>
            <DialogTitle>{t('Batch Import IP Ban Rules')}</DialogTitle>
            <DialogDescription>
              {t(
                'Enter one IP or CIDR per line. Add an inline reason after whitespace, or use the default reason below.'
              )}
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form
              id='ip-ban-batch-form'
              onSubmit={form.handleSubmit((values) =>
                submitValues(values, false)
              )}
              className='min-h-0 flex-1 space-y-4 overflow-y-auto py-1'
            >
              <FormField
                control={form.control}
                name='lines'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('IP List')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        rows={9}
                        className='font-mono text-sm'
                        placeholder={[
                          '203.0.113.10 abusive requests',
                          '198.51.100.0/24 scanner range',
                          '2001:db8::/32',
                        ].join('\n')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Duplicate normalized targets are ignored.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='default_reason'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default Reason')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('Used when a line has no inline reason')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='expires_at'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <FormControl>
                      <DateTimePicker
                        value={unixSecondsToDate(field.value)}
                        onChange={(date) =>
                          field.onChange(dateToUnixSeconds(date))
                        }
                        placeholder={t('Never expires')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Leave empty for permanent bans.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {lastResult && (
                <div className='bg-muted/40 space-y-2 rounded-lg border p-3 text-sm'>
                  <div className='grid grid-cols-3 gap-2'>
                    <div>
                      <div className='text-muted-foreground'>
                        {t('Created')}
                      </div>
                      <div className='font-semibold'>{lastResult.created}</div>
                    </div>
                    <div>
                      <div className='text-muted-foreground'>
                        {t('Skipped')}
                      </div>
                      <div className='font-semibold'>{lastResult.skipped}</div>
                    </div>
                    <div>
                      <div className='text-muted-foreground'>
                        {t('Invalid')}
                      </div>
                      <div className='font-semibold'>
                        {lastResult.invalid.length}
                      </div>
                    </div>
                  </div>
                  {lastResult.invalid.length > 0 && (
                    <div className='bg-background max-h-32 overflow-auto rounded border p-2 font-mono text-xs'>
                      {lastResult.invalid.map((item) => (
                        <div key={`${item.line_number}-${item.content}`}>
                          {t('Line {{line}}', {
                            line: item.line_number,
                          })}
                          : {item.message}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </form>
          </Form>
          <DialogFooter className='grid grid-cols-2 gap-2 sm:flex'>
            <Button
              type='button'
              variant='outline'
              onClick={() => handleOpenChange(false)}
              disabled={isSubmitting}
            >
              {t('Close')}
            </Button>
            <Button
              form='ip-ban-batch-form'
              type='submit'
              disabled={isSubmitting}
            >
              {isSubmitting ? t('Importing...') : t('Import')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm self lock')}
        desc={t(
          'One imported rule matches your current IP address {{ip}}. Continue only if you still have another way to access the admin panel.',
          { ip: confirmationData?.client_ip || '-' }
        )}
        destructive
        isLoading={isSubmitting}
        confirmText={t('Confirm and import')}
        handleConfirm={handleConfirmSelfLock}
      />
    </>
  )
}
