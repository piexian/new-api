import { useMemo } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { DataTableColumnHeader } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  formatDuration,
  formatResetPeriod,
  getModelRestrictionMeta,
  getQuotaWindowItems,
} from '../lib'
import type { PlanRecord } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

export function useSubscriptionsColumns(): ColumnDef<PlanRecord>[] {
  const { t } = useTranslation()
  const { status } = useStatus()
  const enableEpay = !!status?.enable_online_topup

  return useMemo(
    (): ColumnDef<PlanRecord>[] => [
      {
        accessorFn: (row) => row.plan.id,
        id: 'id',
        meta: { label: 'ID', mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title='ID' />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>#{row.original.plan.id}</span>
        ),
        size: 60,
      },
      {
        accessorFn: (row) => row.plan.title,
        id: 'title',
        meta: { label: t('Plan'), mobileTitle: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Plan')} />
        ),
        cell: ({ row }) => {
          const plan = row.original.plan
          return (
            <div className='max-w-[200px]'>
              <div className='truncate font-medium'>{plan.title}</div>
              {plan.subtitle && (
                <div className='text-muted-foreground truncate text-xs'>
                  {plan.subtitle}
                </div>
              )}
            </div>
          )
        },
        size: 200,
      },
      {
        accessorFn: (row) => row.plan.price_amount,
        id: 'price',
        meta: { label: t('Price') },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Price')} />
        ),
        cell: ({ row }) => (
          <span className='font-semibold text-emerald-600'>
            {formatBillingCurrencyFromUSD(
              Number(row.original.plan.price_amount || 0)
            )}
          </span>
        ),
        size: 100,
      },
      {
        id: 'purchase_limit',
        meta: { label: t('Purchase Limit'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Purchase Limit')} />
        ),
        cell: ({ row }) => {
          const limit = Number(row.original.plan.max_purchase_per_user || 0)
          return (
            <span className='text-muted-foreground'>
              {limit > 0 ? limit : t('Unlimited')}
            </span>
          )
        },
        size: 100,
      },
      {
        id: 'duration',
        meta: { label: t('Validity') },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Validity')} />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatDuration(row.original.plan, t)}
          </span>
        ),
        size: 100,
      },
      {
        id: 'reset',
        meta: { label: t('Quota Reset'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Quota Reset')} />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatResetPeriod(row.original.plan, t)}
          </span>
        ),
        size: 80,
      },
      {
        accessorFn: (row) => row.plan.sort_order,
        id: 'sort_order',
        meta: { label: t('Priority'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Priority')} />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {row.original.plan.sort_order}
          </span>
        ),
        size: 80,
      },
      {
        accessorFn: (row) => row.plan.enabled,
        id: 'enabled',
        meta: { label: t('Status'), mobileBadge: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) =>
          row.original.plan.enabled ? (
            <StatusBadge
              label={t('Enable')}
              variant='success'
              copyable={false}
            />
          ) : (
            <StatusBadge
              label={t('Disable')}
              variant='neutral'
              copyable={false}
            />
          ),
        size: 80,
      },
      {
        id: 'payment',
        meta: { label: t('Payment Channel'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Payment Channel')} />
        ),
        cell: ({ row }) => {
          const plan = row.original.plan
          return (
            <div className='flex gap-1'>
              {plan.stripe_price_id && (
                <StatusBadge
                  label='Stripe'
                  variant='neutral'
                  copyable={false}
                />
              )}
              {plan.creem_product_id && (
                <StatusBadge label='Creem' variant='neutral' copyable={false} />
              )}
              {enableEpay && (
                <StatusBadge
                  label={t('Epay')}
                  variant='neutral'
                  copyable={false}
                />
              )}
            </div>
          )
        },
        size: 140,
      },
      {
        id: 'total_amount',
        meta: { label: t('Total Quota'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Total Quota')} />
        ),
        cell: ({ row }) => {
          const total = Number(row.original.plan.total_amount || 0)
          return (
            <span className='text-muted-foreground'>
              {total > 0 ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className='cursor-help'>{formatQuota(total)}</span>
                  </TooltipTrigger>
                  <TooltipContent>
                    {t('Raw Quota')}: {total}
                  </TooltipContent>
                </Tooltip>
              ) : (
                t('Unlimited')
              )}
            </span>
          )
        },
        size: 100,
      },
      {
        id: 'limits',
        meta: { label: t('Model and Window Limits'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('Model and Window Limits')}
          />
        ),
        cell: ({ row }) => {
          const plan = row.original.plan
          const restriction = getModelRestrictionMeta(plan, t)
          const windows = getQuotaWindowItems(plan, t, formatQuota)

          if (!restriction && windows.length === 0) {
            return <span className='text-muted-foreground'>{t('None')}</span>
          }

          return (
            <div className='max-w-[220px] space-y-1 text-xs'>
              {restriction &&
                (restriction.tooltip ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <div className='text-muted-foreground truncate'>
                        {restriction.label}
                      </div>
                    </TooltipTrigger>
                    <TooltipContent>{restriction.tooltip}</TooltipContent>
                  </Tooltip>
                ) : (
                  <div className='text-muted-foreground truncate'>
                    {restriction.label}
                  </div>
                ))}
              {windows.map((item) => (
                <div key={item.label} className='text-muted-foreground truncate'>
                  {item.label}
                </div>
              ))}
            </div>
          )
        },
        size: 220,
      },
      {
        id: 'upgrade_group',
        meta: { label: t('Upgrade Group'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Upgrade Group')} />
        ),
        cell: ({ row }) => {
          const group = row.original.plan.upgrade_group
          if (!group) {
            return <span className='text-muted-foreground'>{t('No Upgrade')}</span>
          }
          return <GroupBadge group={group} />
        },
        size: 100,
      },
      {
        id: 'actions',
        cell: ({ row }) => <DataTableRowActions row={row} />,
        size: 80,
      },
    ],
    [enableEpay, t]
  )
}
