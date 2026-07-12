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
      // 浅色更少更淡；深色更密
      const count = dark
        ? Math.min(380, Math.floor((w * h) / 3000))
        : Math.min(160, Math.floor((w * h) / 7000))
      stars = Array.from({ length: count }, () => ({
        x: Math.random() * w,
        y: Math.random() * h,
        r: dark ? Math.random() * 1.5 + 0.3 : Math.random() * 1.1 + 0.25,
        a: dark ? Math.random() * 0.55 + 0.25 : Math.random() * 0.18 + 0.08,
        tw: Math.random() * Math.PI * 2,
        sp: dark ? 0.006 + Math.random() * 0.016 : 0.0015 + Math.random() * 0.004,
        bright: dark ? Math.random() > 0.9 : Math.random() > 0.97,
      }))
    }

    const frame = (t: number) => {
      if (destroyed) return
      const w = window.innerWidth
      const h = window.innerHeight
      ctx.clearRect(0, 0, w, h)
      const dark = isDarkMode()
      const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
      for (const s of stars) {
        const tw = reduce
          ? 1
          : dark
            ? 0.55 + 0.45 * Math.sin(t * s.sp + s.tw)
            : 0.92 + 0.08 * Math.sin(t * s.sp + s.tw)
        const alpha = s.a * tw
        ctx.beginPath()
        ctx.fillStyle = dark
          ? `rgba(240,248,255,${alpha})`
          : `rgba(59,100,180,${alpha})`
        ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2)
        ctx.fill()
        if (s.bright && dark) {
          ctx.beginPath()
          ctx.fillStyle = `rgba(186,230,253,${alpha * 0.28})`
          ctx.arc(s.x, s.y, s.r * 2.8, 0, Math.PI * 2)
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
    <canvas
      ref={canvasRef}
      aria-hidden
      className='pointer-events-none fixed inset-0 z-0'
    />
  )
}
