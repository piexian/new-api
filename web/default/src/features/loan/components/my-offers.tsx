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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatPercent, formatTimestamp } from '@/lib/format'

import {
  closeLoanMarketOffer,
  getLoanMarketOffers,
  pauseLoanMarketOffer,
  resumeLoanMarketOffer,
  withdrawLoanMarketOffer,
} from '../api'
import type { LoanOffer, LoanOfferStatus } from '../types'
import { CreateOfferDialog } from './create-offer-dialog'
import { LenderDisclaimerDialog } from './lender-disclaimer-dialog'
import { QueryErrorState } from './query-error'

type OfferAction = 'pause' | 'resume' | 'close' | 'withdraw'

interface MyOffersProps {
  disclaimerAgreed: boolean
  onDisclaimerAgreed: () => void
}

function useModeLabel() {
  const { t } = useTranslation()
  return (mode: LoanOffer['mode']) => {
    if (mode === 'pool') return t('Pool')
    if (mode === 'ai') return t('AI')
    return t('Order')
  }
}

function useStatusLabel() {
  const { t } = useTranslation()
  return (status: LoanOfferStatus) => {
    if (status === 'active') return t('Active')
    if (status === 'paused') return t('Paused')
    return t('Closed')
  }
}

function statusBadgeVariant(
  status: LoanOfferStatus
): 'default' | 'secondary' | 'outline' {
  if (status === 'active') return 'default'
  if (status === 'paused') return 'secondary'
  return 'outline'
}

function formatDailyRate(rate: number): string {
  return formatPercent(rate * 100)
}

