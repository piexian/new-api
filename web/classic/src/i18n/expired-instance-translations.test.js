import assert from 'node:assert/strict';
import { test } from 'node:test';
import i18next from 'i18next';

import en from './locales/en.json';
import fr from './locales/fr.json';
import ja from './locales/ja.json';
import ru from './locales/ru.json';
import vi from './locales/vi.json';
import zhCN from './locales/zh-CN.json';
import zhTW from './locales/zh-TW.json';

test('expired-instance deletion feedback renders for every locale and plural form', async () => {
  const resources = { en, fr, ja, ru, vi, 'zh-CN': zhCN, 'zh-TW': zhTW };
  const i18n = i18next.createInstance();
  await i18n.init({ resources, fallbackLng: 'zh-CN', nsSeparator: false });
  const key = '已删除 {{count}} 个过期实例';

  for (const lng of Object.keys(resources)) {
    for (const count of [0, 1, 2, 5, 21]) {
      const result = i18n.t(key, { lng, count });
      assert.ok(result.length > 0, `${lng}/${count} must not be blank`);
      assert.ok(
        result.includes(String(count)),
        `${lng}/${count} must interpolate count`,
      );
      assert.ok(
        !result.includes('{{'),
        `${lng}/${count} must resolve placeholders`,
      );
      if (lng !== 'zh-CN') {
        assert.notEqual(result, i18n.t(key, { lng: 'zh-CN', count }));
      }
    }
  }
  assert.equal(
    i18n.t(key, { lng: 'en', count: 1 }),
    'Deleted 1 stale instance',
  );
  assert.equal(
    i18n.t(key, { lng: 'en', count: 2 }),
    'Deleted 2 stale instances',
  );
});
