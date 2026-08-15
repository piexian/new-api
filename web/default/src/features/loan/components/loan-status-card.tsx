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
import { Landmark } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import { formatPercent } from '@/lib/format'

import type { LoanStatus } from '../types'

import { QueryErrorState } from './query-error'

interface LoanStatusCardProps {
  status?: LoanStatus
  loading: boolean
  error?: string | null
  onRetry?: () => void
}

function StatusItem({
  label,
  value,
  hint,
}: {
  label: string
  value: string
  hint?: string
}) {
  return (
    <div className='space-y-1'>
      <p className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
        {label}
      </p>
      <p className='text-lg font-semibold tabular-nums'>{value}</p>
      {hint ? (
        <p className='text-muted-foreground text-xs'>{hint}</p>
      ) : null}
    </div>
  )
}

export function LoanStatusCard(props: LoanStatusCardProps) {
  const { t } = useTranslation()
  const status = props.status

  // 宽限期截止为服务器本地日序号（unix/86400），仅在未来时展示
  const graceUntil =
    status && status.interest_free_until > 0
      ? dayjs.unix(status.interest_free_until * 86400)
      : null
  const graceActive = graceUntil !== null && graceUntil.isAfter(dayjs())

  const content = (() => {
    if (props.error) {
      return (
        <QueryErrorState
          message={props.error}
          onRetry={props.onRetry ?? (() => {})}
        />
      )
    }

    if (props.loading || !status) {
      return (
        <div className='grid grid-cols-2 gap-4 sm:grid-cols-3'>
          {['a', 'b', 'c', 'd', 'e', 'f'].map((slot) => (
            <Skeleton key={slot} className='h-14 rounded-lg' />
          ))}
        </div>
      )
    }

    return (
      <div className='grid grid-cols-2 gap-x-4 gap-y-5 sm:grid-cols-3'>
        <StatusItem
          label={t('Outstanding Principal')}
          value={formatQuotaWithCurrency(status.principal)}
        />
        <StatusItem
          label={t('Interest (as of now)')}
          value={formatQuotaWithCurrency(status.interest)}
        />
        <StatusItem
          label={t('Total Debt')}
          value={formatQuotaWithCurrency(status.debt)}
        />
        <StatusItem
          label={t('Available to Borrow')}
          value={formatQuotaWithCurrency(status.available)}
          hint={`${t('Credit Limit')}: ${formatQuotaWithCurrency(status.effective_max)}`}
        />
        <StatusItem
          label={t('Daily Rate')}
          value={formatPercent(status.daily_rate * 100)}
        />
        <StatusItem
          label={t('Credit Score')}
          value={String(status.credit_score)}
        />
        <StatusItem
          label={t('Grace Period')}
          value={
            graceActive && graceUntil
              ? graceUntil.format('YYYY-MM-DD')
              : t('None')
          }
          hint={
            graceActive ? t('No interest accrues before this date') : undefined
          }
        />
        <StatusItem
          label={t('Total Borrowed')}
          value={formatQuotaWithCurrency(status.total_borrowed)}
        />
        <StatusItem
          label={t('Total Repaid')}
          value={formatQuotaWithCurrency(status.total_repaid)}
        />
      </div>
    )
  })()

  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex items-center gap-3'>
          <IconBadge tone='neutral' size='lg'>
            <Landmark className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
          </IconBadge>
          <div className='min-w-0'>
            <CardTitle className='text-lg'>{t('Loan Status')}</CardTitle>
            <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
              {t('Interest is projected as of now and accrues daily')}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent className='p-4 sm:p-5'>{content}</CardContent>
    </Card>
  )
}
