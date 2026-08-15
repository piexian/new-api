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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { LoanAccountsTable } from './components/loan-accounts-table'
import { LoanApplicationsTable } from './components/loan-applications-table'
import { LoanFundingsTable } from './components/loan-fundings-table'
import { LoanOffersTable } from './components/loan-offers-table'
import { LoanRecordsTable } from './components/loan-records-table'
import { MarketOverviewCards } from './components/market-overview-cards'

const LOAN_ADMIN_TABS = [
  'accounts',
  'records',
  'applications',
  'overview',
  'offers',
  'fundings',
] as const

type LoanAdminTab = (typeof LOAN_ADMIN_TABS)[number]

export function LoanAdminPage() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<LoanAdminTab>('accounts')

  const handleTabChange = useCallback((value: string) => {
    if (LOAN_ADMIN_TABS.includes(value as LoanAdminTab)) {
      setActiveTab(value as LoanAdminTab)
    }
  }, [])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Loan Management')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          <Tabs value={activeTab} onValueChange={handleTabChange}>
            <TabsList className='w-full max-w-full sm:w-auto'>
              <TabsTrigger value='accounts'>{t('Accounts')}</TabsTrigger>
              <TabsTrigger value='records'>{t('Records')}</TabsTrigger>
              <TabsTrigger value='applications'>
                {t('Applications')}
              </TabsTrigger>
              <TabsTrigger value='overview'>
                {t('Market Overview')}
              </TabsTrigger>
              <TabsTrigger value='offers'>{t('Offers')}</TabsTrigger>
              <TabsTrigger value='fundings'>{t('Fundings')}</TabsTrigger>
            </TabsList>
          </Tabs>
          {activeTab === 'accounts' ? <LoanAccountsTable /> : null}
          {activeTab === 'records' ? <LoanRecordsTable /> : null}
          {activeTab === 'applications' ? <LoanApplicationsTable /> : null}
          {activeTab === 'overview' ? <MarketOverviewCards /> : null}
          {activeTab === 'offers' ? <LoanOffersTable /> : null}
          {activeTab === 'fundings' ? <LoanFundingsTable /> : null}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}