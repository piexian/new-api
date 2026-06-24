import { useState, useEffect } from 'react'
import { Crown, CalendarClock, Package, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { GroupBadge } from '@/components/group-badge'
import {
  paySubscriptionStripe,
  paySubscriptionCreem,
  paySubscriptionEpay,
  paySubscriptionWallet,
  paySubscriptionWaffoPancake,
} from '../../api'
import { formatDuration, formatResetPeriod } from '../../lib'
import type {
  PlanRecord,
  SubscriptionPurchaseMode,
  UserSubscriptionRecord,
} from '../../types'

interface PaymentMethod {
  type: string
  name?: string
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  enableStripe?: boolean
  enableCreem?: boolean
  enableWaffoPancake?: boolean
  enableOnlineTopUp?: boolean
  epayMethods?: PaymentMethod[]
  purchaseLimit?: number
  purchaseCount?: number
  walletQuota?: number
  activeSubscriptions?: UserSubscriptionRecord[]
  onSuccess?: () => void
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t } = useTranslation()
  const [paying, setPaying] = useState(false)
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('')
  const [confirmWalletOpen, setConfirmWalletOpen] = useState(false)
  const [purchaseMode, setPurchaseMode] =
    useState<SubscriptionPurchaseMode>('concurrent')

  useEffect(() => {
    if (props.open && props.epayMethods && props.epayMethods.length > 0) {
      setSelectedEpayMethod(props.epayMethods[0].type)
    } else if (!props.open) {
      setSelectedEpayMethod('')
    }
  }, [props.open, props.epayMethods])

  useEffect(() => {
    setPurchaseMode('concurrent')
  }, [props.open, props.plan?.plan?.id])

  const plan = props.plan?.plan
  if (!plan) return null

  const hasStripe = props.enableStripe && !!plan.stripe_price_id
  const hasCreem = props.enableCreem && !!plan.creem_product_id
  const hasWaffoPancake =
    props.enableWaffoPancake && !!plan.waffo_pancake_product_id
  const hasEpay =
    props.enableOnlineTopUp && (props.epayMethods || []).length > 0
  const hasWallet = typeof props.walletQuota === 'number'
  const hasAnyPayment =
    hasStripe || hasCreem || hasWaffoPancake || hasEpay || hasWallet
  const requiredQuota = (() => {
    const requiredQuotaFromApi = Number(
      (props.plan as PlanRecord & { required_quota?: number })
        ?.required_quota || 0
    )
    if (Number.isFinite(requiredQuotaFromApi) && requiredQuotaFromApi >= 0) {
      return requiredQuotaFromApi
    }
    return 0
  })()
  const walletBalance = props.walletQuota ?? 0
  const walletSufficient = walletBalance >= requiredQuota
  const selectedEpayMethodLabel =
    (props.epayMethods || []).find((m) => m.type === selectedEpayMethod)
      ?.name ||
    selectedEpayMethod ||
    t('Select payment method')
  const totalAmount = Number(plan.total_amount || 0)
  const price = formatBillingCurrencyFromUSD(Number(plan.price_amount || 0))
  const limitReached =
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)
  const hasActiveSamePlan = (props.activeSubscriptions || []).some(
    (record) => record?.subscription?.plan_id === plan.id
  )
  const purchaseModeLabel =
    purchaseMode === 'renew' ? t('Renew') : t('Use Together')

  const handlePayStripe = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionStripe({
        plan_id: plan.id,
        purchase_mode: purchaseMode,
      })
      if (res.message === 'success' && res.data?.pay_link) {
        window.open(res.data.pay_link, '_blank')
        toast.success(t('Payment page opened'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayCreem = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionCreem({
        plan_id: plan.id,
        purchase_mode: purchaseMode,
      })
      if (res.message === 'success' && res.data?.checkout_url) {
        window.open(res.data.checkout_url, '_blank')
        toast.success(t('Payment page opened'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayWallet = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionWallet({
        plan_id: plan.id,
        purchase_mode: purchaseMode,
      })
      if (res.success) {
        toast.success(t('Wallet payment successful'))
        props.onSuccess?.()
        props.onOpenChange(false)
      } else {
        toast.error(res.message || t('Wallet payment failed'))
      }
    } catch {
      toast.error(t('Wallet payment failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayWaffoPancake = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionWaffoPancake({
        plan_id: plan.id,
        purchase_mode: purchaseMode,
      })
      if (res.message === 'success' && res.data?.checkout_url) {
        toast.success(t('Redirecting to payment page...'))
        window.location.href = res.data.checkout_url
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const isSafari =
    typeof navigator !== 'undefined' &&
    /^((?!chrome|android).)*safari/i.test(navigator.userAgent)

  const handlePayEpay = async () => {
    if (!selectedEpayMethod) {
      toast.error(t('Please select a payment method'))
      return
    }
    setPaying(true)
    try {
      const res = await paySubscriptionEpay({
        plan_id: plan.id,
        payment_method: selectedEpayMethod,
        purchase_mode: purchaseMode,
      })
      if (res.message === 'success' && res.url) {
        const form = document.createElement('form')
        form.action = res.url
        form.method = 'POST'
        if (!isSafari) {
          form.target = '_blank'
        }
        Object.entries(res.data || {}).forEach(([key, value]) => {
          const input = document.createElement('input')
          input.type = 'hidden'
          input.name = key
          input.value = String(value)
          form.appendChild(input)
        })
        document.body.appendChild(form)
        form.submit()
        document.body.removeChild(form)
        toast.success(t('Payment initiated'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  return (
    <>
      <Dialog open={props.open} onOpenChange={props.onOpenChange}>
        <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Crown className='h-5 w-5' />
              {t('Purchase Subscription')}
            </DialogTitle>
          </DialogHeader>

          <div className='space-y-3 sm:space-y-4'>
            <div className='bg-muted/50 space-y-2.5 rounded-lg border p-3 sm:space-y-3 sm:p-4'>
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Plan Name')}
                </span>
                <span className='max-w-[200px] truncate text-sm font-medium'>
                  {plan.title}
                </span>
              </div>
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Validity Period')}
                </span>
                <span className='flex items-center gap-1 text-sm'>
                  <CalendarClock className='h-3.5 w-3.5' />
                  {formatDuration(plan, t)}
                </span>
              </div>
              {formatResetPeriod(plan, t) !== t('No Reset') && (
                <div className='flex justify-between'>
                  <span className='text-muted-foreground text-sm'>
                    {t('Reset Period')}
                  </span>
                  <span className='text-sm'>{formatResetPeriod(plan, t)}</span>
                </div>
              )}
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Total Quota')}
                </span>
                <span className='flex items-center gap-1 text-sm'>
                  <Package className='h-3.5 w-3.5' />
                  {totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')}
                </span>
              </div>
              {plan.upgrade_group && (
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground text-sm'>
                    {t('Upgrade Group')}
                  </span>
                  <GroupBadge group={plan.upgrade_group} />
                </div>
              )}
              <Separator />
              <div className='flex items-center justify-between'>
                <span className='text-sm font-medium'>{t('Amount Due')}</span>
                <span className='text-primary text-lg font-bold'>{price}</span>
              </div>
            </div>

            {limitReached && (
              <Alert variant='destructive'>
                <AlertDescription>
                  {t('Purchase limit reached')} ({props.purchaseCount}/
                  {props.purchaseLimit})
                </AlertDescription>
              </Alert>
            )}

            {hasActiveSamePlan && (
              <div className='space-y-2'>
                <p className='text-muted-foreground text-xs'>
                  {t('Purchase Mode')}
                </p>
                <Tabs
                  value={purchaseMode}
                  onValueChange={(value) =>
                    setPurchaseMode(value as SubscriptionPurchaseMode)
                  }
                >
                  <TabsList className='grid w-full grid-cols-2'>
                    <TabsTrigger value='concurrent'>
                      {t('Use Together')}
                    </TabsTrigger>
                    <TabsTrigger value='renew'>{t('Renew')}</TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
            )}

            {hasAnyPayment ? (
              <div className='space-y-3'>
                <p className='text-muted-foreground text-xs'>
                  {t('Select payment method')}
                </p>
                {(hasStripe || hasCreem || hasWaffoPancake || hasWallet) && (
                  <div className='grid grid-cols-2 gap-2 sm:flex'>
                    {hasWallet && (
                      <Button
                        variant='outline'
                        className='flex-1'
                        onClick={() => setConfirmWalletOpen(true)}
                        disabled={paying || limitReached}
                      >
                        <WalletCards className='mr-1.5 h-3.5 w-3.5' />
                        {t('Wallet Balance')}
                      </Button>
                    )}
                    {hasStripe && (
                      <Button
                        variant='outline'
                        className='flex-1'
                        onClick={handlePayStripe}
                        disabled={paying || limitReached}
                      >
                        Stripe
                      </Button>
                    )}
                    {hasCreem && (
                      <Button
                        variant='outline'
                        className='flex-1'
                        onClick={handlePayCreem}
                        disabled={paying || limitReached}
                      >
                        Creem
                      </Button>
                    )}
                    {hasWaffoPancake && (
                      <Button
                        variant='outline'
                        className='flex-1'
                        onClick={handlePayWaffoPancake}
                        disabled={paying || limitReached}
                      >
                        Waffo Pancake
                      </Button>
                    )}
                  </div>
                )}
                {hasEpay && (
                  <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
                    <Select
                      items={[
                        ...(props.epayMethods || []).map((m) => ({
                          value: m.type,
                          label: m.name || m.type,
                        })),
                      ]}
                      value={selectedEpayMethod}
                      onValueChange={(v) =>
                        v !== null && setSelectedEpayMethod(v)
                      }
                      disabled={limitReached}
                    >
                      <SelectTrigger className='flex-1'>
                        <SelectValue>{selectedEpayMethodLabel}</SelectValue>
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {(props.epayMethods || []).map((m) => (
                            <SelectItem key={m.type} value={m.type}>
                              {m.name || m.type}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <Button
                      onClick={handlePayEpay}
                      disabled={paying || !selectedEpayMethod || limitReached}
                    >
                      {t('Pay')}
                    </Button>
                  </div>
                )}
              </div>
            ) : (
              <Alert>
                <AlertDescription>
                  {t(
                    'Online payment is not enabled. Please contact the administrator.'
                  )}
                </AlertDescription>
              </Alert>
            )}
          </div>
        </DialogContent>
      </Dialog>

      {/* Wallet payment confirmation dialog */}
      <Dialog open={confirmWalletOpen} onOpenChange={setConfirmWalletOpen}>
        <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <WalletCards className='h-5 w-5' />
              {t('Confirm Wallet Payment')}
            </DialogTitle>
          </DialogHeader>
          <div className='space-y-3'>
            <div className='bg-muted/50 space-y-2.5 rounded-lg border p-3 sm:space-y-3 sm:p-4'>
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Plan Name')}
                </span>
                <span className='max-w-[200px] truncate text-sm font-medium'>
                  {plan.title}
                </span>
              </div>
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Current Balance')}
                </span>
                <span className='text-sm font-medium'>
                  {formatQuota(walletBalance)}
                </span>
              </div>
              <Separator />
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Deduction Amount')}
                </span>
                <span className='text-sm font-medium'>
                  {requiredQuota > 0 ? formatQuota(requiredQuota) : t('Free')}
                </span>
              </div>
              {requiredQuota === 0 && (
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'This plan is free. No wallet balance will be deducted, but a subscription order will be created.'
                  )}
                </p>
              )}
              {hasActiveSamePlan && (
                <div className='flex justify-between'>
                  <span className='text-muted-foreground text-sm'>
                    {t('Purchase Mode')}
                  </span>
                  <span className='text-sm font-medium'>
                    {purchaseModeLabel}
                  </span>
                </div>
              )}
            </div>
            {!walletSufficient && requiredQuota > 0 && (
              <Alert variant='destructive'>
                <AlertDescription>
                  {t(
                    'Insufficient balance. Current: {{current}}, Required: {{required}}',
                    {
                      current: formatQuota(walletBalance),
                      required: formatQuota(requiredQuota),
                    }
                  )}
                </AlertDescription>
              </Alert>
            )}
            <div className='flex gap-2'>
              <Button
                variant='outline'
                className='flex-1'
                onClick={() => setConfirmWalletOpen(false)}
                disabled={paying}
              >
                {t('Cancel')}
              </Button>
              <Button
                className='flex-1'
                onClick={() => {
                  setConfirmWalletOpen(false)
                  handlePayWallet()
                }}
                disabled={paying || (!walletSufficient && requiredQuota > 0)}
              >
                {paying ? t('Processing...') : t('Confirm Payment')}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
