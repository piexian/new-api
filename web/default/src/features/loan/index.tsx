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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Card, CardContent } from '@/components/ui/card'

import { getLoanStatus } from './api'
import { BorrowForm } from './components/borrow-form'
import { LoanRecordsTable } from './components/loan-records-table'
import { LoanStatusCard } from './components/loan-status-card'
import { OfficerApplications } from './components/officer-applications'
import { RepayForm } from './components/repay-form'
import { TermsDialog } from './components/terms-dialog'

export function LoanPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const {
    data: status,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ['loan-status'],
    queryFn: async () => {
      const res = await getLoanStatus()
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan status')
    },
    staleTime: 10000,
  })

  const termsRequired =
    !!status && status.terms_enabled && !status.terms_agreed

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Token Loan')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-6xl flex-col gap-4 sm:gap-5'>
            {!isLoading && status && !status.enabled ? (
              <Card className='py-0'>
                <CardContent className='text-muted-foreground p-6 text-center text-sm'>
                  {t('The token loan feature is not enabled')}
                </CardContent>
              </Card>
            ) : (
              <>
                <div className='grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]'>
                  <LoanStatusCard
                    status={status}
                    loading={isLoading}
                    error={isError ? error.message : null}
                    onRetry={() => refetch()}
                  />
                  <div className='flex flex-col gap-4'>
                    <BorrowForm status={status} />
                    {status && status.debt > 0 ? (
                      <RepayForm status={status} />
                    ) : null}
                  </div>
                </div>
                <LoanRecordsTable />
                {status?.ai_enabled ? <OfficerApplications /> : null}
              </>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* 与 SectionPageLayout 平级渲染：该布局只渲染 Title/Content 等具名插槽，额外子节点会被丢弃 */}
      <TermsDialog
        open={termsRequired}
        termsText={status?.terms_text ?? ''}
        onAgreed={() =>
          queryClient.invalidateQueries({ queryKey: ['loan-status'] })
        }
      />
    </>
  )
}
