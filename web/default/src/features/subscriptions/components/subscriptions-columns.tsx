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
import { type ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { BadgeCell, DataTableColumnHeader } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useStatus } from '@/hooks/use-status'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'

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
        header: t('ID'),
        meta: { mobileHidden: true },
        cell: ({ row }) => <TableId value={row.original.plan.id} />,
        size: 60,
      },
      {
        accessorFn: (row) => row.plan.title,
        id: 'title',
        header: t('Plan'),
        meta: { mobileTitle: true },
        cell: ({ row }) => {
          const plan = row.original.plan
          const title = plan.title || `#${plan.id}`
          return (
            <div className='w-full max-w-[200px] min-w-0'>
              <div className='truncate font-medium'>{title}</div>
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
        header: t('Price'),
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
        header: t('Validity'),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatDuration(row.original.plan, t)}
          </span>
        ),
        size: 100,
      },
      {
        id: 'reset',
        header: t('Quota Reset'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatResetPeriod(row.original.plan, t)}
          </span>
        ),
        size: 100,
      },
      {
        accessorFn: (row) => row.plan.sort_order,
        id: 'sort_order',
        header: t('Priority'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {row.original.plan.sort_order}
          </span>
        ),
        size: 100,
      },
      {
        accessorFn: (row) => row.plan.enabled,
        id: 'enabled',
        header: t('Status'),
        meta: { mobileBadge: true },
        cell: ({ row }) =>
          row.original.plan.enabled ? (
            <StatusBadge
              label={t('Enable')}
              variant='success'
              copyable={false}
              className='-ml-1.5'
            />
          ) : (
            <StatusBadge
              label={t('Disable')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          ),
        size: 80,
      },
      {
        id: 'payment',
        header: t('Payment Channel'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const plan = row.original.plan
          return (
            <BadgeCell>
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
              {plan.waffo_pancake_product_id && (
                <StatusBadge
                  label='Waffo Pancake'
                  variant='neutral'
                  copyable={false}
                />
              )}
              {enableEpay && (
                <StatusBadge
                  label={t('Epay')}
                  variant='neutral'
                  copyable={false}
                />
              )}
            </BadgeCell>
          )
        },
        size: 140,
      },
      {
        id: 'total_amount',
        header: t('Plan Quota'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const total = Number(row.original.plan.total_amount || 0)
          return (
            <span className='text-muted-foreground'>
              {total > 0 ? (
                <Tooltip>
                  <TooltipTrigger render={<span className='cursor-help' />}>
                    {formatQuota(total)}
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
        size: 150,
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
                    <TooltipTrigger
                      render={
                        <div className='text-muted-foreground truncate' />
                      }
                    >
                      {restriction.label}
                    </TooltipTrigger>
                    <TooltipContent>{restriction.tooltip}</TooltipContent>
                  </Tooltip>
                ) : (
                  <div className='text-muted-foreground truncate'>
                    {restriction.label}
                  </div>
                ))}
              {windows.map((item) => (
                <div
                  key={item.label}
                  className='text-muted-foreground truncate'
                >
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
        header: t('Upgrade Group'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const group = row.original.plan.upgrade_group
          if (!group) {
            return (
              <span className='text-muted-foreground'>{t('No Upgrade')}</span>
            )
          }
          return (
            <BadgeCell>
              <GroupBadge group={group} />
            </BadgeCell>
          )
        },
        size: 120,
      },
      {
        id: 'actions',
        header: () => t('Actions'),
        cell: ({ row }) => <DataTableRowActions row={row} />,
        meta: { pinned: 'right' as const },
      },
    ],
    [enableEpay, t]
  )
}
