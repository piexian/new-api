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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
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
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DateTimePicker } from '@/components/datetime-picker'
import { createIPBan, getIPBan, updateIPBan } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import {
  IP_BAN_FORM_DEFAULT_VALUES,
  getIPBanFormSchema,
  transformIPBanFormToPayload,
  transformIPBanToFormDefaults,
  type IPBanFormValues,
} from '../lib'
import type { ApiResponse, IPBan, IPBanConfirmationData } from '../types'
import { useIPBans } from './ip-bans-provider'

type IPBansMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: IPBan
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

export function IPBansMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: IPBansMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = Boolean(currentRow)
  const { triggerRefresh } = useIPBans()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [pendingValues, setPendingValues] = useState<IPBanFormValues | null>(
    null
  )
  const [confirmationData, setConfirmationData] =
    useState<IPBanConfirmationData | null>(null)

  const form = useForm<IPBanFormValues>({
    resolver: zodResolver(getIPBanFormSchema(t)),
    defaultValues: IP_BAN_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open && isUpdate && currentRow) {
      getIPBan(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformIPBanToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      form.reset(IP_BAN_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  const submitValues = async (values: IPBanFormValues, confirmed = false) => {
    setIsSubmitting(true)
    try {
      const payload = transformIPBanFormToPayload(values, confirmed)
      const result: ApiResponse<IPBan> =
        isUpdate && currentRow
          ? await updateIPBan({ ...payload, id: currentRow.id })
          : await createIPBan(payload)

      const confirmation = getSelfLockConfirmation(result)
      if (confirmation) {
        setConfirmationData(confirmation)
        setPendingValues(values)
        setConfirmOpen(true)
        return
      }

      if (!result.success) {
        toast.error(result.message || t('Failed to save IP ban rule'))
        return
      }

      toast.success(
        t(
          isUpdate
            ? SUCCESS_MESSAGES.IP_BAN_UPDATED
            : SUCCESS_MESSAGES.IP_BAN_CREATED
        )
      )
      onOpenChange(false)
      triggerRefresh()
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleConfirmSelfLock = async () => {
    if (!pendingValues) return
    setConfirmOpen(false)
    await submitValues(pendingValues, true)
  }

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          onOpenChange(v)
          if (!v) {
            form.reset()
            setPendingValues(null)
            setConfirmationData(null)
            setConfirmOpen(false)
          }
        }}
      >
        <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
          <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
            <SheetTitle>
              {isUpdate ? t('Update IP Ban Rule') : t('Add IP Ban Rule')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update an IP or CIDR block rule.')
                : t('Block a single IP address or an entire CIDR range.')}{' '}
              {t('Click save when you&apos;re done.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='ip-ban-form'
              onSubmit={form.handleSubmit((values) =>
                submitValues(values, false)
              )}
              className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
            >
              <FormField
                control={form.control}
                name='target'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('IP / CIDR')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('e.g. 203.0.113.10 or 2001:db8::/32')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Supports IPv4, IPv6, and CIDR ranges.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='reason'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Reason')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        rows={3}
                        placeholder={t('Reason shown in admin records')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Keep the reason short and operational.')}
                    </FormDescription>
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
                      {t('Leave empty for a permanent ban.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </form>
          </Form>
          <SheetFooter className='grid grid-cols-2 gap-2 border-t px-4 py-3 sm:flex sm:px-6 sm:py-4'>
            <SheetClose render={<Button variant='outline' />}>
              {t('Close')}
            </SheetClose>
            <Button form='ip-ban-form' type='submit' disabled={isSubmitting}>
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm self lock')}
        desc={t(
          'This rule matches your current IP address {{ip}}. Continue only if you still have another way to access the admin panel.',
          { ip: confirmationData?.client_ip || '-' }
        )}
        destructive
        isLoading={isSubmitting}
        confirmText={t('Confirm and save')}
        handleConfirm={handleConfirmSelfLock}
      />
    </>
  )
}
