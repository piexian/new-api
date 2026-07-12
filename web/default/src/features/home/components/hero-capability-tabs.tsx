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
*/

import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

type AccentTone = 'emerald' | 'amber' | 'blue' | 'violet' | 'sky' | 'fuchsia'

type ProtocolTab = {
  kind: 'protocol'
  id: string
  label: string
  method: 'POST'
  endpoint: string
  headers: string[]
  request: string[]
  response: string[]
  tokens: number
  latency: number
  accent: AccentTone
}

type SetupTab = {
  kind: 'setup'
  id: string
  label: string
  accent: AccentTone
  chips: string[]
  snippets: { id: string; label: string; body: string }[]
  hint: string
}

type CapabilityTab = ProtocolTab | SetupTab

const ACCENT: Record<
  AccentTone,
  { activeText: string; activeBorder: string; badge: string }
> = {
  emerald: {
    activeText: 'text-emerald-600 dark:text-emerald-400',
    activeBorder: 'border-emerald-500 dark:border-emerald-400',
    badge:
      'bg-emerald-500/10 text-emerald-600 dark:bg-emerald-400/10 dark:text-emerald-400',
  },
  amber: {
    activeText: 'text-amber-600 dark:text-amber-400',
    activeBorder: 'border-amber-500 dark:border-amber-400',
    badge:
      'bg-amber-500/10 text-amber-600 dark:bg-amber-400/10 dark:text-amber-400',
  },
  blue: {
    activeText: 'text-blue-600 dark:text-blue-400',
    activeBorder: 'border-blue-500 dark:border-blue-400',
    badge: 'bg-blue-500/10 text-blue-600 dark:bg-blue-400/10 dark:text-blue-400',
  },
  violet: {
    activeText: 'text-violet-600 dark:text-violet-400',
    activeBorder: 'border-violet-500 dark:border-violet-400',
    badge:
      'bg-violet-500/10 text-violet-600 dark:bg-violet-400/10 dark:text-violet-400',
  },
  sky: {
    activeText: 'text-sky-600 dark:text-sky-400',
    activeBorder: 'border-sky-500 dark:border-sky-400',
    badge: 'bg-sky-500/10 text-sky-600 dark:bg-sky-400/10 dark:text-sky-400',
  },
  fuchsia: {
    activeText: 'text-fuchsia-600 dark:text-fuchsia-400',
    activeBorder: 'border-fuchsia-500 dark:border-fuchsia-400',
    badge:
      'bg-fuchsia-500/10 text-fuchsia-600 dark:bg-fuchsia-400/10 dark:text-fuchsia-400',
  },
}

const PROTOCOL_TABS: ProtocolTab[] = [
  {
    kind: 'protocol',
    id: 'chat',
    label: 'Chat',
    method: 'POST',
    endpoint: '/v1/chat/completions',
    headers: ['"Authorization: Bearer sk-••••"'],
    request: [
      '"model": "your-model",',
      '"messages": [',
      '  { "role": "user", "content": "..." }',
      ']',
    ],
    response: [
      '{',
      '  "choices": [{ "message": { "content": "..." } }],',
      '  "usage": { "total_tokens": 27 }',
      '}',
    ],
    tokens: 27,
    latency: 142,
    accent: 'emerald',
  },
  {
    kind: 'protocol',
    id: 'responses',
    label: 'Responses',
    method: 'POST',
    endpoint: '/v1/responses',
    headers: ['"Authorization: Bearer sk-••••"'],
    request: ['"model": "your-model",', '"input": "..."'],
    response: [
      '{',
      '  "output": [{ "type": "output_text", "text": "..." }],',
      '  "usage": { "total_tokens": 31 }',
      '}',
    ],
    tokens: 31,
    latency: 168,
    accent: 'amber',
  },
  {
    kind: 'protocol',
    id: 'claude',
    label: 'Claude',
    method: 'POST',
    endpoint: '/v1/messages',
    headers: ['"x-api-key: sk-••••"', '"anthropic-version: 2023-06-01"'],
    request: [
      '"model": "your-model",',
      '"max_tokens": 1024,',
      '"messages": [',
      '  { "role": "user", "content": "..." }',
      ']',
    ],
    response: [
      '{',
      '  "content": [{ "type": "text", "text": "..." }],',
      '  "usage": { "input_tokens": 12, "output_tokens": 17 }',
      '}',
    ],
    tokens: 29,
    latency: 156,
    accent: 'blue',
  },
  {
    kind: 'protocol',
    id: 'gemini',
    label: 'Gemini',
    method: 'POST',
    endpoint: '/v1beta/models/{model}:generateContent',
    headers: ['"x-goog-api-key: sk-••••"'],
    request: [
      '"contents": [',
      '  { "role": "user",',
      '    "parts": [{ "text": "..." }] }',
      ']',
    ],
    response: [
      '{',
      '  "candidates": [{ "content": { "parts": [{ "text": "..." }] } }],',
      '  "usageMetadata": { "totalTokenCount": 25 }',
      '}',
    ],
    tokens: 25,
    latency: 93,
    accent: 'violet',
  },
]

