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
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

const route = getRouteApi('/_authenticated/usage-logs/$section')

type UsageLogsAuditScope = 'usage' | 'login'

/**
 * Switches the common logs section between regular usage logs and the login
 * audit scope. Switching resets pagination and the type filter so client-side
 * leftovers never leak into the new result set.
 */
export function UsageLogsScopeTabs() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = route.useSearch()

  const scope: UsageLogsAuditScope =
    search.scope === 'login' ? 'login' : 'usage'

  const handleChange = (next: string) => {
    if (next === scope) return
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        // Only keep filters that apply to login records, so stale usage-log
        // filters (model/token/group/channel) do not silently narrow the list.
        scope: next === 'login' ? 'login' : undefined,
        page: 1,
        pageSize: search.pageSize,
        startTime: search.startTime,
        endTime: search.endTime,
        ip: search.ip,
        requestId: search.requestId,
        userAgent: search.userAgent,
        username: search.username,
      },
    })
  }

  return (
    <Tabs value={scope} onValueChange={handleChange} className='w-fit'>
      <TabsList>
        <TabsTrigger value='usage'>{t('Usage Logs')}</TabsTrigger>
        <TabsTrigger value='login'>{t('Login Audit')}</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
