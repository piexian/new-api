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
import { ExternalLink, Link2 } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

const STORAGE_KEY = 'newapi.floating_ball_position'
const DRAG_THRESHOLD = 6
const EDGE_PAD = 8
const BTN = 54

type FriendLink = {
  name: string
  url: string
  icon?: string
  description?: string
  order?: number
  enabled?: boolean
}

type BallPos = { x: number; y: number }

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n))
}

function defaultPos(): BallPos {
  if (typeof window === 'undefined') return { x: EDGE_PAD, y: 120 }
  return {
    x: EDGE_PAD,
    y: Math.max(EDGE_PAD, window.innerHeight - BTN - 24),
  }
}

function readPos(): BallPos {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return defaultPos()
    const parsed: unknown = JSON.parse(raw)
    if (
      parsed &&
      typeof parsed === 'object' &&
      'x' in parsed &&
      'y' in parsed &&
      typeof parsed.x === 'number' &&
      typeof parsed.y === 'number'
    ) {
      return clampPos({ x: parsed.x, y: parsed.y })
    }
  } catch {
    /* empty */
  }
  return defaultPos()
}

function clampPos(pos: BallPos): BallPos {
  if (typeof window === 'undefined') return pos
  const maxX = Math.max(EDGE_PAD, window.innerWidth - BTN - EDGE_PAD)
  const maxY = Math.max(EDGE_PAD, window.innerHeight - BTN - EDGE_PAD)
  return {
    x: clamp(pos.x, EDGE_PAD, maxX),
    y: clamp(pos.y, EDGE_PAD, maxY),
  }
}

function parseFriendLinks(status: unknown): FriendLink[] {
  if (!status || typeof status !== 'object') return []
  const enabled =
    !('friend_links_enabled' in status) || status.friend_links_enabled !== false
  if (!enabled) return []
  if (!('friend_links' in status)) return []
  const list = status.friend_links
  if (!Array.isArray(list)) return []
  const out: FriendLink[] = []
  for (const item of list) {
    if (!item || typeof item !== 'object') continue
    if (!('name' in item) || !('url' in item)) continue
    if (typeof item.name !== 'string' || typeof item.url !== 'string') continue
    if ('enabled' in item && item.enabled === false) continue
    out.push({
      name: item.name,
      url: item.url,
      icon:
        'icon' in item && typeof item.icon === 'string' ? item.icon : undefined,
      description:
        'description' in item && typeof item.description === 'string'
          ? item.description
          : undefined,
      order: 'order' in item && typeof item.order === 'number' ? item.order : 0,
    })
  }
  out.sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
  return out
}

function isHttpIcon(icon?: string) {
  if (!icon) return false
  const v = icon.trim().toLowerCase()
  return v.startsWith('http://') || v.startsWith('https://')
}

function FriendLinkIcon(props: {
  icon?: string
  name: string
  className?: string
}) {
  const icon = (props.icon || '').trim()
  if (icon && isHttpIcon(icon)) {
    return (
      <img
        src={icon}
        alt=''
        className={props.className ?? 'size-8 rounded-lg object-cover'}
      />
    )
  }
  if (icon) {
    return (
      <div
        className={
          props.className ??
          'bg-primary/10 flex size-8 items-center justify-center rounded-lg text-base leading-none'
        }
        aria-hidden
      >
        {icon}
      </div>
    )
  }
  return (
    <div className='bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg text-xs font-extrabold'>
      {props.name.slice(0, 1).toUpperCase()}
    </div>
  )
}

export function FloatingFriendLinks() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const links = useMemo(() => parseFriendLinks(status), [status])
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState<BallPos>(() =>
    typeof window === 'undefined' ? { x: EDGE_PAD, y: 120 } : readPos()
  )
  const dragRef = useRef<{
    pointerId: number
    startX: number
    startY: number
    origX: number
    origY: number
    dragged: boolean
  } | null>(null)

  useEffect(() => {
    const onResize = () => setPos((p) => clampPos(p))
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  if (links.length === 0) return null

  const panelRight = pos.x + BTN / 2 > window.innerWidth / 2

  const onPointerDown = (e: React.PointerEvent<HTMLButtonElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    dragRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      startY: e.clientY,
      origX: pos.x,
      origY: pos.y,
      dragged: false,
    }
  }

  const onPointerMove = (e: React.PointerEvent<HTMLButtonElement>) => {
    const d = dragRef.current
    if (!d || d.pointerId !== e.pointerId) return
    const dx = e.clientX - d.startX
    const dy = e.clientY - d.startY
    if (!d.dragged && Math.hypot(dx, dy) < DRAG_THRESHOLD) return
    d.dragged = true
    setPos(clampPos({ x: d.origX + dx, y: d.origY + dy }))
  }

  const onPointerUp = (e: React.PointerEvent<HTMLButtonElement>) => {
    const d = dragRef.current
    if (!d || d.pointerId !== e.pointerId) return
    try {
      e.currentTarget.releasePointerCapture(e.pointerId)
    } catch {
      /* empty */
    }
    if (d.dragged) {
      setPos((p) => {
        const next = clampPos(p)
        try {
          localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
        } catch {
          /* empty */
        }
        return next
      })
    } else {
      setOpen((v) => !v)
    }
    dragRef.current = null
  }

  return (
    <div
      className='fixed z-[60]'
      style={{ left: pos.x, top: pos.y, width: BTN, height: BTN }}
    >
      {open ? (
        <div
          className={cn(
            'absolute bottom-[calc(100%+10px)] w-64 overflow-hidden rounded-2xl border shadow-xl backdrop-blur-md',
            'border-border/60 bg-card/95',
            panelRight ? 'right-0' : 'left-0'
          )}
        >
          <div className='border-b px-3 py-2 text-sm font-bold'>
            {t('Friend Links')}
          </div>
          <div className='max-h-72 overflow-auto p-2'>
            {links.map((link) => (
              <a
                key={`${link.name}-${link.url}`}
                href={link.url}
                target='_blank'
                rel='noreferrer noopener'
                className='hover:bg-muted/60 flex items-center gap-3 rounded-xl px-2 py-2 transition-colors'
              >
                <FriendLinkIcon icon={link.icon} name={link.name} />
                <div className='min-w-0 flex-1'>
                  <div className='flex items-center gap-1 truncate text-sm font-semibold'>
                    {link.name}
                    <ExternalLink className='text-muted-foreground size-3 shrink-0' />
                  </div>
                  {link.description ? (
                    <div className='text-muted-foreground truncate text-[11px]'>
                      {link.description}
                    </div>
                  ) : null}
                </div>
              </a>
            ))}
          </div>
        </div>
      ) : null}
      <button
        type='button'
        aria-label={t('Friend Links')}
        aria-expanded={open}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        className={cn(
          'flex size-[54px] items-center justify-center rounded-full border shadow-lg select-none',
          'border-border/50 bg-gradient-to-br from-sky-500 to-indigo-600 text-white',
          'cursor-grab touch-none active:cursor-grabbing'
        )}
      >
        <Link2 className='size-5' />
      </button>
    </div>
  )
}
