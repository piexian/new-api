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
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

const ITEMS = [
  {
    title: 'Lightning fast?',
    desc: 'No global edge optimization yet',
  },
  {
    title: 'High availability?',
    desc: 'Personal hobby service',
  },
  {
    title: 'Community free',
    desc: 'Powered by love — may disappear anytime',
  },
] as const

export function ServiceStrip() {
  const { t } = useTranslation()

  return (
    <section
      aria-label={t('Service notes')}
      className='relative z-10 mx-auto w-full max-w-5xl px-6 pb-16'
    >
      <div className='grid gap-3 md:grid-cols-3'>
        {ITEMS.map((item) => (
          <div
            key={item.title}
            className={cn(
              'rounded-2xl border px-4 py-5 text-center backdrop-blur-md',
              'border-border/50 bg-card/70 shadow-sm'
            )}
          >
            <div className='text-sm font-extrabold'>{t(item.title)}</div>
            <div className='text-muted-foreground mt-1.5 text-xs leading-relaxed'>
              {t(item.desc)}
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
