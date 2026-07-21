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
import type { TFunction } from 'i18next'
import { Gavel, ListChecks, ShieldAlert } from 'lucide-react'

import type { NavGroup, SidebarView } from '../types'

/**
 * Sidebar nav groups for the Risk Center nested view.
 *
 * When the URL matches `/risk/*`, the root navigation is replaced by
 * the Risk Center groups below, with a "Back to Dashboard" header.
 */
function getRiskCenterNavGroups(t: TFunction): NavGroup[] {
  return [
    {
      id: 'risk-center',
      title: t('Risk Center'),
      items: [
        {
          title: t('Probe Guard'),
          icon: ShieldAlert,
          items: [
            {
              title: t('Settings'),
              url: '/risk/probe-guard',
            },
            {
              title: t('IP Offenses'),
              url: '/risk/probe-guard/ip-offenses',
            },
            {
              title: t('User Offenses'),
              url: '/risk/probe-guard/user-offenses',
            },
          ],
        },
        {
          title: t('Error Ban'),
          icon: Gavel,
          items: [
            {
              title: t('Settings'),
              url: '/risk/error-ban',
            },
            {
              title: t('IP States'),
              url: '/risk/error-ban/ip-states',
            },
            {
              title: t('User States'),
              url: '/risk/error-ban/user-states',
            },
          ],
        },
        {
          title: t('Ban Logs'),
          icon: ListChecks,
          url: '/risk/ban-logs',
        },
      ],
    },
  ]
}

/**
 * Nested sidebar view for `/risk/*`.
 *
 * Activates the drill-in sidebar: the root navigation is replaced by
 * the Risk Center groups, with a "Back to Dashboard" affordance.
 */
export const RISK_CENTER_VIEW: SidebarView = {
  id: 'risk-center',
  pathPattern: /^\/risk(\/|$)/,
  parent: {
    to: '/dashboard/overview',
    label: 'Back to Dashboard',
  },
  getNavGroups: getRiskCenterNavGroups,
}
