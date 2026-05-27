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
import { useMemo } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowLeft } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { useLayout } from '@/context/layout-provider'
import { useSidebarConfig } from '@/hooks/use-sidebar-config'
import { useSidebarData } from '@/hooks/use-sidebar-data'
import { DASHBOARD_DEFAULT_SECTION } from '@/features/dashboard/section-registry'
import {
  Sidebar,
  SidebarContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  SidebarSeparator,
  useSidebar,
} from '@/components/ui/sidebar'
import {
  getNavGroupsForPath,
  isInWorkspace,
  WORKSPACE_IDS,
} from '../lib/workspace-registry'
import { NavGroup } from './nav-group'

/**
 * Application sidebar component
 * Fetches corresponding navigation menu from workspace registry based on current path
 * Dynamically filters navigation items based on backend SidebarModulesAdmin configuration
 *
 * Automatically matches workspace configuration for current path through workspace registry system
 * Adding new workspaces only requires registration in workspace-registry.ts
 */
export function AppSidebar() {
  const { t } = useTranslation()
  const { collapsible, variant } = useLayout()
  const { pathname } = useLocation()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const sidebarData = useSidebarData()
  const isSystemSettings = isInWorkspace(
    pathname,
    WORKSPACE_IDS.SYSTEM_SETTINGS
  )

  // Get navigation group configuration corresponding to current path from workspace registry
  const allNavGroups = getNavGroupsForPath(pathname, t) || sidebarData.navGroups

  // Filter sidebar navigation items based on backend configuration
  const configFilteredNavGroups = useSidebarConfig(allNavGroups)

  // Filter navigation groups based on user role
  // Non-Admin users cannot see Admin navigation group
  const currentNavGroups = useMemo(() => {
    const isAdmin = userRole && userRole >= ROLE.ADMIN
    return configFilteredNavGroups.filter((group) => {
      if (group.id === 'admin') {
        return isAdmin
      }
      return true
    })
  }, [configFilteredNavGroups, userRole])

  return (
    <Sidebar collapsible={collapsible} variant={variant}>
      <SidebarContent className='py-2'>
        {isSystemSettings && <SystemSettingsBackItem />}
        {currentNavGroups.map((props) => {
          const key = props.id || props.title
          return <NavGroup key={key} {...props} />
        })}
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}

function SystemSettingsBackItem() {
  const { t } = useTranslation()
  const { setOpenMobile } = useSidebar()

  return (
    <>
      <SidebarMenu className='px-2'>
        <SidebarMenuItem>
          <SidebarMenuButton
            tooltip={t('Back')}
            render={
              <Link
                to='/dashboard/$section'
                params={{ section: DASHBOARD_DEFAULT_SECTION }}
                onClick={() => setOpenMobile(false)}
              />
            }
          >
            <ArrowLeft />
            <span>{t('Back')}</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
      <SidebarSeparator className='my-1' />
    </>
  )
}
