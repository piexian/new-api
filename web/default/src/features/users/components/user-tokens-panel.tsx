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
import { useEffect, useState, type ReactNode } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  ChevronDown,
  KeyRound,
  Loader2,
  Pencil,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Settings2,
  Trash2,
  WalletCards,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserGroups, getUserModels } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
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
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DateTimePicker } from '@/components/datetime-picker'
import { MultiSelect } from '@/components/multi-select'
import { StatusBadge } from '@/components/status-badge'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from '@/features/keys/components/api-key-group-combobox'
import { API_KEY_STATUS, API_KEY_STATUSES } from '@/features/keys/constants'
import {
  getApiKeyFormDefaultValues,
  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
  type ApiKeyFormValues,
} from '@/features/keys/lib'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../../keys/constants'
import {
  createUserToken,
  deleteUserToken,
  getUserToken,
  getUserTokens,
  updateUserToken,
  updateUserTokenStatus,
} from '../api'
import type { User, UserToken } from '../types'

const USER_TOKEN_PAGE_SIZE = 5

type UserTokensPanelProps = {
  user: Pick<User, 'id' | 'username'>
}

type UserTokenDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  currentToken?: UserToken
  onSuccess: () => void
}

type UserTokenFormSectionProps = {
  title: string
  icon: LucideIcon
  children: ReactNode
}

function UserTokenFormSection(props: UserTokenFormSectionProps) {
  const Icon = props.icon

  return (
    <section className='rounded-lg border'>
      <div className='flex items-center gap-2.5 border-b px-3 py-2.5'>
        <div className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg border'>
          <Icon className='size-4' />
        </div>
        <h3 className='text-sm leading-none font-medium'>{props.title}</h3>
      </div>
      <div className='flex flex-col gap-3 p-3'>{props.children}</div>
    </section>
  )
}

function getTimestampLabel(timestamp: number, neverLabel: string) {
  if (timestamp === -1) return neverLabel
  return formatTimestampToDate(timestamp)
}

