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
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const SCRIPT_PATH = fileURLToPath(
  new URL('./apply-i18n-patch.mjs', import.meta.url)
)

async function createFixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'i18n-patch-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const localesDir = path.join(root, 'locales')
  await fs.mkdir(localesDir)
  return { root, localesDir }
}

test('updates values in place and appends new keys without sorting', async (t) => {
  const { root, localesDir } = await createFixture(t)
  const original = {
    translation: {
      Zebra: 'Zebra',
      Alpha: 'Old',
      'footer.newapi.projectAttributionSuffix': 'Project',
    },
  }
  for (const locale of ['en', 'zh']) {
    await fs.writeFile(
      path.join(localesDir, `${locale}.json`),
      `${JSON.stringify(original, null, 2)}\n`
    )
  }

  const patch = {
    en: { Alpha: 'Updated', Beta: 'Beta' },
    zh: { Alpha: '已更新', Beta: '测试' },
  }
  const patchFile = path.join(root, 'patch.json')
  await fs.writeFile(patchFile, JSON.stringify(patch))

  execFileSync(
    process.execPath,
    [SCRIPT_PATH, localesDir, 'en,zh', patchFile],
    { cwd: root }
  )

  const raw = await fs.readFile(path.join(localesDir, 'en.json'), 'utf8')
  const parsed = JSON.parse(raw)
  assert.deepEqual(Object.keys(parsed.translation), [
    'Zebra',
    'Alpha',
    'footer.newapi.projectAttributionSuffix',
    'Beta',
  ])
  assert.equal(parsed.translation.Alpha, 'Updated')
  assert.match(raw, /footer\.new\\u0061pi\.projectAttributionSuffix/)
})

test('rejects incomplete locale coverage before writing files', async (t) => {
  const { root, localesDir } = await createFixture(t)
  const localeFile = path.join(localesDir, 'en.json')
  const original = '{\n  "translation": {\n    "Alpha": "Alpha"\n  }\n}\n'
  await fs.writeFile(localeFile, original)
  await fs.writeFile(
    path.join(root, 'patch.json'),
    JSON.stringify({ en: { Beta: 'Beta' } })
  )

  const result = spawnSync(
    process.execPath,
    [SCRIPT_PATH, localesDir, 'en,zh', path.join(root, 'patch.json')],
    { cwd: root, encoding: 'utf8' }
  )

  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /Locale coverage differs/)
  assert.equal(await fs.readFile(localeFile, 'utf8'), original)
})

test('validates every locale file before writing any changes', async (t) => {
  const { root, localesDir } = await createFixture(t)
  const enFile = path.join(localesDir, 'en.json')
  const original = '{\n  "translation": {\n    "Alpha": "Alpha"\n  }\n}\n'
  await fs.writeFile(enFile, original)
  await fs.writeFile(path.join(localesDir, 'zh.json'), '{}')

  const patchFile = path.join(root, 'patch.json')
  await fs.writeFile(
    patchFile,
    JSON.stringify({
      en: { Beta: 'Beta' },
      zh: { Beta: '测试' },
    })
  )

  const result = spawnSync(
    process.execPath,
    [SCRIPT_PATH, localesDir, 'en,zh', patchFile],
    { cwd: root, encoding: 'utf8' }
  )

  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /does not contain a translation object/)
  assert.equal(await fs.readFile(enFile, 'utf8'), original)
})
