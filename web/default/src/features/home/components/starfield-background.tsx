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
import { useEffect, useRef } from 'react'
import { cn } from '@/lib/utils'

type Star = {
  x: number
  y: number
  r: number
  a: number
  tw: number
  sp: number
  bright: boolean
}

function isDarkMode() {
  return document.documentElement.classList.contains('dark')
}

export function StarfieldBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let stars: Star[] = []
    let raf = 0
    let destroyed = false

    const paint = () => {
      const dpr = window.devicePixelRatio || 1
      const w = window.innerWidth
      const h = window.innerHeight
      canvas.width = w * dpr
      canvas.height = h * dpr
      canvas.style.width = `${w}px`
      canvas.style.height = `${h}px`
      ctx.setTransform(1, 0, 0, 1, 0, 0)
      ctx.scale(dpr, dpr)

      const dark = isDarkMode()
      // 浅色：更少更淡更稳；深色：更密可轻闪（对齐 mockup）
      const count = dark
        ? Math.min(380, Math.floor((w * h) / 3000))
        : Math.min(140, Math.floor((w * h) / 7800))
      stars = Array.from({ length: count }, () => ({
        x: Math.random() * w,
        y: Math.random() * h,
        r: dark ? Math.random() * 1.5 + 0.3 : Math.random() * 1.05 + 0.25,
        a: dark ? Math.random() * 0.55 + 0.25 : Math.random() * 0.22 + 0.1,
        tw: Math.random() * Math.PI * 2,
        sp: dark
          ? 0.006 + Math.random() * 0.016
          : 0.0012 + Math.random() * 0.0035,
        bright: dark ? Math.random() > 0.9 : Math.random() > 0.96,
      }))
    }

    const frame = (t: number) => {
      if (destroyed) return
      const w = window.innerWidth
      const h = window.innerHeight
      ctx.clearRect(0, 0, w, h)
      const dark = isDarkMode()
      const reduce = window.matchMedia(
        '(prefers-reduced-motion: reduce)'
      ).matches
      for (const s of stars) {
        const tw = reduce
          ? 1
          : dark
            ? 0.55 + 0.45 * Math.sin(t * s.sp + s.tw)
            : 0.94 + 0.06 * Math.sin(t * s.sp + s.tw)
        const alpha = s.a * tw
        ctx.beginPath()
        // 浅色用冷蓝星点，贴合浅蓝白背景；深色用近白星
        ctx.fillStyle = dark
          ? `rgba(240,248,255,${alpha})`
          : `rgba(37,99,235,${alpha})`
        ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2)
        ctx.fill()
        if (s.bright) {
          ctx.beginPath()
          ctx.fillStyle = dark
            ? `rgba(186,230,253,${alpha * 0.28})`
            : `rgba(14,165,233,${alpha * 0.22})`
          ctx.arc(s.x, s.y, s.r * 2.4, 0, Math.PI * 2)
          ctx.fill()
        }
      }
      if (!reduce) raf = requestAnimationFrame(frame)
    }

    paint()
    raf = requestAnimationFrame(frame)

    const onResize = () => {
      cancelAnimationFrame(raf)
      paint()
      raf = requestAnimationFrame(frame)
    }
    window.addEventListener('resize', onResize)

    const mo = new MutationObserver(() => {
      cancelAnimationFrame(raf)
      paint()
      raf = requestAnimationFrame(frame)
    })
    mo.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })

    return () => {
      destroyed = true
      cancelAnimationFrame(raf)
      window.removeEventListener('resize', onResize)
      mo.disconnect()
    }
  }, [])

  return (
    <div
      aria-hidden
      className='pointer-events-none fixed inset-0 z-0 overflow-hidden'
    >
      {/* 浅色：蓝白光晕底；深色：夜空光晕 — 与演示卡片同色域 */}
      <div
        className={cn(
          'absolute inset-0',
          'bg-[linear-gradient(180deg,#f0f6ff_0%,#dce9ff_100%)]',
          'dark:bg-[linear-gradient(180deg,#040812_0%,#070d1c_100%)]'
        )}
      />
      <div
        className={cn(
          'absolute inset-0 opacity-90',
          'bg-[radial-gradient(ellipse_90%_70%_at_10%_-10%,rgba(59,130,246,0.42),transparent_55%),radial-gradient(ellipse_70%_55%_at_95%_5%,rgba(14,165,233,0.32),transparent_55%),radial-gradient(ellipse_80%_60%_at_50%_110%,rgba(99,102,241,0.22),transparent_60%)]',
          'dark:bg-[radial-gradient(ellipse_90%_70%_at_10%_-10%,rgba(56,189,248,0.28),transparent_55%),radial-gradient(ellipse_70%_55%_at_95%_5%,rgba(129,140,248,0.24),transparent_55%),radial-gradient(ellipse_80%_60%_at_50%_110%,rgba(168,85,247,0.2),transparent_60%)]'
        )}
      />
      <div
        className={cn(
          'absolute -inset-[10%] -rotate-[18deg] opacity-80 blur-md',
          'bg-[radial-gradient(ellipse_40%_18%_at_50%_42%,rgba(14,165,233,0.35),transparent_70%),radial-gradient(ellipse_28%_12%_at_58%_48%,rgba(99,102,241,0.28),transparent_70%)]',
          'dark:bg-[radial-gradient(ellipse_40%_18%_at_50%_42%,rgba(129,140,248,0.28),transparent_70%),radial-gradient(ellipse_28%_12%_at_58%_48%,rgba(168,85,247,0.22),transparent_70%)]'
        )}
      />
      <canvas ref={canvasRef} className='absolute inset-0 h-full w-full' />
      <div
        className={cn(
          'absolute inset-0',
          'bg-[radial-gradient(ellipse_75%_70%_at_50%_40%,transparent_40%,rgba(240,246,255,0.55)_100%)]',
          'dark:bg-[radial-gradient(ellipse_75%_70%_at_50%_40%,transparent_40%,rgba(4,8,18,0.55)_100%)]'
        )}
      />
    </div>
  )
}
