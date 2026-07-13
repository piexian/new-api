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
import { Link } from '@tanstack/react-router'
import { ArrowRight, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { resolveAppRoute } from '@/lib/frontend-routes'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { HeroCapabilityTabs } from '../hero-capability-tabs'

type HeroProps = {
  className?: string
  isAuthenticated?: boolean
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()

  const serverAddress = useMemo(() => {
    let fromStatus = ''
    if (status && typeof status === 'object' && 'server_address' in status) {
      const raw = status.server_address
      if (typeof raw === 'string') fromStatus = raw.trim()
    }
    if (fromStatus) return fromStatus
    if (typeof window !== 'undefined') return window.location.origin
    return ''
  }, [status])

  const handleCopyBaseURL = async () => {
    if (!serverAddress) return
    const ok = await copyToClipboard(serverAddress)
    if (ok) toast.success(t('Copied!'))
    else toast.error(t('Failed to copy'))
  }

  const primaryTo = props.isAuthenticated
    ? resolveAppRoute('dashboard')
    : resolveAppRoute('sign_up')
  const pricingTo = resolveAppRoute('pricing')

  return (
    <section
      className={cn(
        'relative z-10 flex flex-col items-center overflow-hidden px-6 pt-28 pb-10 md:pt-36 md:pb-14',
        props.className
      )}
    >
      <div className='flex max-w-3xl flex-col items-center text-center'>
        <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-[0.18em] uppercase'>
          ✦ Starfield Gateway
        </p>
        <h1 className='text-[clamp(2rem,5.5vw,3.5rem)] leading-[1.15] font-bold tracking-tight'>
          {t('Unified')}
          <br />
          <span className='bg-gradient-to-r from-blue-500 via-sky-500 to-violet-500 bg-clip-text text-transparent dark:from-blue-400 dark:via-violet-400 dark:to-purple-500'>
            {t('AI API Gateway')}
          </span>
        </h1>
        <p className='text-muted-foreground/80 mt-5 max-w-lg text-base leading-relaxed md:text-lg'>
          {t('One base URL for multi-model access:')}
        </p>

        <div
          className={cn(
            'mt-6 flex w-full max-w-xl items-stretch overflow-hidden rounded-2xl border',
            'border-border/60 bg-card/80 shadow-sm backdrop-blur-md'
          )}
          title={t('Click to copy API base URL')}
        >
          <div className='text-muted-foreground flex shrink-0 items-center border-r px-3 text-[11px] font-bold tracking-wider uppercase'>
            BASE URL
          </div>
          <button
            type='button'
            onClick={handleCopyBaseURL}
            className='min-w-0 flex-1 truncate px-3 py-3 text-left font-mono text-sm font-semibold'
          >
            {serverAddress || '—'}
          </button>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='h-auto rounded-none border-l px-4 font-bold'
            onClick={handleCopyBaseURL}
          >
            <Copy className='mr-1.5 size-3.5' />
            {t('Copy')}
          </Button>
        </div>

        <div className='mt-8 flex flex-wrap items-center justify-center gap-3'>
          <Button className='group rounded-lg' render={<Link to={primaryTo} />}>
            {props.isAuthenticated
              ? t('Go to Dashboard')
              : t('Get API Key / Console')}
            <ArrowRight className='ml-1 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
          </Button>
          <Button
            variant='outline'
            className='border-border/50 hover:border-border hover:bg-muted/50 rounded-lg'
            render={<Link to={pricingTo} />}
          >
            {t('Model Square')}
          </Button>
        </div>
      </div>

      <HeroCapabilityTabs />
    </section>
  )
}
