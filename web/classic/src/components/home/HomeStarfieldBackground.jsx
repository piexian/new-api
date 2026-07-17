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
import { useDomDarkTheme } from '../../hooks/common/useDomDarkTheme';

/**
 * 首页星空底（亮暗双模式，与新版前端参数完全一致）：
 * 亮色为浅蓝天 + 淡蓝星点；暗色为深空夜空 + 闪烁星点 + 偶发流星。
 * 遵循 prefers-reduced-motion：静止星点、无流星。
 */
export default function HomeStarfieldBackground() {
  const canvasRef = useRef(null);
  const dark = useDomDarkTheme();

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let stars = [];
    let meteors = [];
    let nextMeteorAt = 0;
    let lastFrameAt = 0;
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

      const count = dark
        ? Math.min(380, Math.floor((w * h) / 3000))
        : Math.min(160, Math.floor((w * h) / 7000));
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
      }));
    };

    const spawnMeteor = (w, h) => {
      // 从上半部随机位置出发，沿左下或右下方向划过
      const toRight = Math.random() > 0.5;
      meteors.push({
        x: w * 0.15 + Math.random() * w * 0.7,
        y: Math.random() * h * 0.35,
        vx: (toRight ? 1 : -1) * (320 + Math.random() * 280),
        vy: 150 + Math.random() * 120,
        age: 0,
        life: 0.7 + Math.random() * 0.4,
      });
      nextMeteorAt = lastFrameAt + 2500 + Math.random() * 3500;
    };

    const frame = (t) => {
      if (destroyed) return;
      const w = window.innerWidth;
      const h = window.innerHeight;
      const reduce = window.matchMedia(
        '(prefers-reduced-motion: reduce)',
      ).matches;
      const dt = lastFrameAt ? Math.min(0.05, (t - lastFrameAt) / 1000) : 0;
      lastFrameAt = t;

      ctx.clearRect(0, 0, w, h);
      for (const s of stars) {
        const tw = reduce
          ? 1
          : dark
            ? 0.55 + 0.45 * Math.sin(t * s.sp + s.tw)
            : 0.9 + 0.1 * Math.sin(t * s.sp + s.tw);
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

      // 流星仅暗色模式（亮色背景下不可见）
      if (!reduce && dark) {
        if (t >= nextMeteorAt && meteors.length < 2) {
          spawnMeteor(w, h);
        }
        meteors = meteors.filter((m) => m.age < m.life);
        for (const m of meteors) {
          m.age += dt;
          m.x += m.vx * dt;
          m.y += m.vy * dt;
          const fade = Math.sin((Math.PI * Math.min(m.age, m.life)) / m.life);
          const tail = 0.16;
          const tx = m.x - m.vx * tail;
          const ty = m.y - m.vy * tail;
          const gradient = ctx.createLinearGradient(m.x, m.y, tx, ty);
          gradient.addColorStop(0, `rgba(255,255,255,${0.85 * fade})`);
          gradient.addColorStop(1, 'rgba(125,211,252,0)');
          ctx.beginPath();
          ctx.strokeStyle = gradient;
          ctx.lineWidth = 1.6;
          ctx.lineCap = 'round';
          ctx.moveTo(m.x, m.y);
          ctx.lineTo(tx, ty);
          ctx.stroke();
          ctx.beginPath();
          ctx.fillStyle = `rgba(255,255,255,${0.9 * fade})`;
          ctx.arc(m.x, m.y, 1.2, 0, Math.PI * 2);
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
  }, [dark]);

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
            ? 'linear-gradient(180deg,#020617 0%,#060b1d 55%,#0b1030 100%)'
            : 'linear-gradient(180deg,#eef4ff 0%,#e0ebff 55%,#d7e3ff 100%)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          inset: 0,
          opacity: 0.9,
          background: dark
            ? 'radial-gradient(ellipse 90% 65% at 12% -10%, rgba(56,189,248,0.22), transparent 55%), radial-gradient(ellipse 70% 55% at 92% 8%, rgba(129,140,248,0.2), transparent 55%), radial-gradient(ellipse 85% 60% at 50% 112%, rgba(168,85,247,0.16), transparent 60%)'
            : 'radial-gradient(ellipse 90% 65% at 12% -10%, rgba(59,130,246,0.26), transparent 55%), radial-gradient(ellipse 70% 55% at 92% 8%, rgba(56,189,248,0.2), transparent 55%), radial-gradient(ellipse 85% 60% at 50% 112%, rgba(129,140,248,0.16), transparent 60%)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          inset: '-10%',
          transform: 'rotate(-18deg)',
          opacity: 0.8,
          filter: 'blur(6px)',
          background: dark
            ? 'radial-gradient(ellipse 40% 18% at 50% 42%, rgba(129,140,248,0.22), transparent 70%), radial-gradient(ellipse 28% 12% at 58% 48%, rgba(168,85,247,0.18), transparent 70%)'
            : 'radial-gradient(ellipse 40% 18% at 50% 42%, rgba(14,165,233,0.16), transparent 70%), radial-gradient(ellipse 28% 12% at 58% 48%, rgba(99,102,241,0.12), transparent 70%)',
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
      {/* 边缘压暗/压亮，聚焦中央内容 */}
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background: dark
            ? 'radial-gradient(ellipse 75% 70% at 50% 40%, transparent 45%, rgba(2,6,23,0.5) 100%)'
            : 'radial-gradient(ellipse 75% 70% at 50% 40%, transparent 45%, rgba(238,244,255,0.5) 100%)',
        }}
      />
    </div>
  );
}
