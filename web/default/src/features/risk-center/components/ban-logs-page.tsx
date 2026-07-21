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
import { useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import {
  getRiskBanLogs,
  getRiskBanLogStats,
  type BanLogFilters,
  type BanLogStats,
  type PageData,
  type RiskBanLog,
} from '../api'

export function BanLogsPage() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [dimension, setDimension] = useState('')
  const [source, setSource] = useState('')
  const [keyword, setKeyword] = useState('')
  const [dryRun, setDryRun] = useState('')
  const [selectedLog, setSelectedLog] = useState<RiskBanLog | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  const filters: BanLogFilters = {
    p: page,
    page_size: 10,
    ...(dimension && { dimension }),
    ...(source && { source }),
    ...(keyword && { keyword }),
    ...(dryRun && { dry_run: dryRun }),
  }

  const { data: statsData } = useQuery({
    queryKey: ['risk', 'ban-logs', 'stats'],
    queryFn: async () => {
      const res = await getRiskBanLogStats()
      if (res.success) return res.data
      throw new Error(res.message || t('Failed to load stats'))
    },
    refetchInterval: 30000,
  })

  const { data: logsData, isLoading } = useQuery({
    queryKey: ['risk', 'ban-logs', filters],
    queryFn: async () => {
      const res = await getRiskBanLogs(filters)
      if (res.success) return res.data
      throw new Error(res.message || t('Failed to load ban logs'))
    },
  })

  const stats = statsData as BanLogStats | undefined
  const logs = logsData as PageData<RiskBanLog> | undefined
  const totalPages = logs ? Math.ceil(logs.total / logs.page_size) : 1

  const handleViewDetail = (log: RiskBanLog) => {
    setSelectedLog(log)
    setDialogOpen(true)
  }

  const applyFilter = (
    setter: React.Dispatch<React.SetStateAction<string>>
  ) => (e: React.ChangeEvent<HTMLSelectElement | HTMLInputElement>) => {
    setter(e.target.value)
    setPage(1)
  }

  const truncate = (str: string, maxLen = 50) =>
    str.length > maxLen ? `${str.slice(0, maxLen)}...` : str

  const sourceBadgeVariant = (
    src: RiskBanLog['source']
  ): 'default' | 'secondary' | 'destructive' | 'outline' => {
    switch (src) {
      case 'probe_guard':
        return 'default'
      case 'error_ban':
        return 'destructive'
      case 'ip_middleware':
        return 'secondary'
      case 'manual':
        return 'outline'
    }
  }

  const dimensionBadgeVariant = (
    dim: RiskBanLog['dimension']
  ): 'default' | 'secondary' => (dim === 'ip' ? 'default' : 'secondary')

  const formatTime = (ts: number) =>
    ts
      ? dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
      : '-'

  return (
    <SectionPageLayout>
      <div className="space-y-6">
        {/* Stats Cards */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                {t('Total Bans')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats?.total ?? '-'}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                {t('Dry Run')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {stats?.dry_run_count ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                {t('Permanent')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {stats?.permanent ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                {t('Today')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats?.today ?? '-'}</div>
            </CardContent>
          </Card>
        </div>

        {/* Breakdowns by source and dimension */}
        {stats && (
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">
                  {t('By Dimension')}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-1">
                  {Object.keys(stats.by_dimension ?? {}).length === 0 ? (
                    <span className="text-muted-foreground text-sm">
                      {t('No data')}
                    </span>
                  ) : (
                    Object.entries(stats.by_dimension).map(([key, val]) => (
                      <div
                        key={key}
                        className="flex justify-between text-sm"
                      >
                        <span>{key}</span>
                        <span className="font-medium">{val}</span>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">
                  {t('By Source')}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-1">
                  {Object.keys(stats.by_source ?? {}).length === 0 ? (
                    <span className="text-muted-foreground text-sm">
                      {t('No data')}
                    </span>
                  ) : (
                    Object.entries(stats.by_source).map(([key, val]) => (
                      <div
                        key={key}
                        className="flex justify-between text-sm"
                      >
                        <span>{key}</span>
                        <span className="font-medium">{val}</span>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Filters */}
        <div className="flex flex-wrap items-end gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">{t('Dimension')}</label>
            <select
              className="block h-9 rounded-lg border border-border bg-background px-3 text-sm shadow-xs"
              value={dimension}
              onChange={applyFilter(setDimension)}
            >
              <option value="">{t('All')}</option>
              <option value="ip">{t('IP')}</option>
              <option value="user">{t('User')}</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">{t('Source')}</label>
            <select
              className="block h-9 rounded-lg border border-border bg-background px-3 text-sm shadow-xs"
              value={source}
              onChange={applyFilter(setSource)}
            >
              <option value="">{t('All')}</option>
              <option value="probe_guard">
                {t('Probe Guard')}
              </option>
              <option value="error_ban">{t('Error Ban')}</option>
              <option value="ip_middleware">
                {t('IP Middleware')}
              </option>
              <option value="manual">{t('Manual')}</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">{t('Keyword')}</label>
            <input
              className="block h-9 rounded-lg border border-border bg-background px-3 text-sm shadow-xs"
              placeholder={t('Search IP/Username/Reason...')}
              value={keyword}
              onChange={applyFilter(setKeyword)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">{t('Dry Run')}</label>
            <select
              className="block h-9 rounded-lg border border-border bg-background px-3 text-sm shadow-xs"
              value={dryRun}
              onChange={applyFilter(setDryRun)}
            >
              <option value="">{t('All')}</option>
              <option value="true">{t('Yes')}</option>
              <option value="false">{t('No')}</option>
            </select>
          </div>
        </div>

        {/* Table */}
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="min-w-full divide-y divide-border text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('ID')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Dimension')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Target IP')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Username')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Source')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Action')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Reason')}
                </th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                  {t('Offense Count')}
                </th>
                <th className="px-4 py-3 text-center font-medium text-muted-foreground">
                  {t('Perm')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Unban At')}
                </th>
                <th className="px-4 py-3 text-center font-medium text-muted-foreground">
                  {t('Dry Run')}
                </th>
                <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                  {t('Created At')}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {/* eslint-disable-next-line no-nested-ternary */}
              {isLoading ? (
                <tr>
                  <td
                    colSpan={12}
                    className="px-4 py-8 text-center text-muted-foreground"
                  >
                    {t('Loading...')}
                  </td>
                </tr>
              ) : !logs?.items?.length ? (
                <tr>
                  <td
                    colSpan={12}
                    className="px-4 py-8 text-center text-muted-foreground"
                  >
                    {t('No data')}
                  </td>
                </tr>
              ) : (
                logs.items.map((log) => (
                  <tr
                    key={log.id}
                    className="cursor-pointer transition-colors hover:bg-muted/30"
                    onClick={() => handleViewDetail(log)}
                  >
                    <td className="px-4 py-3">{log.id}</td>
                    <td className="px-4 py-3">
                      <Badge
                        variant={dimensionBadgeVariant(log.dimension)}
                      >
                        {log.dimension}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 font-mono">
                      {log.target_ip}
                    </td>
                    <td className="px-4 py-3">{log.username || '-'}</td>
                    <td className="px-4 py-3">
                      <Badge variant={sourceBadgeVariant(log.source)}>
                        {log.source}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">{log.action}</td>
                    <td
                      className="max-w-[200px] px-4 py-3"
                      title={log.reason}
                    >
                      {truncate(log.reason)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      {log.offense_count}
                    </td>
                    <td className="px-4 py-3 text-center">
                      {log.is_permanent ? t('Yes') : t('No')}
                    </td>
                    <td className="px-4 py-3">
                      {formatTime(log.unban_at)}
                    </td>
                    <td className="px-4 py-3 text-center">
                      {log.dry_run ? (
                        <Badge variant="outline">
                          {t('Dry Run')}
                        </Badge>
                      ) : (
                        '-'
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {dayjs
                        .unix(log.created_at)
                        .format('YYYY-MM-DD HH:mm:ss')}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {logs && logs.total > 0 && (
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {t('Total')}: {logs.total}
            </p>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                {t('Prev')}
              </Button>
              <span className="flex items-center px-2 text-sm text-muted-foreground">
                {page} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          </div>
        )}

        {/* Detail Dialog */}
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t('Ban Log Detail')}</DialogTitle>
            </DialogHeader>
            {selectedLog && (
              <div className="space-y-3 text-sm">
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('ID')}
                  </span>
                  <span className="col-span-2">{selectedLog.id}</span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Dimension')}
                  </span>
                  <span className="col-span-2">
                    <Badge
                      variant={dimensionBadgeVariant(selectedLog.dimension)}
                    >
                      {selectedLog.dimension}
                    </Badge>
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Target IP')}
                  </span>
                  <span className="col-span-2 font-mono">
                    {selectedLog.target_ip}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('User ID')}
                  </span>
                  <span className="col-span-2">{selectedLog.user_id}</span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Username')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.username || '-'}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Source')}
                  </span>
                  <span className="col-span-2">
                    <Badge
                      variant={sourceBadgeVariant(selectedLog.source)}
                    >
                      {selectedLog.source}
                    </Badge>
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Rule ID')}
                  </span>
                  <span className="col-span-2 font-mono">
                    {selectedLog.rule_id}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Rule Name')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.rule_name}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Action')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.action}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Duration (min)')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.duration_minutes}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Is Permanent')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.is_permanent ? t('Yes') : t('No')}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Unban At')}
                  </span>
                  <span className="col-span-2">
                    {formatTime(selectedLog.unban_at)}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Offense Count')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.offense_count}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Reason')}
                  </span>
                  <span className="col-span-2 max-h-24 overflow-y-auto whitespace-pre-wrap">
                    {selectedLog.reason || '-'}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Error Sample')}
                  </span>
                  <span className="col-span-2 max-h-32 overflow-y-auto whitespace-pre-wrap font-mono text-xs">
                    {selectedLog.error_sample || '-'}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Models')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.models || '-'}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Operator ID')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.operator_id}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Dry Run')}
                  </span>
                  <span className="col-span-2">
                    {selectedLog.dry_run ? (
                      <Badge variant="outline">{t('Yes')}</Badge>
                    ) : (
                      t('No')
                    )}
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <span className="font-medium text-muted-foreground">
                    {t('Created At')}
                  </span>
                  <span className="col-span-2">
                    {dayjs
                      .unix(selectedLog.created_at)
                      .format('YYYY-MM-DD HH:mm:ss')}
                  </span>
                </div>
              </div>
            )}
          </DialogContent>
        </Dialog>
      </div>
    </SectionPageLayout>
  )
}
