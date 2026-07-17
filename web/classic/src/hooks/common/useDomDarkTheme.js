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

import { useEffect, useState } from 'react';

function readDomDark() {
  return (
    document.documentElement.classList.contains('dark') ||
    document.body.getAttribute('theme-mode') === 'dark'
  );
}

/**
 * 以 DOM 主题标记（html.dark / body[theme-mode]）为准的亮暗状态。
 * 相比 Theme context：DOM 由 Provider 的副作用维护，始终为真值；
 * 监听其变化可保证消费组件在主题切换时可靠重渲染（无需刷新页面）。
 */
export function useDomDarkTheme() {
  const [dark, setDark] = useState(readDomDark);

  useEffect(() => {
    const update = () => setDark(readDomDark());
    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    });
    observer.observe(document.body, {
      attributes: true,
      attributeFilter: ['theme-mode'],
    });
    update();
    return () => observer.disconnect();
  }, []);

  return dark;
}
