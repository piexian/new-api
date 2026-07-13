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
import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import type { SubscriptionPurchaseMode } from '@/features/subscriptions/types'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { getSelf } from '@/lib/api'

import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { CreemConfirmDialog } from './components/dialogs/creem-confirm-dialog'
import { PaymentConfirmDialog } from './components/dialogs/payment-confirm-dialog'
import { MySubscriptionsCard } from './components/my-subscriptions-card'
import { RechargeFormCard } from './components/recharge-form-card'
import { SubscriptionPlansCard } from './components/subscription-plans-card'
import { WalletStatsCard } from './components/wallet-stats-card'
import { DEFAULT_DISCOUNT_RATE } from './constants'
import {
  useTopupInfo,
  usePayment,
  useRedemption,
  useCreemPayment,
  useWaffoPayment,
  useWaffoPancakePayment,
} from './hooks'
import {
  getDefaultPaymentType,
  getMinTopupAmount,
  isWaffoPancakePayment,
} from './lib'
import type {
  UserWalletData,
  PaymentMethod,
  PresetAmount,
  CreemProduct,
} from './types'

interface WalletProps {
  initialShowHistory?: boolean
  paymentStatus?: 'success' | 'fail' | 'pending'
}

export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [topupAmount, setTopupAmount] = useState(0)
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [selectedPaymentMethod, setSelectedPaymentMethod] =
    useState<PaymentMethod>()
  const [paymentLoading, setPaymentLoading] = useState<string | null>(null)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [redemptionPurchaseMode, setRedemptionPurchaseMode] =
    useState<SubscriptionPurchaseMode>('concurrent')
  const [creemDialogOpen, setCreemDialogOpen] = useState(false)
  const [selectedCreemProduct, setSelectedCreemProduct] =
    useState<CreemProduct | null>(null)
  const [showSubscriptionPanel, setShowSubscriptionPanel] = useState(true)
  const [subscriptionRefreshKey, setSubscriptionRefreshKey] = useState(0)

  // Track if payment was initiated to enable auto-refresh
  const paymentInitiatedRef = useRef(false)
  const lastBalanceRef = useRef<number | null>(null)

  const { status } = useStatus()
  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading: topupLoading } = useTopupInfo()

  // Calculate effective exchange rate - when display type is USD, use rate of 1
  const effectiveUsdExchangeRate = useMemo(() => {
    return currency?.quotaDisplayType === 'USD'
      ? 1
      : currency?.usdExchangeRate || 1
  }, [currency?.quotaDisplayType, currency?.usdExchangeRate])
  const {
    amount: paymentAmount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
  } = usePayment()
  const { redeeming, redeemCode } = useRedemption()
  const { processing: creemProcessing, processCreemPayment } = useCreemPayment()
  const { processWaffoPayment } = useWaffoPayment()
  const { processing: pancakeProcessing, processWaffoPancakePayment } =
    useWaffoPancakePayment()

  // Fetch and refresh user data
  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
    } finally {
      setUserLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  // Auto-refresh user data when page becomes visible after payment
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (
        document.visibilityState === 'visible' &&
        paymentInitiatedRef.current
      ) {
        // Delay slightly to allow backend to process callback
        setTimeout(() => {
          fetchUser()
        }, 500)
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () =>
      document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [fetchUser])

  // Show notification when balance changes after payment
  useEffect(() => {
    if (user && lastBalanceRef.current !== null) {
      const balanceChanged = user.quota !== lastBalanceRef.current
      if (balanceChanged && paymentInitiatedRef.current) {
        // Balance updated successfully
        paymentInitiatedRef.current = false
        // Toast notification handled by backend response
      }
    }
    if (user) {
      lastBalanceRef.current = user.quota
    }
  }, [user])

  useEffect(() => {
    if (props.initialShowHistory) {
      setBillingDialogOpen(true)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory])

  // Handle payment status from URL (after epay redirect)
  useEffect(() => {
    if (!props.paymentStatus) return

    // Clear URL parameter
    const url = new URL(window.location.href)
    url.searchParams.delete('pay')
    window.history.replaceState({}, '', url.pathname + url.search)

    // Refresh user data immediately
    fetchUser()

    // Show appropriate toast based on status
    switch (props.paymentStatus) {
      case 'success':
        toast.success(t('Payment completed! Balance updated.'))
        break
      case 'pending':
        toast.warning(
          t(
            'Payment is still processing. Please wait a moment and refresh manually if needed.'
          )
        )
        // Keep polling for pending payments
        paymentInitiatedRef.current = true
        const pollInterval = setInterval(async () => {
          if (!paymentInitiatedRef.current) {
            clearInterval(pollInterval)
            return
          }
          await fetchUser()
        }, 5000)
        setTimeout(() => {
          clearInterval(pollInterval)
          paymentInitiatedRef.current = false
        }, 120000) // Poll for 2 minutes for pending payments
        break
      case 'fail':
        toast.error(t('Payment failed. Please try again.'))
        break
    }
  }, [props.paymentStatus, fetchUser, t])

  // Initialize topup amount when topup info is loaded
  useEffect(() => {
    if (topupInfo && topupAmount === 0) {
      const minTopup = getMinTopupAmount(topupInfo)
      setTopupAmount(minTopup)

      // Calculate initial payment amount with default payment type
      const defaultPaymentType = getDefaultPaymentType(topupInfo)
      calculatePaymentAmount(minTopup, defaultPaymentType)
    }
  }, [topupInfo, topupAmount, calculatePaymentAmount])

  // Get current payment type (selected or default)
  const getCurrentPaymentType = useCallback(() => {
    return selectedPaymentMethod?.type || getDefaultPaymentType(topupInfo)
  }, [selectedPaymentMethod, topupInfo])

  // Handle preset selection
  const handleSelectPreset = (preset: PresetAmount) => {
    setTopupAmount(preset.value)
    setSelectedPreset(preset.value)
    calculatePaymentAmount(preset.value, getCurrentPaymentType())
  }

  // Handle topup amount change
  const handleTopupAmountChange = (amount: number) => {
    setTopupAmount(amount)
    setSelectedPreset(null)
    calculatePaymentAmount(amount, getCurrentPaymentType())
  }

  // Handle payment method selection
  const handlePaymentMethodSelect = async (method: PaymentMethod) => {
    setSelectedPaymentMethod(method)
    setPaymentLoading(method.type)

    try {
      // Validate minimum topup
      const minTopup = getMinTopupAmount(topupInfo)
      if (topupAmount < minTopup) {
        return
      }

      // Calculate payment amount and show confirmation dialog
      await calculatePaymentAmount(topupAmount, method.type)
      setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Handle payment confirmation
  const handlePaymentConfirm = async () => {
    if (!selectedPaymentMethod) return

    const isPancake = isWaffoPancakePayment(selectedPaymentMethod.type)
    const success = isPancake
      ? await processWaffoPancakePayment(topupAmount)
      : await processPayment(topupAmount, selectedPaymentMethod.type)

    if (success) {
      // Mark that payment was initiated - will auto-refresh when user returns
      paymentInitiatedRef.current = true
      setConfirmDialogOpen(false)

      // Start polling for balance changes (fallback if visibility API doesn't work)
      const pollInterval = setInterval(async () => {
        if (!paymentInitiatedRef.current) {
          clearInterval(pollInterval)
          return
        }
        await fetchUser()
      }, 5000) // Poll every 5 seconds

      // Stop polling after 5 minutes
      setTimeout(() => {
        clearInterval(pollInterval)
        paymentInitiatedRef.current = false
      }, 300000)
    }
  }

  // Handle redemption
  const handleRedeem = async () => {
    if (!redemptionCode) return

    const success = await redeemCode(redemptionCode, redemptionPurchaseMode)
    if (success) {
      setRedemptionCode('')
      setRedemptionPurchaseMode('concurrent')
      await fetchUser()
      setSubscriptionRefreshKey((key) => key + 1)
    }
  }

  // Handle Creem product selection
  const handleCreemProductSelect = (product: CreemProduct) => {
    setSelectedCreemProduct(product)
    setCreemDialogOpen(true)
  }

  // Handle Creem payment confirmation
  const handleCreemConfirm = async () => {
    if (!selectedCreemProduct) return

    const success = await processCreemPayment(selectedCreemProduct.productId)
    if (success) {
      setCreemDialogOpen(false)
      setSelectedCreemProduct(null)
      await fetchUser()
    }
  }

  const handleWaffoMethodSelect = async (_method: unknown, index: number) => {
    const loadingKey = `waffo-${index}`
    setPaymentLoading(loadingKey)

    try {
      const success = await processWaffoPayment(topupAmount, index)
      if (success) {
        // Mark that payment was initiated
        paymentInitiatedRef.current = true

        // Start polling for balance changes
        const pollInterval = setInterval(async () => {
          if (!paymentInitiatedRef.current) {
            clearInterval(pollInterval)
            return
          }
          await fetchUser()
        }, 5000)

        // Stop polling after 5 minutes
        setTimeout(() => {
          clearInterval(pollInterval)
          paymentInitiatedRef.current = false
        }, 300000)
      }
    } finally {
      setPaymentLoading(null)
    }
  }

  // Get discount rate for current topup amount
  const getDiscountRate = useCallback(() => {
    return topupInfo?.discount?.[topupAmount] || DEFAULT_DISCOUNT_RATE
  }, [topupInfo, topupAmount])

  const handleSubscriptionAvailabilityChange = useCallback(
    (available: boolean) => {
      setShowSubscriptionPanel(available)
    },
    []
  )

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Wallet')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <WalletStatsCard user={user} loading={userLoading} />

            <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.82fr)] lg:items-start xl:grid-cols-[minmax(0,2fr)_minmax(320px,0.8fr)]'>
              <div
                className={
                  showSubscriptionPanel
                    ? 'grid min-w-0 gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)] xl:items-start'
                    : 'min-w-0'
                }
              >
                <div id='wallet-add-funds' className='min-w-0 scroll-mt-4'>
                  <RechargeFormCard
                    topupInfo={topupInfo}
                    presetAmounts={presetAmounts}
                    selectedPreset={selectedPreset}
                    onSelectPreset={handleSelectPreset}
                    topupAmount={topupAmount}
                    onTopupAmountChange={handleTopupAmountChange}
                    paymentAmount={paymentAmount}
                    calculating={calculating}
                    onPaymentMethodSelect={handlePaymentMethodSelect}
                    paymentLoading={paymentLoading}
                    redemptionCode={redemptionCode}
                    onRedemptionCodeChange={setRedemptionCode}
                    redemptionPurchaseMode={redemptionPurchaseMode}
                    onRedemptionPurchaseModeChange={setRedemptionPurchaseMode}
                    onRedeem={handleRedeem}
                    redeeming={redeeming}
                    topupLink={topupInfo?.topup_link}
                    loading={topupLoading}
                    priceRatio={(status?.price as number) || 1}
                    usdExchangeRate={effectiveUsdExchangeRate}
                    onOpenBilling={() => setBillingDialogOpen(true)}
                    creemProducts={topupInfo?.creem_products}
                    enableCreemTopup={topupInfo?.enable_creem_topup}
                    onCreemProductSelect={handleCreemProductSelect}
                    enableWaffoTopup={topupInfo?.enable_waffo_topup}
                    waffoPayMethods={topupInfo?.waffo_pay_methods}
                    waffoMinTopup={topupInfo?.waffo_min_topup}
                    onWaffoMethodSelect={handleWaffoMethodSelect}
                    enableWaffoPancakeTopup={
                      topupInfo?.enable_waffo_pancake_topup
                    }
                  />
                </div>

                <SubscriptionPlansCard
                  topupInfo={topupInfo}
                  onAvailabilityChange={handleSubscriptionAvailabilityChange}
                  walletQuota={user?.quota}
                  onWalletPaySuccess={fetchUser}
                  refreshKey={subscriptionRefreshKey}
                  hideSubscriptions
                />
              </div>

              <div className='min-w-0'>
                <MySubscriptionsCard
                  refreshKey={subscriptionRefreshKey}
                  compact
                />
              </div>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <PaymentConfirmDialog
        open={confirmDialogOpen}
        onOpenChange={setConfirmDialogOpen}
        onConfirm={handlePaymentConfirm}
        topupAmount={topupAmount}
        paymentAmount={paymentAmount}
        paymentMethod={selectedPaymentMethod}
        calculating={calculating}
        processing={processing || pancakeProcessing}
        discountRate={getDiscountRate()}
        usdExchangeRate={effectiveUsdExchangeRate}
      />

      <BillingHistoryDialog
        open={billingDialogOpen}
        onOpenChange={setBillingDialogOpen}
      />

      <CreemConfirmDialog
        open={creemDialogOpen}
        onOpenChange={setCreemDialogOpen}
        onConfirm={handleCreemConfirm}
        product={selectedCreemProduct}
        processing={creemProcessing}
      />
    </>
  )
}
