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

type Star = {
  x: number
  y: number
  r: number
  a: number
  tw: number
  sp: number
  bright: boolean
}

type Meteor = {
  x: number
  y: number
  vx: number
  vy: number
  age: number
  life: number
}

function isDarkMode() {
  return document.documentElement.classList.contains('dark')
}

/**
 * 首页星空底（亮暗双模式，与经典前端参数完全一致）：
 * 亮色为浅蓝天 + 淡蓝星点；暗色为深空夜空 + 闪烁星点 + 偶发流星。
 * 遵循 prefers-reduced-motion：静止星点、无流星。
 */
export function StarfieldBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let stars: Star[] = []
    let meteors: Meteor[] = []
    let nextMeteorAt = 0
    let lastFrameAt = 0
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
      const count = dark
        ? Math.min(380, Math.floor((w * h) / 3000))
        : Math.min(160, Math.floor((w * h) / 7000))
      stars = Array.from({ length: count }, () => ({
        x: Math.random() * w,
        y: Math.random() * h,
        r: dark ? Math.random() * 1.5 + 0.3 : Math.random() * 1.05 + 0.25,
        a: dark ? Math.random() * 0.55 + 0.25 : Math.random() * 0.23 + 0.12,
        tw: Math.random() * Math.PI * 2,
        sp: dark
          ? 0.006 + Math.random() * 0.016
          : 0.0015 + Math.random() * 0.004,
        bright: dark ? Math.random() > 0.9 : Math.random() > 0.96,
      }))
    }

    const spawnMeteor = (w: number, h: number) => {
      // 从上半部随机位置出发，沿左下或右下方向划过
      const toRight = Math.random() > 0.5
      meteors.push({
        x: w * 0.15 + Math.random() * w * 0.7,
        y: Math.random() * h * 0.35,
        vx: (toRight ? 1 : -1) * (320 + Math.random() * 280),
        vy: 150 + Math.random() * 120,
        age: 0,
        life: 0.7 + Math.random() * 0.4,
      })
      nextMeteorAt = lastFrameAt + 2500 + Math.random() * 3500
    }

    const frame = (t: number) => {
      if (destroyed) return
      const w = window.innerWidth
      const h = window.innerHeight
      const reduce = window.matchMedia(
        '(prefers-reduced-motion: reduce)'
      ).matches
      const dt = lastFrameAt ? Math.min(0.05, (t - lastFrameAt) / 1000) : 0
      lastFrameAt = t

      const dark = isDarkMode()
      ctx.clearRect(0, 0, w, h)
      for (const s of stars) {
        const tw = reduce
          ? 1
          : dark
            ? 0.55 + 0.45 * Math.sin(t * s.sp + s.tw)
            : 0.9 + 0.1 * Math.sin(t * s.sp + s.tw)
        const alpha = s.a * tw
        ctx.beginPath()
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

      // 流星仅暗色模式（亮色背景下不可见）
      if (!reduce && dark) {
        if (t >= nextMeteorAt && meteors.length < 2) {
          spawnMeteor(w, h)
        }
        meteors = meteors.filter((m) => m.age < m.life)
        for (const m of meteors) {
          m.age += dt
          m.x += m.vx * dt
          m.y += m.vy * dt
          const fade = Math.sin((Math.PI * Math.min(m.age, m.life)) / m.life)
          const tail = 0.16
          const tx = m.x - m.vx * tail
          const ty = m.y - m.vy * tail
          const gradient = ctx.createLinearGradient(m.x, m.y, tx, ty)
          gradient.addColorStop(0, `rgba(255,255,255,${0.85 * fade})`)
          gradient.addColorStop(1, 'rgba(125,211,252,0)')
          ctx.beginPath()
          ctx.strokeStyle = gradient
          ctx.lineWidth = 1.6
          ctx.lineCap = 'round'
          ctx.moveTo(m.x, m.y)
          ctx.lineTo(tx, ty)
          ctx.stroke()
          ctx.beginPath()
          ctx.fillStyle = `rgba(255,255,255,${0.9 * fade})`
          ctx.arc(m.x, m.y, 1.2, 0, Math.PI * 2)
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
      {/* 天空底 + 星云光晕（亮暗双模式，与经典前端一致） */}
      <div className='absolute inset-0 bg-[linear-gradient(180deg,#eef4ff_0%,#e0ebff_55%,#d7e3ff_100%)] dark:bg-[linear-gradient(180deg,#020617_0%,#060b1d_55%,#0b1030_100%)]' />
      <div className='absolute inset-0 bg-[radial-gradient(ellipse_90%_65%_at_12%_-10%,rgba(59,130,246,0.26),transparent_55%),radial-gradient(ellipse_70%_55%_at_92%_8%,rgba(56,189,248,0.2),transparent_55%),radial-gradient(ellipse_85%_60%_at_50%_112%,rgba(129,140,248,0.16),transparent_60%)] opacity-90 dark:bg-[radial-gradient(ellipse_90%_65%_at_12%_-10%,rgba(56,189,248,0.22),transparent_55%),radial-gradient(ellipse_70%_55%_at_92%_8%,rgba(129,140,248,0.2),transparent_55%),radial-gradient(ellipse_85%_60%_at_50%_112%,rgba(168,85,247,0.16),transparent_60%)]' />
      <div className='absolute -inset-[10%] -rotate-[18deg] bg-[radial-gradient(ellipse_40%_18%_at_50%_42%,rgba(14,165,233,0.16),transparent_70%),radial-gradient(ellipse_28%_12%_at_58%_48%,rgba(99,102,241,0.12),transparent_70%)] opacity-80 blur-md dark:bg-[radial-gradient(ellipse_40%_18%_at_50%_42%,rgba(129,140,248,0.22),transparent_70%),radial-gradient(ellipse_28%_12%_at_58%_48%,rgba(168,85,247,0.18),transparent_70%)]' />
      <canvas ref={canvasRef} className='absolute inset-0 h-full w-full' />
      {/* 边缘压暗/压亮，聚焦中央内容 */}
      <div className='absolute inset-0 bg-[radial-gradient(ellipse_75%_70%_at_50%_40%,transparent_45%,rgba(238,244,255,0.5)_100%)] dark:bg-[radial-gradient(ellipse_75%_70%_at_50%_40%,transparent_45%,rgba(2,6,23,0.5)_100%)]' />
    </div>
  )
}
