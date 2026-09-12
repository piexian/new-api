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

const groupSelector = '[data-slot="input-group"]'
const controlSelector =
  'input[data-slot="input-group-control"], textarea[data-slot="input-group-control"]'
const interactiveSelector =
  'button, a[href], input, textarea, select, label, summary, [tabindex], [contenteditable]:not([contenteditable="false"]), [role="button"], [role="link"], [role="combobox"], [role="textbox"], [role="menuitem"], [role="option"]'

export function getInputGroupFocusTarget(
  addon: HTMLElement,
  target: Node
): HTMLInputElement | HTMLTextAreaElement | undefined {
  // React portal events bubble through the component tree, not the DOM tree.
  if (!addon.contains(target)) {
    return
  }

  const group = addon.closest(groupSelector)
  const element =
    target.nodeType === 1 ? (target as Element) : target.parentElement
  const interactiveElement = element?.closest(interactiveSelector)
  if (
    !group ||
    !element ||
    element.closest(groupSelector) !== group ||
    (interactiveElement && addon.contains(interactiveElement))
  ) {
    return
  }

  return [
    ...group.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>(
      controlSelector
    ),
  ].find(
    (control) =>
      control.closest(groupSelector) === group &&
      control.type !== 'hidden' &&
      !control.matches(':disabled') &&
      !control.closest('[hidden], [inert], [aria-hidden="true"]')
  )
}
