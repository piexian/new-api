import assert from 'node:assert/strict'
import { test } from 'node:test'

import { createInstance } from 'i18next'

import { resources } from './config'

test('all supported languages resolve playground modes and interface labels', async () => {
  const i18n = createInstance()
  await i18n.init({
    resources,
    lng: 'zhCN',
    fallbackLng: false,
    keySeparator: false,
  })
  const keys = [
    'playground.mode.chat',
    'playground.mode.image',
    'playground.mode.video',
    'playground.mode.audio',
    'Image Generation',
    'Image Edit',
    'Text to Speech',
    'Audio Transcription',
  ]
  for (const language of Object.keys(resources)) {
    await i18n.changeLanguage(language)
    for (const key of keys) {
      assert.equal(i18n.exists(key), true, `${language}: ${key}`)
    }
    assert.notEqual(i18n.t('playground.mode.chat'), 'playground.mode.chat')
  }
  await i18n.changeLanguage('zhCN')
  assert.equal(i18n.t('playground.mode.chat'), '对话')
  assert.equal(i18n.t('Image Generation'), '图片生成')
})