function UserTokenDialog(props: UserTokenDialogProps) {
  const { t } = useTranslation()
  const isUpdate = !!props.currentToken
  const { status } = useStatus()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const defaultUseAutoGroup = status?.default_use_auto_group === true

  const { data: modelsData } = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    staleTime: 5 * 60 * 1000,
  })

  const { data: groupsData } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 5 * 60 * 1000,
  })

  const models = modelsData?.data || []
  const groupsRaw = groupsData?.data || {}
  const groups: ApiKeyGroupOption[] = Object.entries(groupsRaw).map(
    ([key, info]) => ({
      value: key,
      label: key,
      desc: info.desc || key,
      ratio: info.ratio,
    })
  )
  const backendHasAuto = groups.some((g) => g.value === 'auto')
  const schema = getApiKeyFormSchema(t)

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getApiKeyFormDefaultValues(defaultUseAutoGroup),
  })

  useEffect(() => {
    if (props.open && isUpdate && props.currentToken) {
      getUserToken(props.userId, props.currentToken.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformApiKeyToFormDefaults(result.data))
        }
      })
    } else if (props.open && !isUpdate) {
      form.reset(
        getApiKeyFormDefaultValues(defaultUseAutoGroup && backendHasAuto)
      )
    }
  }, [
    props.open,
    props.userId,
    props.currentToken,
    isUpdate,
    form,
    defaultUseAutoGroup,
    backendHasAuto,
  ])

  useEffect(() => {
    if (groups.length === 0) return
    const currentGroup = form.getValues('group')
    if (currentGroup && !groups.some((g) => g.value === currentGroup)) {
      const fallback =
        groups.find((g) => g.value === 'default')?.value ??
        groups[0]?.value ??
        ''
      form.setValue('group', fallback)
      if (currentGroup === 'auto') {
        form.setValue('cross_group_retry', false)
      }
    }
  }, [groups, form])

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    if (months === 0 && days === 0 && hours === 0) {
      form.setValue('expired_time', undefined)
      return
    }

    const now = new Date()
    now.setMonth(now.getMonth() + months)
    now.setDate(now.getDate() + days)
    now.setHours(now.getHours() + hours)
    form.setValue('expired_time', now)
  }

  const onSubmit = async (data: ApiKeyFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && props.currentToken) {
        const result = await updateUserToken(
          props.userId,
          props.currentToken.id,
          basePayload
        )
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          props.onOpenChange(false)
          props.onSuccess()
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
        return
      }

      const count = data.tokenCount || 1
      let successCount = 0
      for (let i = 0; i < count; i++) {
        const result = await createUserToken(props.userId, {
          ...basePayload,
          name:
            i === 0 && data.name
              ? data.name
              : `${data.name || 'default'}-${Math.random().toString(36).slice(2, 8)}`,
        })
        if (result.success) {
          successCount++
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
          break
        }
      }

      if (successCount > 0) {
        toast.success(
          t('Successfully created {{count}} API Key(s)', {
            count: successCount,
          })
        )
        props.onOpenChange(false)
        props.onSuccess()
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const onInvalid: SubmitErrorHandler<ApiKeyFormValues> = () => {
    toast.error(t('Please fix the highlighted fields before saving'))
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const selectedGroup = form.watch('group')
  const unlimitedQuota = form.watch('unlimited_quota')
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col overflow-hidden max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {isUpdate ? t('Update API Key') : t('Create API Key')}
          </DialogTitle>
          <DialogDescription>
            {isUpdate
              ? t('Update the API key by providing necessary info.')
              : t('Add a new API key by providing necessary info.')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form className='min-h-0 flex-1 overflow-y-auto pr-1'>
            <div className='flex flex-col gap-3'>
              <UserTokenFormSection
                title={t('Basic Information')}
                icon={KeyRound}
              >
                <FormField
                  control={form.control}
                  name='name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Name')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Enter a name')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='group'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Group')}</FormLabel>
                      <FormControl>
                        <ApiKeyGroupCombobox
                          options={groups}
                          value={field.value}
                          onValueChange={field.onChange}
                          placeholder={t('Select a group')}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {selectedGroup === 'auto' && (
                  <FormField
                    control={form.control}
                    name='cross_group_retry'
                    render={({ field }) => (
                      <FormItem className='flex min-h-16 flex-row items-center justify-between gap-3 rounded-lg border px-3 py-2.5'>
                        <div className='flex flex-col gap-0.5'>
                          <FormLabel className='text-sm'>
                            {t('Cross-group retry')}
                          </FormLabel>
                          <FormDescription className='text-xs'>
                            {t(
                              'When enabled, if channels in the current group fail, it will try channels in the next group in order.'
                            )}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={!!field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                )}

                <FormField
                  control={form.control}
                  name='expired_time'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Expiration Time')}</FormLabel>
                      <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'>
                        <FormControl>
                          <DateTimePicker
                            value={field.value}
                            onChange={field.onChange}
                            placeholder={t('Never expires')}
                            className='min-w-0 [&_input[type=time]]:w-24 sm:[&_input[type=time]]:w-32'
                          />
                        </FormControl>
                        <div className='grid grid-cols-4 gap-2 sm:flex'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='px-2 text-xs sm:px-3 sm:text-sm'
                            onClick={() => handleSetExpiry(0, 0, 0)}
                          >
                            {t('Never')}
                          </Button>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='px-2 text-xs sm:px-3 sm:text-sm'
                            onClick={() => handleSetExpiry(1, 0, 0)}
                          >
                            {t('1 Month')}
                          </Button>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='px-2 text-xs sm:px-3 sm:text-sm'
                            onClick={() => handleSetExpiry(0, 1, 0)}
                          >
                            {t('1 Day')}
                          </Button>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='px-2 text-xs sm:px-3 sm:text-sm'
                            onClick={() => handleSetExpiry(0, 0, 1)}
                          >
                            {t('1 Hour')}
                          </Button>
                        </div>
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {!isUpdate && (
                  <FormField
                    control={form.control}
                    name='tokenCount'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Quantity')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='1'
                            placeholder={t('Number of keys to create')}
                            onChange={(e) =>
                              field.onChange(parseInt(e.target.value, 10) || 1)
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </UserTokenFormSection>

              <UserTokenFormSection
                title={t('Quota Settings')}
                icon={WalletCards}
              >
                {!unlimitedQuota && (
                  <FormField
                    control={form.control}
                    name='remain_quota_dollars'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{quotaLabel}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            step={tokensOnly ? 1 : 0.01}
                            placeholder={quotaPlaceholder}
                            onChange={(e) =>
                              field.onChange(parseFloat(e.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <FormField
                  control={form.control}
                  name='unlimited_quota'
                  render={({ field }) => (
                    <FormItem className='flex min-h-16 flex-row items-center justify-between gap-3 rounded-lg border px-3 py-2.5'>
                      <div className='flex flex-col gap-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Unlimited Quota')}
                        </FormLabel>
                        <FormDescription className='text-xs'>
                          {t('Enable unlimited quota for this API key')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </UserTokenFormSection>

              <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <section className='rounded-lg border'>
                  <CollapsibleTrigger
                    render={
                      <button
                        type='button'
                        className='hover:bg-muted/50 flex w-full items-center gap-2.5 px-3 py-2.5 text-left transition-colors'
                      />
                    }
                  >
                    <div className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg border'>
                      <Settings2 className='size-4' />
                    </div>
                    <h3 className='min-w-0 flex-1 text-sm leading-none font-medium'>
                      {t('Advanced Settings')}
                    </h3>
                    <ChevronDown
                      className={cn(
                        'text-muted-foreground size-4 shrink-0 transition-transform',
                        advancedOpen && 'rotate-180'
                      )}
                    />
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className='flex flex-col gap-3 border-t p-3'>
                      <FormField
                        control={form.control}
                        name='model_limits'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Model Limits')}</FormLabel>
                            <FormControl>
                              <MultiSelect
                                options={models.map((m) => ({
                                  label: m,
                                  value: m,
                                }))}
                                selected={field.value}
                                onChange={field.onChange}
                                placeholder={t(
                                  'Select models (empty for allow all)'
                                )}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='allow_ips'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('IP Whitelist (supports CIDR)')}
                            </FormLabel>
                            <FormControl>
                              <Textarea
                                {...field}
                                className='min-h-20 resize-none'
                                placeholder={t(
                                  'One IP per line (empty for no restriction)'
                                )}
                                rows={3}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                  </CollapsibleContent>
                </section>
              </Collapsible>
            </div>
          </form>
        </Form>
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={form.handleSubmit(onSubmit, onInvalid)}
            disabled={isSubmitting}
          >
            {isSubmitting && <Loader2 data-icon='inline-start' />}
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function UserTokenDetail(props: { label: string; value: ReactNode }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-[11px]'>{props.label}</div>
      <div className='truncate text-xs font-medium'>{props.value}</div>
    </div>
  )
}

function UserTokenQuota({ token }: { token: UserToken }) {
  const { t } = useTranslation()
  if (token.unlimited_quota) {
    return <span>{t('Unlimited')}</span>
  }
  return (
    <span>
      {formatQuota(token.remain_quota)} / {formatQuota(token.used_quota)}
    </span>
  )
}

export function UserTokensPanel({ user }: UserTokensPanelProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [editorOpen, setEditorOpen] = useState(false)
  const [currentToken, setCurrentToken] = useState<UserToken | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<UserToken | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [togglingId, setTogglingId] = useState<number | null>(null)

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['admin-user-tokens', user.id, page],
    queryFn: () =>
      getUserTokens(user.id, {
        p: page,
        size: USER_TOKEN_PAGE_SIZE,
      }),
    enabled: user.id > 0,
  })

  const tokens = data?.data?.items || []
  const total = data?.data?.total || 0
  const totalPages = Math.max(1, Math.ceil(total / USER_TOKEN_PAGE_SIZE))

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages)
    }
  }, [page, totalPages])

  const openCreateDialog = () => {
    setCurrentToken(undefined)
    setEditorOpen(true)
  }

  const openEditDialog = (token: UserToken) => {
    setCurrentToken(token)
    setEditorOpen(true)
  }

  const handleToggleStatus = async (token: UserToken) => {
    const enabled = token.status === API_KEY_STATUS.ENABLED
    const nextStatus = enabled
      ? API_KEY_STATUS.DISABLED
      : API_KEY_STATUS.ENABLED

    setTogglingId(token.id)
    try {
      const result = await updateUserTokenStatus(user.id, token.id, nextStatus)
      if (result.success) {
        toast.success(
          enabled
            ? t(SUCCESS_MESSAGES.API_KEY_DISABLED)
            : t(SUCCESS_MESSAGES.API_KEY_ENABLED)
        )
        refetch()
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.STATUS_UPDATE_FAILED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setTogglingId(null)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeletingId(deleteTarget.id)
    try {
      const result = await deleteUserToken(user.id, deleteTarget.id)
      if (result.success) {
        toast.success(t(SUCCESS_MESSAGES.API_KEY_DELETED))
        setDeleteTarget(null)
        if (tokens.length === 1 && page > 1) {
          setPage(page - 1)
        } else {
          refetch()
        }
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.DELETE_FAILED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <>
      <section className='rounded-lg border'>
        <div className='flex items-center justify-between gap-3 border-b px-3 py-2.5'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <div className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg border'>
              <KeyRound className='size-4' />
            </div>
            <div className='min-w-0'>
              <h3 className='text-sm leading-none font-medium'>
                {t('API Keys')}
              </h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('Total')}: {total}
              </p>
            </div>
          </div>
          <div className='flex shrink-0 items-center gap-1'>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    onClick={() => refetch()}
                    disabled={isFetching}
                    aria-label={t('Refresh')}
                  />
                }
              >
                {isFetching ? (
                  <Loader2 className='animate-spin' />
                ) : (
                  <RefreshCw />
                )}
              </TooltipTrigger>
              <TooltipContent>{t('Refresh')}</TooltipContent>
            </Tooltip>
            <Button type='button' size='sm' onClick={openCreateDialog}>
              <Plus data-icon='inline-start' />
              {t('Create')}
            </Button>
          </div>
        </div>

        <div className='flex flex-col gap-2 p-3'>
          {isLoading ? (
            <>
              <Skeleton className='h-24 w-full' />
              <Skeleton className='h-24 w-full' />
            </>
          ) : tokens.length === 0 ? (
            <Empty className='min-h-32'>
              <EmptyHeader>
                <EmptyMedia variant='icon'>
                  <KeyRound />
                </EmptyMedia>
                <EmptyTitle>{t('No API key yet')}</EmptyTitle>
                <EmptyDescription>
                  {t(
                    'No API keys available. Create your first API key to get started.'
                  )}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button type='button' size='sm' onClick={openCreateDialog}>
                  <Plus data-icon='inline-start' />
                  {t('Create')}
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            tokens.map((token) => {
              const enabled = token.status === API_KEY_STATUS.ENABLED
              const statusConfig = API_KEY_STATUSES[token.status]
              return (
                <div key={token.id} className='rounded-lg border p-3'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <div className='flex min-w-0 items-center gap-2'>
                        <div className='truncate text-sm font-medium'>
                          {token.name}
                        </div>
                        {statusConfig && (
                          <StatusBadge
                            label={t(statusConfig.label)}
                            variant={statusConfig.variant}
                            showDot={statusConfig.showDot}
                            copyable={false}
                          />
                        )}
                      </div>
                      <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                        sk-{token.key}
                      </div>
                    </div>
                    <div className='flex shrink-0 items-center gap-1'>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => handleToggleStatus(token)}
                              disabled={togglingId === token.id}
                              aria-label={enabled ? t('Disable') : t('Enable')}
                            />
                          }
                        >
                          {togglingId === token.id ? (
                            <Loader2 className='animate-spin' />
                          ) : enabled ? (
                            <PowerOff />
                          ) : (
                            <Power />
                          )}
                        </TooltipTrigger>
                        <TooltipContent>
                          {enabled ? t('Disable') : t('Enable')}
                        </TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => openEditDialog(token)}
                              aria-label={t('Edit')}
                            />
                          }
                        >
                          <Pencil />
                        </TooltipTrigger>
                        <TooltipContent>{t('Edit')}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => setDeleteTarget(token)}
                              aria-label={t('Delete')}
                            />
                          }
                        >
                          <Trash2 />
                        </TooltipTrigger>
                        <TooltipContent>{t('Delete')}</TooltipContent>
                      </Tooltip>
                    </div>
                  </div>

                  <div className='mt-3 grid gap-2 sm:grid-cols-2'>
                    <UserTokenDetail
                      label={t('Group')}
                      value={token.group || 'default'}
                    />
                    <UserTokenDetail
                      label={t('Quota')}
                      value={<UserTokenQuota token={token} />}
                    />
                    <UserTokenDetail
                      label={t('Expires')}
                      value={getTimestampLabel(token.expired_time, t('Never'))}
                    />
                    <UserTokenDetail
                      label={t('Last Used')}
                      value={formatTimestampToDate(token.accessed_time)}
                    />
                  </div>
                </div>
              )
            })
          )}
        </div>

        {totalPages > 1 && (
          <div className='flex items-center justify-between gap-3 border-t px-3 py-2 text-xs'>
            <span className='text-muted-foreground'>
              {t('Page')} {page} / {totalPages}
            </span>
            <div className='flex items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                size='xs'
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                {t('Previous')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='xs'
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              >
                {t('Next')}
              </Button>
            </div>
          </div>
        )}
      </section>

      <UserTokenDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        userId={user.id}
        currentToken={currentToken}
        onSuccess={() => refetch()}
      />

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This action cannot be undone.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingId !== null}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={deletingId !== null}
              onClick={handleDelete}
            >
              {deletingId !== null && <Loader2 data-icon='inline-start' />}
              {deletingId !== null ? t('Processing...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
