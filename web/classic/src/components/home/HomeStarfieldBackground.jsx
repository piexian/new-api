/*
Copyright (C) 2025 QuantumNous

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

import React, { useEffect, useRef } from 'react';
import { useActualTheme } from '../../context/Theme';

function isDarkTheme(theme) {
  return (
    theme === 'dark' || document.body.getAttribute('theme-mode') === 'dark'
  );
}

/**
 * Classic 星空底：结构与 default 对齐（浅蓝白光晕 + 深色夜空）
 */
export default function HomeStarfieldBackground() {
  const canvasRef = useRef(null);
  const actualTheme = useActualTheme();

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let stars = [];
    let raf = 0;
    let destroyed = false;

    const paint = () => {
      const dpr = window.devicePixelRatio || 1;
      const w = window.innerWidth;
      const h = window.innerHeight;
      canvas.width = w * dpr;
      canvas.height = h * dpr;
      canvas.style.width = `${w}px`;
      canvas.style.height = `${h}px`;
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.scale(dpr, dpr);

      const dark = isDarkTheme(actualTheme);
      const count = dark
        ? Math.min(380, Math.floor((w * h) / 3000))
        : Math.min(140, Math.floor((w * h) / 7800));
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
      }));
    };

    const frame = (t) => {
      if (destroyed) return;
      const w = window.innerWidth;
      const h = window.innerHeight;
      ctx.clearRect(0, 0, w, h);
      const dark = isDarkTheme(actualTheme);
      const reduce = window.matchMedia(
        '(prefers-reduced-motion: reduce)',
      ).matches;
      for (const s of stars) {
        const tw = reduce
          ? 1
          : dark
            ? 0.55 + 0.45 * Math.sin(t * s.sp + s.tw)
            : 0.94 + 0.06 * Math.sin(t * s.sp + s.tw);
        const alpha = s.a * tw;
        ctx.beginPath();
        ctx.fillStyle = dark
          ? `rgba(240,248,255,${alpha})`
          : `rgba(37,99,235,${alpha})`;
        ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2);
        ctx.fill();
        if (s.bright) {
          ctx.beginPath();
          ctx.fillStyle = dark
            ? `rgba(186,230,253,${alpha * 0.28})`
            : `rgba(14,165,233,${alpha * 0.22})`;
          ctx.arc(s.x, s.y, s.r * 2.4, 0, Math.PI * 2);
          ctx.fill();
        }
      }
      if (!reduce) raf = requestAnimationFrame(frame);
    };

    paint();
    raf = requestAnimationFrame(frame);
    const onResize = () => {
      cancelAnimationFrame(raf);
      paint();
      raf = requestAnimationFrame(frame);
    };
    window.addEventListener('resize', onResize);
    return () => {
      destroyed = true;
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', onResize);
    };
  }, [actualTheme]);

  const dark = isDarkTheme(actualTheme);

  return (
    <div
      aria-hidden
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 0,
        pointerEvents: 'none',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background: dark
            ? 'linear-gradient(180deg,#040812 0%,#070d1c 100%)'
            : 'linear-gradient(180deg,#f0f6ff 0%,#dce9ff 100%)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          inset: 0,
          opacity: 0.9,
          background: dark
            ? 'radial-gradient(ellipse 90% 70% at 10% -10%, rgba(56,189,248,0.28), transparent 55%), radial-gradient(ellipse 70% 55% at 95% 5%, rgba(129,140,248,0.24), transparent 55%), radial-gradient(ellipse 80% 60% at 50% 110%, rgba(168,85,247,0.2), transparent 60%)'
            : 'radial-gradient(ellipse 90% 70% at 10% -10%, rgba(59,130,246,0.42), transparent 55%), radial-gradient(ellipse 70% 55% at 95% 5%, rgba(14,165,233,0.32), transparent 55%), radial-gradient(ellipse 80% 60% at 50% 110%, rgba(99,102,241,0.22), transparent 60%)',
        }}
      />
      <canvas
        ref={canvasRef}
        style={{
          position: 'absolute',
          inset: 0,
          width: '100%',
          height: '100%',
        }}
      />
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background: dark
            ? 'radial-gradient(ellipse 75% 70% at 50% 40%, transparent 40%, rgba(4,8,18,0.55) 100%)'
            : 'radial-gradient(ellipse 75% 70% at 50% 40%, transparent 40%, rgba(240,246,255,0.55) 100%)',
        }}
      />
    </div>
  );
}
