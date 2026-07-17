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

export function buildAITemplatePrompt(
  event: string,
  locale: string,
  placeholders: string[]
) {
  const tokens = placeholders
    .map((placeholder) => `{{ ${placeholder} }}`)
    .join(', ')

  return `You are a senior transactional-email designer. Help me create a polished email template for the event "${event}" in locale "${locale}".

Before generating the template, ask me one concise question about the desired brand style, tone, colors, and call to action. After I answer, return only the complete HTML document that I can paste directly into an HTML field.

Requirements:
- Output HTML only. Do not return JSON, a subject line, explanations, or Markdown code fences.
- The response must start with <!doctype html> or <html and end with </html>.
- The HTML must be a complete responsive email document using table-based layout and inline CSS.
- Use a restrained, professional visual style with accessible contrast and mobile-friendly spacing.
- Do not use JavaScript, forms, external fonts, external stylesheets, embedded images, or remote tracking assets.
- If {{ logo_url }} is available in the placeholder list, use it as the only remote image and keep the layout usable when it is unavailable. Otherwise, do not use remote images.
- Only use these placeholders, preserving their exact syntax: ${tokens}.
- Do not invent any other placeholders.
- Escape normal HTML text correctly, but keep placeholders unchanged.
- When a *_url placeholder is available, use it as the href of a clear action button.
- Keep the HTML under 50,000 characters.`
}

export function resolveEmailPreviewLink(
  rawHref: string,
  baseHref: string
): string | null {
  try {
    const url = new URL(rawHref.trim(), baseHref)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
    return url.href
  } catch {
    return null
  }
}
