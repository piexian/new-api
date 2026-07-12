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
    title: '极速响应？',
    desc: '全球节点均未优化',
  },
  {
    title: '稳定高可用？',
    desc: '私人自用服务',
  },
  {
    title: '公益免费',
    desc: '用爱发电 随时跑路',
  },
] as const

export function ServiceStrip() {
  const { t } = useTranslation()

  return (
    <section
      aria-label={t('服务说明')}
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
