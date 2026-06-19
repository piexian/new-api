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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Gift, Link2, RotateCcw, Share2, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getSelf } from '@/lib/api'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { SectionPageLayout } from '@/components/layout'
import { TransferDialog } from '@/features/wallet/components/dialogs/transfer-dialog'
import {
  getAffiliateCode,
  getInvitedUsers,
  getInviteTopupInfo,
  resetAffiliateCode,
  transferAffiliateQuota,
} from './api'
import type { InvitedUser, InviteRewardsUserData } from './types'

function generateAffiliateLink(affCode: string): string {
  if (!affCode || typeof window === 'undefined') return ''
  return `${window.location.origin}/sign-up?aff=${affCode}`
}

function StatCard({
  label,
  value,
  icon: Icon,
  loading,
}: {
  label: string
  value: string
  icon: typeof Gift
  loading: boolean
}) {
  return (
    <Card className='py-0'>
      <CardContent className='flex min-h-24 items-center justify-between gap-3 p-4'>
        <div className='min-w-0'>
          <p className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
            {label}
          </p>
          {loading ? (
            <Skeleton className='mt-2 h-7 w-24' />
          ) : (
            <p className='mt-1 truncate text-2xl font-semibold tabular-nums'>
              {value}
            </p>
          )}
        </div>
        <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-lg'>
          <Icon className='text-muted-foreground size-5' />
        </div>
      </CardContent>
    </Card>
  )
}

