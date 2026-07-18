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
import fs from 'node:fs/promises'
import path from 'node:path'

const OBFUSCATED_KEYS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

function usage() {
  console.error(
    'Usage: apply-i18n-patch.mjs <locales-dir> <locale-csv> <patch.json>'
  )
}

function isPlainObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function stableStringify(value) {
  let text = JSON.stringify(value, null, 2)
  for (const key of OBFUSCATED_KEYS) {
    text = text.replaceAll(`"${key.runtime}":`, `"${key.serialized}":`)
  }
  return `${text}\n`
}

function assertSameKeys(referenceKeys, translations, locale) {
  const keys = Object.keys(translations)
  const referenceSet = new Set(referenceKeys)
  const keySet = new Set(keys)
  const missing = referenceKeys.filter((key) => !keySet.has(key))
  const extras = keys.filter((key) => !referenceSet.has(key))
  if (missing.length > 0 || extras.length > 0) {
    throw new Error(
      `${locale} patch keys differ (missing: ${missing.join(', ') || 'none'}; extra: ${extras.join(', ') || 'none'})`
    )
  }
}

async function main() {
  const [localesDirArg, localeCsv, patchFileArg] = process.argv.slice(2)
  if (!localesDirArg || !localeCsv || !patchFileArg) {
    usage()
    process.exitCode = 1
    return
  }

  const locales = localeCsv
    .split(',')
    .map((locale) => locale.trim())
    .filter(Boolean)
  const localesDir = path.resolve(localesDirArg)
  const patchFile = path.resolve(patchFileArg)
  const patch = JSON.parse(await fs.readFile(patchFile, 'utf8'))

  if (!isPlainObject(patch) || locales.length === 0) {
    throw new Error('The patch must be an object keyed by locale.')
  }

  const unexpectedLocales = Object.keys(patch).filter(
    (locale) => !locales.includes(locale)
  )
  const missingLocales = locales.filter(
    (locale) => !Object.hasOwn(patch, locale)
  )
  if (missingLocales.length > 0 || unexpectedLocales.length > 0) {
    throw new Error(
      `Locale coverage differs (missing: ${missingLocales.join(', ') || 'none'}; unexpected: ${unexpectedLocales.join(', ') || 'none'})`
    )
  }

  const referenceLocale = locales[0]
  const referenceTranslations = patch[referenceLocale]
  if (!isPlainObject(referenceTranslations)) {
    throw new Error(`${referenceLocale} translations must be an object.`)
  }
  const referenceKeys = Object.keys(referenceTranslations)
  if (referenceKeys.length === 0) {
    throw new Error('The patch does not contain any translation keys.')
  }

  for (const locale of locales) {
    const translations = patch[locale]
    if (!isPlainObject(translations)) {
      throw new Error(`${locale} translations must be an object.`)
    }
    assertSameKeys(referenceKeys, translations, locale)
    for (const [key, value] of Object.entries(translations)) {
      if (typeof value !== 'string') {
        throw new Error(`${locale}.${key} must be a string.`)
      }
    }
  }

  const localeDocuments = new Map()
  for (const locale of locales) {
    const filePath = path.join(localesDir, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))
    if (!isPlainObject(json.translation)) {
      throw new Error(`${filePath} does not contain a translation object.`)
    }
    localeDocuments.set(locale, { filePath, json })
  }

  let totalApplied = 0
  for (const locale of locales) {
    const { filePath, json } = localeDocuments.get(locale)
    let applied = 0
    for (const [key, value] of Object.entries(patch[locale])) {
      if (json.translation[key] !== value) {
        json.translation[key] = value
        applied++
      }
    }

    if (applied > 0) {
      await fs.writeFile(filePath, stableStringify(json), 'utf8')
    }
    console.log(`${locale}: ${applied} translations applied`)
    totalApplied += applied
  }

  console.log(`Total: ${totalApplied} translations applied`)
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
