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
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getErrorBanConfig,
  getErrorBanStats,
  testErrorBanRule,
  updateErrorBanConfig,
  type ErrorBanConfig,
  type ErrorBanRule,
  type ErrorBanTier,
} from '../api'
import { RiskWhitelistGroupsField } from './risk-whitelist-groups-field'

function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2)
}

function cloneRule(rule: ErrorBanRule): ErrorBanRule {
  return {
    ...rule,
    keywords: [...rule.keywords],
    error_codes: [...rule.error_codes],
    tiers: rule.tiers.map((tier) => ({ ...tier })),
  }
}

export function ErrorBanPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [config, setConfig] = useState<ErrorBanConfig | null>(null)
  const [isDirty, setIsDirty] = useState(false)
  const [ruleDialogOpen, setRuleDialogOpen] = useState(false)
  const [editingRuleIndex, setEditingRuleIndex] = useState<number | null>(null)
  const [ruleDraft, setRuleDraft] = useState<ErrorBanRule | null>(null)
  const [testSample, setTestSample] = useState('')
  const [testErrorCode, setTestErrorCode] = useState('')

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
    mutationFn: (data: {
      pattern: string
      keywords: string[]
      error_codes: string[]
      sample_text: string
      error_code: string
    }) => testErrorBanRule(data),
    onSuccess: (res) => {
      if (res.success && res.data) {
        const result = res.data
        if (result.valid && result.matched) {
          toast.success(t('Rule matched'))
        } else if (result.valid && !result.matched) {
          toast(t('Rule is valid but did not match'))
        } else {
          toast.error(result.error || t('Rule is invalid'))
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

  const openNewRule = () => {
    setEditingRuleIndex(null)
    setRuleDraft({
      id: generateId(),
      name: '',
      pattern: '',
      keywords: [],
      error_codes: [],
      enabled: true,
      dimension: '',
      threshold: 5,
      reason_template: '',
      tiers: [
        {
          offense_count: 1,
          action: 'temp_ip_ban',
          duration_minutes: 30,
          reason_suffix: '',
        },
      ],
    })
    setTestSample('')
    setTestErrorCode('')
    testMutation.reset()
    setRuleDialogOpen(true)
  }

  const openRuleEditor = (index: number) => {
    if (!config) return
    setEditingRuleIndex(index)
    setRuleDraft(cloneRule(config.rules[index]))
    setTestSample('')
    setTestErrorCode('')
    testMutation.reset()
    setRuleDialogOpen(true)
  }

  const saveRuleDraft = () => {
    if (!ruleDraft) return
    setConfig((prev) => {
      if (!prev) return prev
      const rules = [...prev.rules]
      if (editingRuleIndex === null) rules.push(ruleDraft)
      else rules[editingRuleIndex] = ruleDraft
      return { ...prev, rules }
    })
    setIsDirty(true)
    setRuleDialogOpen(false)
  }

  const deleteRule = (index: number) => {
    setConfig((prev) => {
      if (!prev) return prev
      const rules = prev.rules.filter((_, i) => i !== index)
      const next = { ...prev, rules }
      setIsDirty(true)
      return next
    })
  }

  const toggleRule = (index: number, enabled: boolean) => {
    setConfig((prev) => {
      if (!prev) return prev
      const rules = [...prev.rules]
      rules[index] = { ...rules[index], enabled }
      return { ...prev, rules }
    })
    setIsDirty(true)
  }

  const canSaveRule = Boolean(
    ruleDraft?.name.trim() &&
    ruleDraft.id.trim() &&
    ruleDraft.threshold >= 1 &&
    (!ruleDraft.enabled ||
      ruleDraft.pattern.trim() ||
      ruleDraft.keywords.length ||
      ruleDraft.error_codes.length) &&
    ruleDraft.tiers.length &&
    ruleDraft.tiers.every(
      (tier) =>
        tier.offense_count >= 1 &&
        tier.duration_minutes >= 0 &&
        (tier.action !== 'temp_ip_ban' || tier.duration_minutes >= 1)
    )
  )
  let testResultMessage = ''
  if (testMutation.data?.data) {
    const result = testMutation.data.data
    if (!result.valid) testResultMessage = result.error || t('Rule is invalid')
    else if (result.matched) testResultMessage = t('Rule matched')
    else testResultMessage = t('Rule did not match')
  }

  if (configLoading || !config) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Error Ban')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='text-muted-foreground flex items-center justify-center py-12'>
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
            <div className='space-y-2'>
              <Label>{t('Whitelist Groups')}</Label>
              <RiskWhitelistGroupsField
                selected={config.whitelist_groups}
                onChange={(groups) => updateField('whitelist_groups', groups)}
              />
            </div>

            <div className='space-y-3'>
              <div className='flex items-center justify-between'>
                <Label className='text-base font-semibold'>{t('Rules')}</Label>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant='outline'
                        size='icon-sm'
                        onClick={openNewRule}
                        disabled={config.rules.length >= 20}
                      />
                    }
                  >
                    <Plus className='size-4' />
                  </TooltipTrigger>
                  <TooltipContent>{t('Add Rule')}</TooltipContent>
                </Tooltip>
              </div>
              {config.rules.length === 0 ? (
                <div className='text-muted-foreground py-4 text-center text-sm'>
                  {t('No rules configured')}
                </div>
              ) : (
                <div className='divide-y rounded-lg border'>
                  {config.rules.map((rule, index) => (
                    <div
                      key={rule.id}
                      className='flex min-h-14 items-center gap-3 px-3 py-2'
                    >
                      <span className='min-w-0 flex-1 truncate text-sm font-medium'>
                        {rule.name || rule.id}
                      </span>
                      <span className='text-muted-foreground shrink-0 text-sm'>
                        {t('Threshold')}: {rule.threshold}
                      </span>
                      <Switch
                        checked={rule.enabled}
                        aria-label={t('Toggle rule {{name}}', {
                          name: rule.name || rule.id,
                        })}
                        onCheckedChange={(enabled) =>
                          toggleRule(index, enabled)
                        }
                      />
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => openRuleEditor(index)}
                            />
                          }
                        >
                          <Pencil className='size-4' />
                        </TooltipTrigger>
                        <TooltipContent>{t('Edit')}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => deleteRule(index)}
                            />
                          }
                        >
                          <Trash2 className='text-destructive size-4' />
                        </TooltipTrigger>
                        <TooltipContent>{t('Delete')}</TooltipContent>
                      </Tooltip>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Save button */}
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

        <Dialog open={ruleDialogOpen} onOpenChange={setRuleDialogOpen}>
          <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-3xl'>
            <DialogHeader>
              <DialogTitle>
                {editingRuleIndex === null ? t('Add Rule') : t('Edit Rule')}
              </DialogTitle>
              <DialogDescription>
                {t('All configured match conditions must be satisfied')}
              </DialogDescription>
            </DialogHeader>
            {ruleDraft && (
              <div className='space-y-5'>
                <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                  <div className='space-y-2'>
                    <Label>{t('Name')}</Label>
                    <Input
                      value={ruleDraft.name}
                      onChange={(event) =>
                        setRuleDraft({
                          ...ruleDraft,
                          name: event.target.value,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label>{t('Rule ID')}</Label>
                    <Input
                      value={ruleDraft.id}
                      onChange={(event) =>
                        setRuleDraft({ ...ruleDraft, id: event.target.value })
                      }
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label>{t('Threshold')}</Label>
                    <Input
                      type='number'
                      min={1}
                      max={100000}
                      value={ruleDraft.threshold}
                      onChange={(event) =>
                        setRuleDraft({
                          ...ruleDraft,
                          threshold: Number(event.target.value),
                        })
                      }
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label>{t('Dimension')}</Label>
                    <Select
                      value={ruleDraft.dimension || '_default'}
                      onValueChange={(value) =>
                        setRuleDraft({
                          ...ruleDraft,
                          dimension:
                            value === '_default'
                              ? ''
                              : (value as 'ip' | 'user'),
                        })
                      }
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='_default'>
                          {t('Inherit default')}
                        </SelectItem>
                        <SelectItem value='ip'>{t('IP')}</SelectItem>
                        <SelectItem value='user'>{t('User')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <div className='space-y-2'>
                  <Label>{t('Regular Expression')}</Label>
                  <Input
                    value={ruleDraft.pattern}
                    onChange={(event) =>
                      setRuleDraft({
                        ...ruleDraft,
                        pattern: event.target.value,
                      })
                    }
                    placeholder={t('Optional regular expression')}
                  />
                </div>
                <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                  <div className='space-y-2'>
                    <Label>{t('Error Keywords')}</Label>
                    <Textarea
                      value={ruleDraft.keywords.join('\n')}
                      onChange={(event) =>
                        setRuleDraft({
                          ...ruleDraft,
                          keywords: event.target.value
                            .split('\n')
                            .map((value) => value.trim())
                            .filter(Boolean),
                        })
                      }
                      rows={3}
                      placeholder={t('One keyword per line; all must match')}
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label>{t('Error Codes')}</Label>
                    <Textarea
                      value={ruleDraft.error_codes.join('\n')}
                      onChange={(event) =>
                        setRuleDraft({
                          ...ruleDraft,
                          error_codes: event.target.value
                            .split('\n')
                            .map((value) => value.trim())
                            .filter(Boolean),
                        })
                      }
                      rows={3}
                      placeholder='*'
                    />
                  </div>
                </div>
                <div className='space-y-2'>
                  <Label>{t('Reason Template')}</Label>
                  <Input
                    value={ruleDraft.reason_template}
                    onChange={(event) =>
                      setRuleDraft({
                        ...ruleDraft,
                        reason_template: event.target.value,
                      })
                    }
                  />
                </div>

                <div className='space-y-3 border-t pt-4'>
                  <div className='flex items-center justify-between'>
                    <Label className='text-base font-semibold'>
                      {t('Tiers')}
                    </Label>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='outline'
                            size='icon-sm'
                            onClick={() =>
                              setRuleDraft({
                                ...ruleDraft,
                                tiers: [
                                  ...ruleDraft.tiers,
                                  {
                                    offense_count:
                                      (ruleDraft.tiers.at(-1)?.offense_count ??
                                        0) + 1,
                                    action: 'temp_ip_ban',
                                    duration_minutes: 30,
                                    reason_suffix: '',
                                  },
                                ],
                              })
                            }
                          />
                        }
                      >
                        <Plus className='size-4' />
                      </TooltipTrigger>
                      <TooltipContent>{t('Add Tier')}</TooltipContent>
                    </Tooltip>
                  </div>
                  {ruleDraft.tiers.map((tier, index) => (
                    <div
                      // eslint-disable-next-line react/no-array-index-key
                      key={index}
                      className='space-y-3 border-t pt-3 first:border-t-0 first:pt-0'
                    >
                      <div className='flex items-center justify-between'>
                        <span className='text-sm font-medium'>
                          {t('Tier')} {index + 1}
                        </span>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() =>
                            setRuleDraft({
                              ...ruleDraft,
                              tiers: ruleDraft.tiers.filter(
                                (_, tierIndex) => tierIndex !== index
                              ),
                            })
                          }
                        >
                          <Trash2 className='text-destructive size-4' />
                          <span className='sr-only'>{t('Delete')}</span>
                        </Button>
                      </div>
                      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                        <div className='space-y-2'>
                          <Label>{t('Offense Count')}</Label>
                          <Input
                            type='number'
                            min={1}
                            max={100000}
                            value={tier.offense_count}
                            onChange={(event) => {
                              const tiers = [...ruleDraft.tiers]
                              tiers[index] = {
                                ...tier,
                                offense_count: Number(event.target.value),
                              }
                              setRuleDraft({ ...ruleDraft, tiers })
                            }}
                          />
                        </div>
                        <div className='space-y-2'>
                          <Label>{t('Action')}</Label>
                          <Select
                            value={tier.action}
                            onValueChange={(value) => {
                              const tiers = [...ruleDraft.tiers]
                              tiers[index] = {
                                ...tier,
                                action: value as ErrorBanTier['action'],
                                duration_minutes:
                                  value === 'temp_ip_ban' &&
                                  tier.duration_minutes <= 0
                                    ? 1
                                    : tier.duration_minutes,
                              }
                              setRuleDraft({ ...ruleDraft, tiers })
                            }}
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
                        {tier.action !== 'perm_ip_ban' && (
                          <div className='space-y-2'>
                            <Label>
                              {tier.action === 'temp_ip_ban'
                                ? t('IP ban duration (minutes)')
                                : t(
                                    'Account ban duration (minutes, 0 for permanent)'
                                  )}
                            </Label>
                            <Input
                              type='number'
                              min={tier.action === 'temp_ip_ban' ? 1 : 0}
                              max={525600}
                              value={tier.duration_minutes}
                              onChange={(event) => {
                                const tiers = [...ruleDraft.tiers]
                                tiers[index] = {
                                  ...tier,
                                  duration_minutes: Number(event.target.value),
                                }
                                setRuleDraft({ ...ruleDraft, tiers })
                              }}
                            />
                          </div>
                        )}
                        <div className='space-y-2'>
                          <Label>{t('Reason Suffix')}</Label>
                          <Input
                            value={tier.reason_suffix}
                            onChange={(event) => {
                              const tiers = [...ruleDraft.tiers]
                              tiers[index] = {
                                ...tier,
                                reason_suffix: event.target.value,
                              }
                              setRuleDraft({ ...ruleDraft, tiers })
                            }}
                          />
                        </div>
                      </div>
                    </div>
                  ))}
                </div>

                <div className='space-y-3 border-t pt-4'>
                  <Label className='text-base font-semibold'>
                    {t('Test Rule')}
                  </Label>
                  <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                    <div className='space-y-2'>
                      <Label>{t('Sample Text')}</Label>
                      <Textarea
                        value={testSample}
                        onChange={(event) => setTestSample(event.target.value)}
                        rows={3}
                      />
                    </div>
                    <div className='space-y-2'>
                      <Label>{t('Sample Error Code')}</Label>
                      <Input
                        value={testErrorCode}
                        onChange={(event) =>
                          setTestErrorCode(event.target.value)
                        }
                      />
                    </div>
                  </div>
                  <div className='flex items-center justify-between gap-3'>
                    <span className='text-muted-foreground text-sm'>
                      {testResultMessage}
                    </span>
                    <Button
                      variant='secondary'
                      disabled={testMutation.isPending}
                      onClick={() =>
                        testMutation.mutate({
                          pattern: ruleDraft.pattern,
                          keywords: ruleDraft.keywords,
                          error_codes: ruleDraft.error_codes,
                          sample_text: testSample,
                          error_code: testErrorCode,
                        })
                      }
                    >
                      {testMutation.isPending ? t('Testing...') : t('Test')}
                    </Button>
                  </div>
                </div>
              </div>
            )}
            <DialogFooter>
              <Button
                variant='outline'
                onClick={() => setRuleDialogOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button disabled={!canSaveRule} onClick={saveRuleDraft}>
                {t('Confirm')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
