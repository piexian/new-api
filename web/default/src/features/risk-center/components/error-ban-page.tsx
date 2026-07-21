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
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
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
  getErrorBanConfig,
  getErrorBanStats,
  testErrorBanRule,
  updateErrorBanConfig,
  type ErrorBanConfig,
  type ErrorBanRule,
  type ErrorBanTier,
} from '../api'

function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2)
}

export function ErrorBanPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [config, setConfig] = useState<ErrorBanConfig | null>(null)
  const [isDirty, setIsDirty] = useState(false)

  const { data: configData, isLoading: configLoading } = useQuery({
    queryKey: ['risk', 'error-ban', 'config'],
    queryFn: async () => {
      const res = await getErrorBanConfig()
      if (res.success && res.data) {
        setConfig(res.data)
        return res.data
      }
      throw new Error(res.message || t('Failed to load config'))
    },
  })

  const { data: statsData } = useQuery({
    queryKey: ['risk', 'error-ban', 'stats'],
    queryFn: async () => {
      const res = await getErrorBanStats()
      if (res.success) return res.data
      throw new Error(res.message || 'Failed to load stats')
    },
    refetchInterval: 30000,
  })

  const saveMutation = useMutation({
    mutationFn: (data: ErrorBanConfig) => updateErrorBanConfig(data),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Settings saved'))
        setIsDirty(false)
        queryClient.invalidateQueries({ queryKey: ['risk', 'error-ban'] })
      } else {
        toast.error(res.message || t('Failed to save settings'))
      }
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  const testMutation = useMutation({
    mutationFn: (data: { pattern: string; sample_text: string }) =>
      testErrorBanRule(data),
    onSuccess: (res) => {
      if (res.success && res.data) {
        const result = res.data
        if (result.valid && result.matched) {
          toast.success(t('Pattern matched'))
        } else if (result.valid && !result.matched) {
          toast(t('Pattern valid but did not match'))
        } else {
          toast.error(result.error || t('Pattern is invalid'))
        }
      } else {
        toast.error(res.message || t('Test failed'))
      }
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  const updateField = <K extends keyof ErrorBanConfig>(
    key: K,
    value: ErrorBanConfig[K]
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

  const updateRule = (index: number, rule: ErrorBanRule) => {
    setConfig((prev) => {
      if (!prev) return prev
      const rules = [...prev.rules]
      rules[index] = rule
      const next = { ...prev, rules }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  const addRule = () => {
    setConfig((prev) => {
      if (!prev) return prev
      if (prev.rules.length >= 20) return prev
      const newRule: ErrorBanRule = {
        id: generateId(),
        name: '',
        pattern: '',
        enabled: true,
        dimension: 'ip',
        threshold: 5,
        reason_template: '',
      }
      const rules = [...prev.rules, newRule]
      const next = { ...prev, rules }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  const deleteRule = (index: number) => {
    setConfig((prev) => {
      if (!prev) return prev
      const rules = prev.rules.filter((_, i) => i !== index)
      const next = { ...prev, rules }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  const updateTier = (index: number, tier: ErrorBanTier) => {
    setConfig((prev) => {
      if (!prev) return prev
      const tiers = [...prev.tiers]
      tiers[index] = tier
      const next = { ...prev, tiers }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  const addTier = () => {
    setConfig((prev) => {
      if (!prev) return prev
      const newTier: ErrorBanTier = {
        offense_count: 10,
        action: 'temp_ip_ban',
        duration_minutes: 60,
        reason_suffix: '',
      }
      const tiers = [...prev.tiers, newTier]
      const next = { ...prev, tiers }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  const deleteTier = (index: number) => {
    setConfig((prev) => {
      if (!prev) return prev
      const tiers = prev.tiers.filter((_, i) => i !== index)
      const next = { ...prev, tiers }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const eq = (a: any, b: any) => a === b
      setIsDirty(!eq(next, configData))
      return next
    })
  }

  const [testPattern, setTestPattern] = useState('')
  const [testSample, setTestSample] = useState('')

  if (configLoading || !config) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Error Ban')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='flex items-center justify-center py-12 text-muted-foreground'>
            {t('Loading...')}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Error Ban')}</span>
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
              <CardTitle className='text-sm font-medium text-muted-foreground'>
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
              <CardTitle className='text-sm font-medium text-muted-foreground'>
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
              <CardTitle className='text-sm font-medium text-muted-foreground'>
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
              <CardTitle className='text-sm font-medium text-muted-foreground'>
                {t('Active Rules')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {statsData?.active_rules ?? '-'}
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
            <div className='flex items-center justify-between'>
              <Label>{t('Notify User')}</Label>
              <Switch
                checked={config.notify_user_enabled}
                onCheckedChange={(v) => updateField('notify_user_enabled', v)}
              />
            </div>
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
                <Label>{t('Default Dimension')}</Label>
                <Select
                  value={config.default_dimension}
                  onValueChange={(v) =>
                    updateField('default_dimension', v as 'ip' | 'user')
                  }
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='ip'>{t('IP')}</SelectItem>
                    <SelectItem value='user'>{t('User')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2'>
                <Label>{t('Exclude Status Codes')}</Label>
                <Input
                  value={config.exclude_status_codes.join(',')}
                  onChange={(e) => {
                    const codes = e.target.value
                      .split(',')
                      .map((s) => s.trim())
                      .filter(Boolean)
                      .map(Number)
                    updateField('exclude_status_codes', codes)
                  }}
                  placeholder='400,403,404'
                />
              </div>
            </div>
            <div className='space-y-2'>
              <Label>{t('Default Reason Template')}</Label>
              <Textarea
                value={config.default_reason_template}
                onChange={(e) =>
                  updateField('default_reason_template', e.target.value)
                }
                rows={2}
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('Appeal Hint')}</Label>
              <Textarea
                value={config.appeal_hint}
                onChange={(e) => updateField('appeal_hint', e.target.value)}
                rows={2}
              />
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

            {/* Rules */}
            <div className='space-y-4'>
              <div className='flex items-center justify-between'>
                <Label className='text-base font-semibold'>{t('Rules')}</Label>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={addRule}
                  disabled={config.rules.length >= 20}
                >
                  {t('Add Rule')}
                </Button>
              </div>
              {config.rules.length === 0 ? (
                <div className='py-4 text-center text-sm text-muted-foreground'>
                  {t('No rules configured')}
                </div>
              ) : (
                config.rules.map((rule, index) => (
                  <div
                    key={rule.id}
                    className='rounded-lg border p-4 space-y-4'
                  >
                    <div className='flex items-center justify-between'>
                      <span className='text-sm font-medium'>
                        {t('Rule')} {index + 1}
                      </span>
                      <Button
                        variant='destructive'
                        size='sm'
                        onClick={() => deleteRule(index)}
                      >
                        {t('Delete')}
                      </Button>
                    </div>
                    <div className='flex items-center justify-between'>
                      <Label>{t('Enabled')}</Label>
                      <Switch
                        checked={rule.enabled}
                        onCheckedChange={(v) =>
                          updateRule(index, { ...rule, enabled: v })
                        }
                      />
                    </div>
                    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                      <div className='space-y-2'>
                        <Label>{t('Name')}</Label>
                        <Input
                          value={rule.name}
                          onChange={(e) =>
                            updateRule(index, { ...rule, name: e.target.value })
                          }
                        />
                      </div>
                      <div className='space-y-2'>
                        <Label>{t('Dimension')}</Label>
                        <Select
                          value={rule.dimension}
                          onValueChange={(v) =>
                            updateRule(index, {
                              ...rule,
                              dimension: v as '' | 'ip' | 'user',
                            })
                          }
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value=''>{t('None')}</SelectItem>
                            <SelectItem value='ip'>{t('IP')}</SelectItem>
                            <SelectItem value='user'>{t('User')}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className='space-y-2'>
                        <Label>{t('Pattern')}</Label>
                        <Input
                          value={rule.pattern}
                          onChange={(e) =>
                            updateRule(index, {
                              ...rule,
                              pattern: e.target.value,
                            })
                          }
                          placeholder={t('Error pattern regex')}
                        />
                      </div>
                      <div className='space-y-2'>
                        <Label>{t('Threshold')}</Label>
                        <Input
                          type='number'
                          min={1}
                          value={rule.threshold}
                          onChange={(e) =>
                            updateRule(index, {
                              ...rule,
                              threshold: Number(e.target.value),
                            })
                          }
                        />
                      </div>
                    </div>
                    <div className='space-y-2'>
                      <Label>{t('Reason Template')}</Label>
                      <Input
                        value={rule.reason_template}
                        onChange={(e) =>
                          updateRule(index, {
                            ...rule,
                            reason_template: e.target.value,
                          })
                        }
                      />
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Tiers */}
            <div className='space-y-4'>
              <div className='flex items-center justify-between'>
                <Label className='text-base font-semibold'>{t('Tiers')}</Label>
                <Button variant='outline' size='sm' onClick={addTier}>
                  {t('Add Tier')}
                </Button>
              </div>
              {config.tiers.length === 0 ? (
                <div className='py-4 text-center text-sm text-muted-foreground'>
                  {t('No tiers configured')}
                </div>
              ) : (
                config.tiers.map((tier, index) => (
                  // eslint-disable-next-line react/no-array-index-key
                  <div key={index} className='rounded-lg border p-4 space-y-4'>
                    <div className='flex items-center justify-between'>
                      <span className='text-sm font-medium'>
                        {t('Tier')} {index + 1}
                      </span>
                      <Button
                        variant='destructive'
                        size='sm'
                        onClick={() => deleteTier(index)}
                      >
                        {t('Delete')}
                      </Button>
                    </div>
                    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                      <div className='space-y-2'>
                        <Label>{t('Offense Count')}</Label>
                        <Input
                          type='number'
                          min={1}
                          value={tier.offense_count}
                          onChange={(e) =>
                            updateTier(index, {
                              ...tier,
                              offense_count: Number(e.target.value),
                            })
                          }
                        />
                      </div>
                      <div className='space-y-2'>
                        <Label>{t('Action')}</Label>
                        <Select
                          value={tier.action}
                          onValueChange={(v) =>
                            updateTier(index, {
                              ...tier,
                              action: v as ErrorBanTier['action'],
                            })
                          }
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value='temp_ip_ban'>
                              {t('Temp IP Ban')}
                            </SelectItem>
                            <SelectItem value='perm_ip_ban'>
                              {t('Perm IP Ban')}
                            </SelectItem>
                            <SelectItem value='disable_user'>
                              {t('Disable User')}
                            </SelectItem>
                            <SelectItem value='both'>{t('Both')}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className='space-y-2'>
                        <Label>{t('Duration (minutes)')}</Label>
                        <Input
                          type='number'
                          min={0}
                          value={tier.duration_minutes}
                          onChange={(e) =>
                            updateTier(index, {
                              ...tier,
                              duration_minutes: Number(e.target.value),
                            })
                          }
                        />
                      </div>
                      <div className='space-y-2'>
                        <Label>{t('Reason Suffix')}</Label>
                        <Input
                          value={tier.reason_suffix}
                          onChange={(e) =>
                            updateTier(index, {
                              ...tier,
                              reason_suffix: e.target.value,
                            })
                          }
                        />
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Test Rule */}
            <Collapsible className='space-y-4'>
              <CollapsibleTrigger className='flex w-full items-center justify-between rounded-lg border p-3 text-sm font-medium hover:bg-muted/50'>
                {t('Test Rule')}
                <span className='text-muted-foreground'>&#9660;</span>
              </CollapsibleTrigger>
              <CollapsibleContent className='space-y-4 px-1'>
                <div className='space-y-2'>
                  <Label>{t('Pattern')}</Label>
                  <Input
                    value={testPattern}
                    onChange={(e) => setTestPattern(e.target.value)}
                    placeholder={t('Error pattern regex')}
                  />
                </div>
                <div className='space-y-2'>
                  <Label>{t('Sample Text')}</Label>
                  <Textarea
                    value={testSample}
                    onChange={(e) => setTestSample(e.target.value)}
                    rows={3}
                    placeholder={t('Sample error text to test against the pattern')}
                  />
                </div>
                <div className='flex justify-end'>
                  <Button
                    onClick={() =>
                      testMutation.mutate({
                        pattern: testPattern,
                        sample_text: testSample,
                      })
                    }
                    disabled={!testPattern || !testSample || testMutation.isPending}
                    variant='secondary'
                  >
                    {testMutation.isPending
                      ? t('Testing...')
                      : t('Test')}
                  </Button>
                </div>
                {testMutation.data?.success && testMutation.data.data && (() => {
                  const result = testMutation.data.data
                  // eslint-disable-next-line no-nested-ternary
                  const colorClass = result.valid
                    ? result.matched
                      ? 'bg-green-50 text-green-800 dark:bg-green-950 dark:text-green-200'
                      : 'bg-yellow-50 text-yellow-800 dark:bg-yellow-950 dark:text-yellow-200'
                    : 'bg-red-50 text-red-800 dark:bg-red-950 dark:text-red-200'
                  // eslint-disable-next-line no-nested-ternary
                  const message = result.valid
                    ? result.matched
                      ? t('Pattern matched the sample text')
                      : t('Pattern is valid but did not match the sample text')
                    : result.error
                      ? `${t('Pattern is invalid')}: ${result.error}`
                      : t('Pattern is invalid')
                  return (
                    <div className={`rounded-md p-3 text-sm ${colorClass}`}>
                      {message}
                    </div>
                  )
                })()}
              </CollapsibleContent>
            </Collapsible>

            {/* Save button */}
            <div className='flex justify-end'>
              <Button
                onClick={() => config && saveMutation.mutate(config)}
                disabled={!isDirty || saveMutation.isPending}
              >
                {saveMutation.isPending
                  ? t('Saving...')
                  : t('Save Settings')}
              </Button>
            </div>
          </div>
        </TitledCard>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