export function MyOffers(props: MyOffersProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const modeLabel = useModeLabel()
  const statusLabel = useStatusLabel()

  const [createOpen, setCreateOpen] = useState(false)
  const [disclaimerOpen, setDisclaimerOpen] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<{
    offer: LoanOffer
    action: OfferAction
  } | null>(null)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-market-offers'],
    queryFn: async () => {
      const res = await getLoanMarketOffers()
      if (res.success && res.data) {
        return res.data.offers
      }
      throw new Error(res.message || 'Failed to fetch loan offers')
    },
    staleTime: 10000,
  })

  const offers = data ?? []

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['loan-market-offers'] })
    queryClient.invalidateQueries({ queryKey: ['loan-market-fundings'] })
    queryClient.invalidateQueries({ queryKey: ['user-self-quota'] })
  }

  const runAction = async (
    offer: LoanOffer,
    action: OfferAction,
    onSuccess?: (result: { refunded?: number }) => void
  ) => {
    try {
      let res
      if (action === 'pause') res = await pauseLoanMarketOffer(offer.id)
      else if (action === 'resume') res = await resumeLoanMarketOffer(offer.id)
      else if (action === 'close') res = await closeLoanMarketOffer(offer.id)
      else res = await withdrawLoanMarketOffer(offer.id)
      if (res.success) {
        toast.success(actionSuccessMessage(action, t))
        invalidate()
        onSuccess?.(res.data as { refunded?: number })
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Operation failed'))
    }
  }

  const handleConfirm = async () => {
    const target = confirmTarget
    if (!target) return
    setConfirmTarget(null)
    const { offer, action } = target
    if (action === 'withdraw') {
      await runAction(offer, action, (result) => {
        if (result?.refunded !== undefined) {
          toast.info(
            t('{{amount}} refunded to your balance', {
              amount: formatQuotaWithCurrency(result.refunded),
            })
          )
        }
      })
      return
    }
    await runAction(offer, action)
  }

  const handleCreateClick = () => {
    if (!props.disclaimerAgreed) {
      setDisclaimerOpen(true)
      return
    }
    setCreateOpen(true)
  }

  const content = (() => {
    if (isError) {
      return (
        <QueryErrorState message={error.message} onRetry={() => refetch()} />
      )
    }

    if (isLoading) {
      return (
        <div className='space-y-2'>
          {['a', 'b', 'c'].map((slot) => (
            <Skeleton key={slot} className='h-10 rounded-lg' />
          ))}
        </div>
      )
    }

    if (offers.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No offers yet. Create one to start lending.')}
        </div>
      )
    }

    return (
      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Mode')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Total Amount')}</TableHead>
              <TableHead>{t('Available')}</TableHead>
              <TableHead>{t('Total Lent')}</TableHead>
              <TableHead>{t('Interest Earned')}</TableHead>
              <TableHead>{t('Rate')}</TableHead>
              <TableHead>{t('Min Credit')}</TableHead>
              <TableHead>{t('Created At')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {offers.map((offer) => (
              <TableRow key={offer.id}>
                <TableCell>
                  <div className='flex flex-col gap-0.5'>
                    <Badge variant='outline'>{modeLabel(offer.mode)}</Badge>
                    {offer.mode === 'ai' && offer.per_loan_cap > 0 ? (
                      <span className='text-muted-foreground text-xs'>
                        {t('Cap')}:{' '}
                        {formatQuotaWithCurrency(offer.per_loan_cap)}
                      </span>
                    ) : null}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={statusBadgeVariant(offer.status)}>
                    {statusLabel(offer.status)}
                  </Badge>
                </TableCell>
                <TableCell className='font-medium tabular-nums'>
                  {formatQuotaWithCurrency(offer.amount_total)}
                </TableCell>
                <TableCell className='tabular-nums'>
                  {formatQuotaWithCurrency(offer.amount_available)}
                </TableCell>
                <TableCell className='tabular-nums'>
                  {formatQuotaWithCurrency(offer.total_lent)}
                </TableCell>
                <TableCell className='tabular-nums'>
                  {formatQuotaWithCurrency(offer.total_interest_earned)}
                </TableCell>
                <TableCell className='text-muted-foreground'>
                  {offer.mode === 'ai'
                    ? `${formatDailyRate(offer.rate_min)} – ${formatDailyRate(offer.rate_max)}`
                    : formatDailyRate(offer.rate_fixed)}
                </TableCell>
                <TableCell className='text-muted-foreground tabular-nums'>
                  {offer.min_credit_score === -50
                    ? t('No Limit')
                    : offer.min_credit_score}
                </TableCell>
                <TableCell className='text-muted-foreground text-xs'>
                  {formatTimestamp(offer.created_at)}
                </TableCell>
                <TableCell className='text-right'>
                  <div className='flex flex-wrap items-center justify-end gap-1.5'>
                    {offer.status === 'active' ? (
                      <Button
                        variant='outline'
                        size='sm'
                        onClick={() =>
                          setConfirmTarget({ offer, action: 'pause' })
                        }
                      >
                        {t('Pause')}
                      </Button>
                    ) : null}
                    {offer.status === 'paused' ? (
                      <Button
                        variant='outline'
                        size='sm'
                        onClick={() =>
                          setConfirmTarget({ offer, action: 'resume' })
                        }
                      >
                        {t('Resume')}
                      </Button>
                    ) : null}
                    {offer.status !== 'closed' ? (
                      <>
                        <Button
                          variant='outline'
                          size='sm'
                          disabled={offer.amount_available <= 0}
                          onClick={() =>
                            setConfirmTarget({ offer, action: 'withdraw' })
                          }
                        >
                          {t('Withdraw')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() =>
                            setConfirmTarget({ offer, action: 'close' })
                          }
                        >
                          {t('Close')}
                        </Button>
                      </>
                    ) : null}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    )
  })()

  return (
    <>
      <div className='flex items-center justify-between gap-3'>
        <div className='min-w-0'>
          <p className='text-sm font-medium'>{t('My Offers')}</p>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t(
              'Offers you placed; idle quota can be withdrawn or the offer closed.'
            )}
          </p>
        </div>
        <Button size='sm' onClick={handleCreateClick} className='shrink-0'>
          <Plus className='h-4 w-4' />
          {t('Create Offer')}
        </Button>
      </div>

      <div className='mt-4'>{content}</div>

      <CreateOfferDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={invalidate}
      />

      <LenderDisclaimerDialog
        open={disclaimerOpen}
        onAgreed={() => {
          setDisclaimerOpen(false)
          setCreateOpen(true)
          props.onDisclaimerAgreed()
        }}
        onCancel={() => setDisclaimerOpen(false)}
      />

      <AlertDialog
        open={confirmTarget !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmTarget(null)
        }}
      >
        <AlertDialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmTarget
                ? confirmTitle(confirmTarget.action, t)
                : t('Confirm')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirmTarget ? confirmDescription(confirmTarget, t) : ''}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirm}>
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function actionSuccessMessage(
  action: OfferAction,
  t: (key: string) => string
): string {
  if (action === 'pause') return t('Offer paused')
  if (action === 'resume') return t('Offer resumed')
  if (action === 'close') return t('Offer closed')
  return t('Offer withdrawn')
}

function confirmTitle(action: OfferAction, t: (key: string) => string): string {
  if (action === 'pause') return t('Pause Offer')
  if (action === 'resume') return t('Resume Offer')
  if (action === 'close') return t('Close Offer')
  return t('Withdraw Offer')
}

function confirmDescription(
  target: { offer: LoanOffer; action: OfferAction },
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  const { offer, action } = target
  const available = formatQuotaWithCurrency(offer.amount_available)
  if (action === 'pause') {
    return t('Pause this offer? It will stop matching new loans.')
  }
  if (action === 'resume') {
    return t('Resume this offer? It will match new loans again.')
  }
  if (action === 'close') {
    return t(
      'Close this offer? This cannot be undone and idle quota is refunded.'
    )
  }
  return t(
    'Withdraw {{available}} idle quota back to your balance? The offer keeps its current status.',
    { available }
  )
}
