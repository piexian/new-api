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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../context/Status';

const STORAGE_KEY = 'newapi.floating_ball_position';
const DRAG_THRESHOLD = 6;
const EDGE_PAD = 8;
const BTN = 54;

function clamp(n, min, max) {
  return Math.min(max, Math.max(min, n));
}

function defaultPos() {
  return {
    x: EDGE_PAD,
    y: Math.max(EDGE_PAD, window.innerHeight - BTN - 24),
  };
}

function clampPos(pos) {
  const maxX = Math.max(EDGE_PAD, window.innerWidth - BTN - EDGE_PAD);
  const maxY = Math.max(EDGE_PAD, window.innerHeight - BTN - EDGE_PAD);
  return {
    x: clamp(pos.x, EDGE_PAD, maxX),
    y: clamp(pos.y, EDGE_PAD, maxY),
  };
}

function readPos() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultPos();
    const parsed = JSON.parse(raw);
    if (
      parsed &&
      typeof parsed.x === 'number' &&
      typeof parsed.y === 'number'
    ) {
      return clampPos(parsed);
    }
  } catch {
    // ignore
  }
  return defaultPos();
}

const FloatingFriendLinks = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const links = useMemo(() => {
    const status = statusState?.status;
    if (!status || status.friend_links_enabled === false) return [];
    const list = Array.isArray(status.friend_links) ? status.friend_links : [];
    return list
      .filter((item) => item && item.enabled !== false && item.name && item.url)
      .slice()
      .sort((a, b) => (a.order || 0) - (b.order || 0));
  }, [statusState]);

  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState(() =>
    typeof window === 'undefined' ? { x: EDGE_PAD, y: 120 } : readPos(),
  );
  const dragRef = useRef(null);

  useEffect(() => {
    const onResize = () => setPos((p) => clampPos(p));
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  if (!links.length) return null;

  const panelRight = pos.x + BTN / 2 > window.innerWidth / 2;

  const onPointerDown = (e) => {
    e.currentTarget.setPointerCapture(e.pointerId);
    dragRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      startY: e.clientY,
      origX: pos.x,
      origY: pos.y,
      dragged: false,
    };
  };

  const onPointerMove = (e) => {
    const d = dragRef.current;
    if (!d || d.pointerId !== e.pointerId) return;
    const dx = e.clientX - d.startX;
    const dy = e.clientY - d.startY;
    if (!d.dragged && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
    d.dragged = true;
    setPos(clampPos({ x: d.origX + dx, y: d.origY + dy }));
  };

  const onPointerUp = (e) => {
    const d = dragRef.current;
    if (!d || d.pointerId !== e.pointerId) return;
    try {
      e.currentTarget.releasePointerCapture(e.pointerId);
    } catch {
      // ignore
    }
    if (d.dragged) {
      setPos((p) => {
        const next = clampPos(p);
        try {
          localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
        } catch {
          // ignore
        }
        return next;
      });
    } else {
      setOpen((v) => !v);
    }
    dragRef.current = null;
  };

  return (
    <div
      style={{
        position: 'fixed',
        left: pos.x,
        top: pos.y,
        width: BTN,
        height: BTN,
        zIndex: 1000,
      }}
    >
      {open && (
        <div
          style={{
            position: 'absolute',
            bottom: BTN + 10,
            [panelRight ? 'right' : 'left']: 0,
            width: 260,
            borderRadius: 16,
            border: '1px solid var(--semi-color-border)',
            background: 'var(--semi-color-bg-1)',
            boxShadow: '0 12px 40px rgba(0,0,0,.18)',
            overflow: 'hidden',
          }}
        >
          <div style={{ padding: '10px 12px', fontWeight: 700 }}>
            {t('友情链接')}
          </div>
          <div style={{ maxHeight: 280, overflow: 'auto', padding: 8 }}>
            {links.map((link) => (
              <a
                key={`${link.name}-${link.url}`}
                href={link.url}
                target='_blank'
                rel='noreferrer noopener'
                style={{
                  display: 'flex',
                  gap: 10,
                  alignItems: 'center',
                  padding: '8px 10px',
                  borderRadius: 12,
                  textDecoration: 'none',
                  color: 'inherit',
                }}
              >
                {link.icon ? (
                  <img
                    src={link.icon}
                    alt=''
                    style={{ width: 32, height: 32, borderRadius: 8 }}
                  />
                ) : (
                  <div
                    style={{
                      width: 32,
                      height: 32,
                      borderRadius: 8,
                      display: 'grid',
                      placeItems: 'center',
                      background: 'var(--semi-color-primary-light-default)',
                      color: 'var(--semi-color-primary)',
                      fontWeight: 800,
                    }}
                  >
                    {String(link.name).slice(0, 1).toUpperCase()}
                  </div>
                )}
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: 13 }}>{link.name}</div>
                  {link.description ? (
                    <div
                      style={{
                        fontSize: 11,
                        color: 'var(--semi-color-text-2)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {link.description}
                    </div>
                  ) : null}
                </div>
              </a>
            ))}
          </div>
        </div>
      )}
      <button
        type='button'
        aria-label={t('友情链接')}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        style={{
          width: BTN,
          height: BTN,
          borderRadius: 999,
          border: 'none',
          cursor: 'grab',
          color: '#fff',
          fontWeight: 800,
          background: 'linear-gradient(135deg,#0ea5e9,#4f46e5)',
          boxShadow: '0 10px 24px rgba(37,99,235,.35)',
          touchAction: 'none',
        }}
      >
        链
      </button>
    </div>
  );
};

export default FloatingFriendLinks;
