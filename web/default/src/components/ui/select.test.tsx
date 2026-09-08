import assert from 'node:assert/strict'
import { test } from 'node:test'

import { renderToStaticMarkup } from 'react-dom/server'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './select'

test('closed selects render translated labels on every fresh mount', () => {
  for (const label of ['图片生成', 'Image Generation', '图片生成']) {
    const html = renderToStaticMarkup(
      <Select defaultValue='generations'>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectItem value='generations'>{label}</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
    )
    assert.match(html, new RegExp(label))
  }
})

test('explicit items and custom value renderers retain precedence', () => {
  const html = renderToStaticMarkup(
    <Select value='a' items={{ a: 'Explicit label' }}>
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value='a'>Inline label</SelectItem>
      </SelectContent>
    </Select>
  )
  assert.match(html, /Explicit label/)
  const custom = renderToStaticMarkup(
    <Select value='a'>
      <SelectTrigger>
        <SelectValue>{() => 'Custom label'}</SelectValue>
      </SelectTrigger>
    </Select>
  )
  assert.match(custom, /Custom label/)
})

test('empty selections show the placeholder before the menu is opened', () => {
  for (const value of [null, '']) {
    const html = renderToStaticMarkup(
      <Select value={value}>
        <SelectTrigger>
          <SelectValue placeholder='请选择' />
        </SelectTrigger>
      </Select>
    )
    assert.match(html, /请选择/)
  }
})
