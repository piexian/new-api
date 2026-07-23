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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { TitledCard } from '@/components/ui/titled-card'

import {
  getProbeGuardConfig,
  getProbeGuardStats,
  updateProbeGuardConfig,
  type ProbeGuardConfig,
} from '../api'
import { RiskWhitelistGroupsField } from './risk-whitelist-groups-field'

export function ProbeGuardPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [config, setConfig] = useState<ProbeGuardConfig | null>(null)
  const [isDirty, setIsDirty] = useState(false)

  const { data: configData, isLoading: configLoading } = useQuery({
    queryKey: ['risk', 'probe-guard', 'config'],
    queryFn: async () => {
      const res = await getProbeGuardConfig()
      if (res.success && res.data) {
        setConfig(res.data)
        return res.data
      }
      throw new Error(res.message || t('Failed to load config'))
    },
  })

  const { data: statsData } = useQuery({
    queryKey: ['risk', 'probe-guard', 'stats'],
    queryFn: async () => {
      const res = await getProbeGuardStats()
      if (res.success) return res.data
      throw new Error(res.message || 'Failed to load stats')
    },
    refetchInterval: 30000,
  })

  const saveMutation = useMutation({
    mutationFn: (data: ProbeGuardConfig) => updateProbeGuardConfig(data),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Settings saved'))
        setIsDirty(false)
        queryClient.invalidateQueries({ queryKey: ['risk', 'probe-guard'] })
      } else {
        toast.error(res.message || t('Failed to save settings'))
      }
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  const updateField = <K extends keyof ProbeGuardConfig>(
    key: K,
    value: ProbeGuardConfig[K]
  ) => {
    setConfig((prev) => {
      if (!prev) return prev
      const next = { ...prev, [key]: value }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  if (configLoading || !config) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Probe Guard')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='text-muted-foreground flex items-center justify-center py-12'>
            {t('Loading...')}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  const bansUser = config.ban_dimension !== 'ip'

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Probe Guard')}</span>
          <Badge variant='outline' className='shrink-0'>
            Root
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {/* Stats cards */}
        <div className='mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>
                {t('IP States')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {statsData?.total_ip_states ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>
                {t('User States')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {statsData?.total_user_states ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>
                {t('Total Offenses')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {statsData?.total_offenses ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>
                {t('Recent Offenses')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {statsData?.recent_offenses ?? '-'}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Settings form */}
        <TitledCard title={t('Settings')}>
          <div className='space-y-6'>
            <div className='flex items-center justify-between'>
              <Label>{t('Enabled')}</Label>
              <Switch
                checked={config.enabled}
                onCheckedChange={(v) => updateField('enabled', v)}
              />
            </div>
            <div className='flex items-center justify-between'>
              <Label>{t('Dry Run')}</Label>
              <Switch
                checked={config.dry_run}
                onCheckedChange={(v) => updateField('dry_run', v)}
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('Ban Dimension')}</Label>
              <Select
                value={config.ban_dimension}
                onValueChange={(value) =>
                  updateField(
                    'ban_dimension',
                    value as ProbeGuardConfig['ban_dimension']
                  )
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='ip'>{t('IP')}</SelectItem>
                  <SelectItem value='user'>{t('User')}</SelectItem>
                  <SelectItem value='both'>{t('IP + User')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {bansUser && (
              <div className='flex items-center justify-between'>
                <Label>{t('Notify User')}</Label>
                <Switch
                  checked={config.notify_user_enabled}
                  onCheckedChange={(v) => updateField('notify_user_enabled', v)}
                />
              </div>
            )}
            <div className='flex items-center justify-between'>
              <Label>{t('Notify Admin')}</Label>
              <Switch
                checked={config.notify_admin_enabled}
                onCheckedChange={(v) => updateField('notify_admin_enabled', v)}
              />
            </div>
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <div className='space-y-2'>
                <Label>{t('Window (seconds)')}</Label>
                <Input
                  type='number'
                  min={1}
                  value={config.window_seconds}
                  onChange={(e) =>
                    updateField('window_seconds', Number(e.target.value))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Distinct Model Count')}</Label>
                <Input
                  type='number'
                  min={1}
                  value={config.distinct_model_count}
                  onChange={(e) =>
                    updateField('distinct_model_count', Number(e.target.value))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('First Ban (minutes)')}</Label>
                <Input
                  type='number'
                  min={1}
                  value={config.first_ip_ban_minutes}
                  onChange={(e) =>
                    updateField('first_ip_ban_minutes', Number(e.target.value))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Second Ban (minutes)')}</Label>
                <Input
                  type='number'
                  min={1}
                  value={config.second_ip_ban_minutes}
                  onChange={(e) =>
                    updateField('second_ip_ban_minutes', Number(e.target.value))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Permanent Offense Count')}</Label>
                <Input
                  type='number'
                  min={1}
                  value={config.permanent_offense_count}
                  onChange={(e) =>
                    updateField(
                      'permanent_offense_count',
                      Number(e.target.value)
                    )
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Offense Dedupe (seconds)')}</Label>
                <Input
                  type='number'
                  min={0}
                  value={config.offense_dedupe_seconds}
                  onChange={(e) =>
                    updateField(
                      'offense_dedupe_seconds',
                      Number(e.target.value)
                    )
                  }
                />
              </div>
            </div>
            <div className='space-y-2'>
              <Label>{t('Whitelist User IDs')}</Label>
              <Input
                value={config.whitelist_user_ids}
                onChange={(e) =>
                  updateField('whitelist_user_ids', e.target.value)
                }
                placeholder='1,2,3'
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('Whitelist Groups')}</Label>
              <RiskWhitelistGroupsField
                selected={config.whitelist_groups}
                onChange={(groups) => updateField('whitelist_groups', groups)}
              />
            </div>
            {bansUser && (
              <div className='space-y-2'>
                <Label>{t('User Ban Reason')}</Label>
                <Input
                  value={config.user_ban_reason}
                  onChange={(e) =>
                    updateField('user_ban_reason', e.target.value)
                  }
                />
              </div>
            )}
            <div className='space-y-2'>
              <Label>{t('Appeal Hint')}</Label>
              <Textarea
                value={config.appeal_hint}
                onChange={(e) => updateField('appeal_hint', e.target.value)}
              />
            </div>

            <div className='flex justify-end'>
              <Button
                onClick={() => config && saveMutation.mutate(config)}
                disabled={!isDirty || saveMutation.isPending}
              >
                {saveMutation.isPending ? t('Saving...') : t('Save Settings')}
              </Button>
            </div>
          </div>
        </TitledCard>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
