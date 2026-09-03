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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
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

const rateLimitDialogSchema = z.object({
  groupName: z.string().min(1, 'Group name is required'),
  maxRequests: z
    .number()
    .min(0, 'Must be ≥ 0')
    .max(2147483647, 'Must be ≤ 2,147,483,647'),
  maxSuccess: z
    .number()
    .min(1, 'Must be ≥ 1')
    .max(2147483647, 'Must be ≤ 2,147,483,647'),
})

const concurrencyDialogSchema = z.object({
  groupName: z.string().min(1, 'Group name is required'),
  maxConcurrent: z
    .number()
    .int('Must be an integer')
    .min(0, 'Must be ≥ 0')
    .max(2147483647, 'Must be ≤ 2,147,483,647'),
})

export type RateLimitEditorMode = 'rate' | 'concurrency'

export type RateLimitEntryData = {
  groupName: string
  maxRequests?: number
  maxSuccess?: number
  maxConcurrent?: number
}

type RateLimitDialogFormValues = {
  groupName: string
  maxRequests?: number
  maxSuccess?: number
  maxConcurrent?: number
}

type RateLimitDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (data: RateLimitEntryData) => void
  editData?: RateLimitEntryData | null
  mode?: RateLimitEditorMode
}

export function RateLimitDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  mode = 'rate',
}: RateLimitDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData
  const isConcurrency = mode === 'concurrency'
  // 同一页面可能同时挂载速率与并发两个弹窗，表单 id 需按模式区分
  const formId = `rate-limit-form-${mode}`

  const form = useForm<RateLimitDialogFormValues>({
    resolver: zodResolver(
      isConcurrency ? concurrencyDialogSchema : rateLimitDialogSchema
    ) as unknown as Resolver<RateLimitDialogFormValues>,
    defaultValues: {
      groupName: '',
      maxRequests: 0,
      maxSuccess: 1,
      maxConcurrent: 0,
    },
  })

  useEffect(() => {
    if (editData) {
      form.reset(editData)
    } else {
      form.reset({
        groupName: '',
        maxRequests: 0,
        maxSuccess: 1,
        maxConcurrent: 0,
      })
    }
  }, [editData, form, open])

  const handleSubmit = (values: RateLimitDialogFormValues) => {
    onSave(values)
    form.reset()
    onOpenChange(false)
  }

  const title = isConcurrency
    ? isEditMode
      ? t('Edit group concurrency limit')
      : t('Add group concurrency limit')
    : isEditMode
      ? t('Edit group rate limit')
      : t('Add group rate limit')

  const description = isConcurrency
    ? t('Limit concurrent in-flight requests per account for a specific group.')
    : t('Configure rate limiting rules for a specific user group.')

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      contentClassName='sm:max-w-[500px]'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={formId}>
            {isEditMode ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={formId}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='groupName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group Name')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g., default, vip, premium')}
                    {...field}
                    disabled={isEditMode}
                  />
                </FormControl>
                <FormDescription>
                  {isEditMode
                    ? t('Group name cannot be changed when editing.')
                    : t('Unique identifier for this group.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {isConcurrency ? (
            <FormField
              control={form.control}
              name='maxConcurrent'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Concurrent Requests')}</FormLabel>
                  <FormControl>
                    <div className='flex items-center gap-2'>
                      <Input
                        type='number'
                        min={0}
                        max={2147483647}
                        step={1}
                        {...field}
                        onChange={(e) =>
                          field.onChange(parseInt(e.target.value) || 0)
                        }
                      />
                      <span className='text-muted-foreground text-sm'>
                        {t('requests')}
                      </span>
                    </div>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'In-flight requests allowed per account in this group. 0 = unlimited.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          ) : (
            <>
              <FormField
                control={form.control}
                name='maxRequests'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Max Requests (including failures)')}
                    </FormLabel>
                    <FormControl>
                      <div className='flex items-center gap-2'>
                        <Input
                          type='number'
                          min={0}
                          max={2147483647}
                          step={1}
                          {...field}
                          onChange={(e) =>
                            field.onChange(parseInt(e.target.value) || 0)
                          }
                        />
                        <span className='text-muted-foreground text-sm'>
                          {t('times')}
                        </span>
                      </div>
                    </FormControl>
                    <FormDescription>
                      {t('Total requests allowed per period. 0 = unlimited.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='maxSuccess'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Max Successful Requests')}</FormLabel>
                    <FormControl>
                      <div className='flex items-center gap-2'>
                        <Input
                          type='number'
                          min={1}
                          max={2147483647}
                          step={1}
                          {...field}
                          onChange={(e) =>
                            field.onChange(parseInt(e.target.value) || 1)
                          }
                        />
                        <span className='text-muted-foreground text-sm'>
                          {t('times')}
                        </span>
                      </div>
                    </FormControl>
                    <FormDescription>
                      {t('Only successful requests count toward this limit.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          )}
        </form>
      </Form>
    </Dialog>
  )
}