const CYCLE_INTERVAL = 4500
const PROTOCOL_COUNT = PROTOCOL_TABS.length

function normalizeBaseUrl(raw: string) {
  return raw.replace(/\/+$/, '')
}

function buildTabs(baseUrl: string): CapabilityTab[] {
  const origin = normalizeBaseUrl(baseUrl || 'https://your-gateway.example')
  const openaiBase = `${origin}/v1`
  return [
    ...PROTOCOL_TABS,
    {
      kind: 'setup',
      id: 'codex',
      label: 'Codex',
      accent: 'sky',
      chips: ['config.toml', 'auth.json', 'responses'],
      snippets: [
        {
          id: 'codex-config',
          label: 'config',
          body: `[model_providers.OpenAI]
name = "OpenAI"
base_url = "${openaiBase}"
wire_api = "responses"
requires_openai_auth = true`,
        },
        {
          id: 'codex-auth',
          label: 'auth',
          body: `{
  "auth_mode": "apikey",
  "OPENAI_API_KEY": "sk-your-new-api-token"
}`,
        },
      ],
      hint: 'Write ~/.codex/config.toml and ~/.codex/auth.json. wire_api must be responses.',
    },
    {
      kind: 'setup',
      id: 'claude-code',
      label: 'Claude Code',
      accent: 'fuchsia',
      chips: ['settings.json', 'env', 'gateway'],
      snippets: [
        {
          id: 'claude-settings',
          label: 'settings',
          body: `{
  "env": {
    "ANTHROPIC_BASE_URL": "${origin}",
    "ANTHROPIC_AUTH_TOKEN": "sk-your-new-api-token",
    "ANTHROPIC_MODEL": "your-gateway-model",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "your-gateway-model",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "your-gateway-model",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "your-gateway-model",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
    "CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING": "1",
    "API_TIMEOUT_MS": "600000"
  }
}`,
        },
      ],
      hint: 'Write ~/.claude/settings.json (or project .claude/settings.json). Use gateway model IDs.',
    },
  ]
}

