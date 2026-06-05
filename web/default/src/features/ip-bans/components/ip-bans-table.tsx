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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
} from '@/components/data-table'
import { getIPBans, searchIPBans } from '../api'
import { IP_BAN_TYPE_VALUES, getIPBanTypeOptions } from '../constants'
import type { IPBan, IPBanType } from '../types'
import { useIPBansColumns } from './ip-bans-columns'
import { useIPBans } from './ip-bans-provider'

const route = getRouteApi('/_authenticated/ip-bans/')

function getTypeFilter(
  columnFilters: { id: string; value: unknown }[]
): IPBanType | '' {
  const value = columnFilters.find((filter) => filter.id === 'type')?.value
  if (!Array.isArray(value) || value.length === 0) return ''
  const candidate = String(value[0])
  return IP_BAN_TYPE_VALUES.includes(candidate as IPBanType)
    ? (candidate as IPBanType)
    : ''
}

function isExpiredIPBanRow(ban: IPBan) {
  return ban.expires_at > 0 && ban.expires_at <= Math.floor(Date.now() / 1000)
}

export function IPBansTable() {
  const { t } = useTranslation()
  const columns = useIPBansColumns()
  const { refreshTrigger } = useIPBans()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [{ columnId: 'type', searchKey: 'type', type: 'array' }],
  })

  const selectedType = getTypeFilter(columnFilters)

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'ip-bans',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      selectedType,
      refreshTrigger,
    ],
    queryFn: async () => {
      const hasFilter = globalFilter?.trim()
      const params = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        type: selectedType,
      }

      const result = hasFilter
        ? await searchIPBans({ ...params, keyword: globalFilter })
        : await getIPBans(params)

      if (!result.success) {
        toast.error(result.message || t('Failed to load IP ban rules'))
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const ipBans = data?.items || []

  const table = useReactTable({
    data: ipBans,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
      globalFilter,
      pagination,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    globalFilterFn: (row, _columnId, filterValue) => {
      const searchValue = String(filterValue).toLowerCase()
      return [row.original.target, row.original.reason].some((field) =>
        String(field || '')
          .toLowerCase()
          .includes(searchValue)
      )
    },
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    pageCount: Math.ceil((data?.total || 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  const typeOptions = useMemo(() => getIPBanTypeOptions(t), [t])

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No IP Ban Rules Found')}
      emptyDescription={t(
        'No IP ban rules available. Add a rule to block abusive traffic.'
      )}
      skeletonKeyPrefix='ip-bans-skeleton'
      toolbarProps={{
        searchPlaceholder: t('Filter by IP, CIDR or reason...'),
        filters: [
          {
            columnId: 'type',
            title: t('Type'),
            options: typeOptions,
            singleSelect: true,
          },
        ],
      }}
      getRowClassName={(row, { isMobile }) =>
        isExpiredIPBanRow(row.original)
          ? isMobile
            ? DISABLED_ROW_MOBILE
            : DISABLED_ROW_DESKTOP
          : undefined
      }
    />
  )
}
