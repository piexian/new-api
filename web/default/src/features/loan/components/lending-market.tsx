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
import { useQueryClient } from '@tanstack/react-query'
import { Handshake } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { MarketOffer } from '../types'
import { MarketBrowse } from './market-browse'
import { MyFundings } from './my-fundings'
import { MyOffers } from './my-offers'

interface LendingMarketProps {
  disclaimerAgreed: boolean
  onDisclaimerAgreed: () => void
  onBorrowOrder: (offer: MarketOffer) => void
}

const DEFAULT_TAB = 'offers'

export function LendingMarket(props: LendingMarketProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState(DEFAULT_TAB)

  return (
    <Card className='gap-0 py-0'>
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <CardHeader className='border-b p-4 sm:p-5'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex items-center gap-3'>
              <IconBadge tone='neutral' size='lg'>
                <Handshake className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
              </IconBadge>
              <div className='min-w-0'>
                <CardTitle className='text-lg'>{t('Lending Market')}</CardTitle>
                <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
                  {t('Lend idle quota to other users and earn daily interest')}
                </p>
              </div>
            </div>
            <TabsList className='w-full max-w-full sm:w-auto'>
              <TabsTrigger value='offers'>{t('My Offers')}</TabsTrigger>
              <TabsTrigger value='fundings'>{t('My Fundings')}</TabsTrigger>
              <TabsTrigger value='market'>{t('Market')}</TabsTrigger>
            </TabsList>
          </div>
        </CardHeader>
        <CardContent className='p-4 sm:p-5'>
          <TabsContent value='offers'>
            <MyOffers
              disclaimerAgreed={props.disclaimerAgreed}
              onDisclaimerAgreed={() => {
                queryClient.invalidateQueries({ queryKey: ['loan-status'] })
                props.onDisclaimerAgreed()
              }}
            />
          </TabsContent>
          <TabsContent value='fundings'>
            <MyFundings />
          </TabsContent>
          <TabsContent value='market'>
            <MarketBrowse onBorrow={props.onBorrowOrder} />
          </TabsContent>

          {/* 常驻免责声明条 */}
          <p className='text-muted-foreground mt-4 border-t pt-3 text-xs'>
            {t(
              'Disclaimer: lending in this market is purely for entertainment and not real finance. Lent quota may be lost entirely; the platform does not guarantee repayment or pursue debts.'
            )}
          </p>
        </CardContent>
      </Tabs>
    </Card>
  )
}