export function HeroCapabilityTabs() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const baseUrl = useMemo(() => {
    let fromStatus = ''
    if (status && typeof status === 'object' && 'server_address' in status) {
      const raw = status.server_address
      if (typeof raw === 'string') fromStatus = raw.trim()
    }
    if (fromStatus) return fromStatus
    if (typeof window !== 'undefined') return window.location.origin
    return 'https://your-gateway.example'
  }, [status])

  const tabs = useMemo(() => buildTabs(baseUrl), [baseUrl])
  const [activeIndex, setActiveIndex] = useState(0)
  const intervalRef = useRef<number | undefined>(undefined)

  const clearCycle = () => {
    if (intervalRef.current !== undefined) {
      window.clearInterval(intervalRef.current)
      intervalRef.current = undefined
    }
  }

  const startCycle = (fromIndex: number) => {
    clearCycle()
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    if (tabs[fromIndex]?.kind !== 'protocol') return
    intervalRef.current = window.setInterval(() => {
      setActiveIndex((prev) => {
        if (tabs[prev]?.kind !== 'protocol') return prev
        return (prev + 1) % PROTOCOL_COUNT
      })
    }, CYCLE_INTERVAL)
  }

  useEffect(() => {
    startCycle(0)
    return clearCycle
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tabs])

  const handleSelect = (index: number) => {
    setActiveIndex(index)
    startCycle(index)
  }

  const active = tabs[activeIndex]
  const accent = ACCENT[active.accent]

  const handleCopy = async (text: string, label: string) => {
    const ok = await copyToClipboard(text)
    if (ok) toast.success(t('Copied!'))
    else toast.error(t('Failed to copy {{label}}', { label }))
  }

  return (
    <div className='mx-auto mt-12 w-full max-w-2xl md:mt-16'>
      <div
        className={cn(
          'overflow-hidden rounded-2xl border backdrop-blur-sm',
          'border-border/60 bg-white/95 shadow-[0_20px_50px_-25px_rgba(15,23,42,0.18)]',
          'dark:border-white/[0.06] dark:bg-[#0b0f17]/95 dark:shadow-[0_20px_60px_-25px_rgba(0,0,0,0.7)]'
        )}
      >
        <div
          className={cn(
            'flex items-center gap-1 overflow-x-auto border-b px-2 sm:gap-1.5 sm:px-3',
            'border-border/50 dark:border-white/[0.05]'
          )}
        >
          {tabs.map((item, index) => {
            const tone = ACCENT[item.accent]
            const isActive = index === activeIndex
            const showSep = index === PROTOCOL_COUNT
            return (
              <div key={item.id} className='flex items-center'>
                {showSep ? (
                  <span
                    aria-hidden
                    className='bg-border/80 mx-1 h-4 w-px shrink-0 dark:bg-white/10'
                  />
                ) : null}
                <button
                  type='button'
                  onClick={() => handleSelect(index)}
                  className={cn(
                    'relative -mb-px flex items-center gap-1.5 border-b-2 px-2.5 py-2.5 text-[11px] font-medium tracking-wide whitespace-nowrap transition-colors sm:px-3 sm:text-xs',
                    isActive
                      ? `${tone.activeBorder} ${tone.activeText}`
                      : 'text-foreground/40 hover:text-foreground/70 border-transparent'
                  )}
                >
                  {item.label}
                </button>
              </div>
            )
          })}
          <div className='ml-auto flex shrink-0 items-center gap-2 pr-2 sm:pr-3'>
            <span className='inline-block size-1.5 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.45)]' />
            <span className='text-foreground/40 font-mono text-[10px] tracking-wider uppercase'>
              {active.kind === 'protocol' ? '200 ok' : 'setup'}
            </span>
          </div>
        </div>

        {active.kind === 'protocol' ? (
          <>
            <div
              className={cn(
                'flex items-center gap-2.5 border-b px-5 py-3',
                'border-border/40 dark:border-white/[0.04]'
              )}
            >
              <span
                className={cn(
                  'rounded-md px-1.5 py-0.5 font-mono text-[10px] font-semibold tracking-wider',
                  accent.badge
                )}
              >
                {active.method}
              </span>
              <code className='text-foreground/75 truncate font-mono text-[12.5px]'>
                {active.endpoint}
              </code>
            </div>
            <div className='grid min-h-[300px] grid-rows-2 font-mono text-[12.5px] leading-[1.55]'>
              <div className='px-5 py-4'>
                <div className='text-foreground/40 mb-2 text-[10px] font-bold tracking-wider uppercase'>
                  Request
                </div>
                <pre className='text-foreground/80 whitespace-pre-wrap'>
                  <span className='text-teal-700 dark:text-emerald-400'>curl</span>{' '}
                  <span className='text-amber-700 dark:text-amber-300'>-X POST</span>{' '}
                  <span className='text-violet-700 dark:text-violet-300'>
                    &quot;{active.endpoint}&quot;
                  </span>
                  {' \\\n'}
                  {active.headers.map((h) => (
                    <span key={h}>
                      {'  '}
                      <span className='text-amber-700 dark:text-amber-300'>-H</span>{' '}
                      <span className='text-violet-700 dark:text-violet-300'>
                        {h}
                      </span>
                      {' \\\n'}
                    </span>
                  ))}
                  {'  '}
                  <span className='text-amber-700 dark:text-amber-300'>-d</span>{' '}
                  <span className='text-violet-700 dark:text-violet-300'>
                    &apos;{'{'}
                    {'\n'}
                    {active.request.map((line) => `    ${line}\n`).join('')}
                    {'  }'}&apos;
                  </span>
                </pre>
              </div>
              <div className='border-border/40 border-t px-5 py-4 dark:border-white/[0.04]'>
                <div className='text-foreground/40 mb-2 text-[10px] font-bold tracking-wider uppercase'>
                  Response
                </div>
                <pre className='text-foreground/80 whitespace-pre-wrap'>
                  {active.response.join('\n')}
                </pre>
              </div>
            </div>
            <div
              className={cn(
                'flex items-center justify-between border-t px-5 py-2.5',
                'border-border/40 bg-muted/30 dark:border-white/[0.05] dark:bg-white/[0.02]'
              )}
            >
              <div className='text-foreground/40 flex items-center gap-3 text-[10px] tabular-nums'>
                <span className='font-mono'>
                  {active.latency}{' '}
                  <span className='tracking-wider uppercase'>ms</span>
                </span>
                <span className='bg-foreground/15 size-1 rounded-full' />
                <span className='font-mono'>
                  {active.tokens}{' '}
                  <span className='tracking-wider uppercase'>tokens</span>
                </span>
              </div>
              <span className='text-foreground/30 font-mono text-[10px] tracking-wider uppercase'>
                stream · sse
              </span>
            </div>
          </>
        ) : (
          <div className='space-y-3 px-5 py-4 text-left'>
            <div className='flex flex-wrap gap-2'>
              {active.chips.map((chip, i) => (
                <span
                  key={chip}
                  className={cn(
                    'rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-wider uppercase',
                    i === 0
                      ? accent.badge + ' border-transparent'
                      : 'text-foreground/50 border-border/60'
                  )}
                >
                  {chip}
                </span>
              ))}
            </div>
            {active.snippets.map((snippet) => (
              <div key={snippet.id} className='space-y-2'>
                <pre
                  className={cn(
                    'overflow-auto rounded-xl border p-4 font-mono text-[12.5px] leading-[1.55] whitespace-pre-wrap',
                    'border-border/50 bg-slate-50 text-slate-900',
                    'dark:border-white/[0.06] dark:bg-white/[0.03] dark:text-slate-100'
                  )}
                >
                  {snippet.body}
                </pre>
                <div className='flex justify-end'>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    className='h-8 rounded-lg text-xs'
                    onClick={() => handleCopy(snippet.body, snippet.label)}
                  >
                    {t('Copy {{label}}', { label: snippet.label })}
                  </Button>
                </div>
              </div>
            ))}
            <p className='text-muted-foreground text-xs leading-relaxed'>
              {t(active.hint)}
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