function InvitedUsersList({
  users,
  loading,
}: {
  users: InvitedUser[]
  loading: boolean
}) {
  const { t } = useTranslation()

  return (
    <Card className='py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex items-center gap-3'>
          <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg'>
            <Users className='text-muted-foreground size-4' />
          </div>
          <div className='min-w-0'>
            <CardTitle className='text-lg'>{t('Invited Users')}</CardTitle>
            <CardDescription>
              {t('People who registered through your referral link')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className='p-3 sm:p-4'>
        {loading ? (
          <div className='space-y-2'>
            {Array.from({ length: 3 }).map((_, index) => (
              <Skeleton key={index} className='h-16 rounded-lg' />
            ))}
          </div>
        ) : users.length === 0 ? (
          <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
            {t('No invited users yet')}
          </div>
        ) : (
          <div className='space-y-2'>
            {users.map((user) => (
              <div
                key={user.id}
                className='grid gap-2 rounded-lg border p-3 sm:grid-cols-[minmax(0,1fr)_minmax(150px,max-content)] sm:items-center'
              >
                <div className='min-w-0'>
                  <p className='truncate text-sm font-medium'>
                    {user.display_name || user.username}
                  </p>
                  <p className='text-muted-foreground mt-0.5 truncate text-xs'>
                    @{user.username} · {t('User ID')} #{user.id}
                  </p>
                </div>
                <div className='text-muted-foreground text-xs sm:text-right'>
                  <span className='font-medium'>{t('Joined')}</span>
                  <span className='ml-2'>
                    {user.created_at
                      ? formatTimestampToDate(user.created_at)
                      : '-'}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function InviteRewards() {
  const { t } = useTranslation()
  const [user, setUser] = useState<InviteRewardsUserData | null>(null)
  const [affiliateCode, setAffiliateCode] = useState('')
  const [invitedUsers, setInvitedUsers] = useState<InvitedUser[]>([])
  const [loading, setLoading] = useState(true)
  const [resetting, setResetting] = useState(false)
  const [transferring, setTransferring] = useState(false)
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [resetDialogOpen, setResetDialogOpen] = useState(false)
  const [complianceConfirmed, setComplianceConfirmed] = useState(true)

  const affiliateLink = useMemo(
    () => generateAffiliateLink(affiliateCode),
    [affiliateCode]
  )

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [selfResponse, codeResponse, invitedResponse, topupResponse] =
        await Promise.all([
          getSelf(),
          getAffiliateCode(),
          getInvitedUsers(),
          getInviteTopupInfo(),
        ])

      if (selfResponse.success && selfResponse.data) {
        setUser(selfResponse.data as InviteRewardsUserData)
      }
      if (codeResponse.success && codeResponse.data) {
        setAffiliateCode(codeResponse.data)
      }
      if (invitedResponse.success && Array.isArray(invitedResponse.data)) {
        setInvitedUsers(invitedResponse.data)
      }
      if (topupResponse.success && topupResponse.data) {
        setComplianceConfirmed(
          topupResponse.data.payment_compliance_confirmed !== false
        )
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch invite rewards data:', error)
      toast.error(t('Failed to load invite rewards'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleResetCode = async () => {
    try {
      setResetting(true)
      const response = await resetAffiliateCode()
      if (response.success && response.data) {
        setAffiliateCode(response.data)
        setResetDialogOpen(false)
        toast.success(t('Referral code reset'))
        return
      }
      toast.error(response.message || t('Reset failed'))
    } catch {
      toast.error(t('Reset failed'))
    } finally {
      setResetting(false)
    }
  }

  const handleTransfer = async (amount: number): Promise<boolean> => {
    try {
      setTransferring(true)
      const response = await transferAffiliateQuota({ quota: amount })
      if (response.success) {
        toast.success(response.message || t('Transfer successful'))
        await fetchData()
        return true
      }
      toast.error(response.message || t('Transfer failed'))
      return false
    } catch {
      toast.error(t('Transfer failed'))
      return false
    } finally {
      setTransferring(false)
    }
  }

  const hasRewards = (user?.aff_quota ?? 0) > 0

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Invite Rewards')}</SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Manage referral links, rewards, and invited users')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-6xl flex-col gap-4 sm:gap-5'>
            <div className='grid gap-3 sm:grid-cols-3'>
              <StatCard
                label={t('Pending')}
                value={formatQuota(user?.aff_quota ?? 0)}
                icon={Gift}
                loading={loading}
              />
              <StatCard
                label={t('Total Earned')}
                value={formatQuota(user?.aff_history_quota ?? 0)}
                icon={Share2}
                loading={loading}
              />
              <StatCard
                label={t('Invites')}
                value={String(user?.aff_count ?? invitedUsers.length)}
                icon={Users}
                loading={loading}
              />
            </div>

            <Card className='py-0'>
              <CardHeader className='border-b p-4 sm:p-5'>
                <div className='flex items-center gap-3'>
                  <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg'>
                    <Link2 className='text-muted-foreground size-4' />
                  </div>
                  <div className='min-w-0'>
                    <CardTitle className='text-lg'>
                      {t('Your Referral Link')}
                    </CardTitle>
                    <CardDescription>
                      {t(
                        'Earn rewards when your referrals add funds. Transfer accumulated rewards to your balance anytime.'
                      )}
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className='space-y-4 p-4 sm:p-5'>
                <div className='grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center'>
                  <div className='grid gap-3 sm:grid-cols-[160px_minmax(0,1fr)]'>
                    <div>
                      <p className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                        {t('Referral Code')}
                      </p>
                      {loading ? (
                        <Skeleton className='mt-2 h-9 w-full' />
                      ) : (
                        <Input
                          value={affiliateCode}
                          readOnly
                          className='mt-2 h-9 font-mono text-sm'
                        />
                      )}
                    </div>
                    <div className='min-w-0'>
                      <p className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                        {t('Referral link:')}
                      </p>
                      {loading ? (
                        <Skeleton className='mt-2 h-9 w-full' />
                      ) : (
                        <div className='mt-2 flex min-w-0 items-center gap-2'>
                          <Input
                            value={affiliateLink}
                            readOnly
                            className='h-9 min-w-0 flex-1 font-mono text-xs'
                          />
                          <CopyButton
                            value={affiliateLink}
                            variant='outline'
                            className='size-9 shrink-0'
                            iconClassName='size-4'
                            tooltip={t('Copy referral link')}
                            aria-label={t('Copy referral link')}
                          />
                        </div>
                      )}
                    </div>
                  </div>

                  <div className='flex flex-wrap gap-2 lg:justify-end'>
                    <Button
                      variant='outline'
                      onClick={() => setResetDialogOpen(true)}
                      disabled={loading || resetting}
                    >
                      <RotateCcw className='size-4' />
                      {t('Reset Code')}
                    </Button>
                    {hasRewards && (
                      <Button
                        onClick={() => setTransferDialogOpen(true)}
                        disabled={!complianceConfirmed}
                      >
                        <Gift className='size-4' />
                        {t('Transfer to Balance')}
                      </Button>
                    )}
                  </div>
                </div>

                {!complianceConfirmed ? (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Referral reward transfer is disabled until the administrator confirms compliance terms.'
                    )}
                  </p>
                ) : null}
              </CardContent>
            </Card>

            <InvitedUsersList users={invitedUsers} loading={loading} />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <TransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        onConfirm={handleTransfer}
        availableQuota={user?.aff_quota ?? 0}
        transferring={transferring}
      />

      <ConfirmDialog
        open={resetDialogOpen}
        onOpenChange={setResetDialogOpen}
        title={t('Reset referral code?')}
        desc={t(
          'Resetting your referral code invalidates existing invite links. The users you have already invited will stay linked to your account.'
        )}
        confirmText={t('Reset Code')}
        handleConfirm={handleResetCode}
        isLoading={resetting}
      />
    </>
  )
}
