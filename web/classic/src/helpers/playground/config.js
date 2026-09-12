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

import { DEFAULT_CONFIG } from '../../constants/playground.constants';
import { OPTIONAL_PARAMETERS, normalizeOptionalNumber } from './parameters';

const isObject = (value) =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

export function unwrapConfig(value) {
  return isObject(value) && 'version' in value && 'data' in value
    ? value.data
    : value;
}

export function isConfig(value) {
  const config = unwrapConfig(value);
  return (
    isObject(config) &&
    (isObject(config.inputs) ||
      Object.keys(DEFAULT_CONFIG.inputs).some((key) => key in config))
  );
}

// Both themes used the same key before isolation. Preserve explicit false/zero.
export function normalizeConfig(value, legacyParameterEnabled) {
  const config = unwrapConfig(value);
  const source = isObject(config) ? config : {};
  const legacyClassic = isObject(source.inputs);
  const storedInputs = legacyClassic ? source.inputs : source;
  const inputs = { ...DEFAULT_CONFIG.inputs };
  for (const key of Object.keys(inputs)) {
    const value = storedInputs[key];
    if (OPTIONAL_PARAMETERS.includes(key)) {
      inputs[key] = normalizeOptionalNumber(value);
    } else if (Array.isArray(inputs[key])) {
      if (
        Array.isArray(value) &&
        value.every((item) => typeof item === 'string')
      ) {
        inputs[key] = [...value];
      }
    } else if (typeof value === typeof inputs[key]) {
      inputs[key] = value;
    }
  }
  for (const key of ['webSearchEnabled', 'codeInterpreterEnabled']) {
    inputs[key] =
      typeof storedInputs[key] === 'boolean'
        ? storedInputs[key]
        : storedInputs.toolsEnabled === true;
  }
  const storedEnabled = unwrapConfig(
    source.parameterEnabled ?? legacyParameterEnabled,
  );
  const parameterEnabled = { ...DEFAULT_CONFIG.parameterEnabled };
  for (const key of OPTIONAL_PARAMETERS) {
    if (typeof storedEnabled?.[key] === 'boolean') {
      parameterEnabled[key] = storedEnabled[key];
    } else if (isConfig(value) && key in storedInputs) {
      // Missing historical toggles followed the old theme defaults.
      parameterEnabled[key] = [
        'temperature',
        'top_p',
        'frequency_penalty',
        'presence_penalty',
      ].includes(key);
    }
  }
  const normalized = { ...DEFAULT_CONFIG, inputs, parameterEnabled };
  for (const key of ['showDebugPanel', 'customRequestMode']) {
    if (typeof source[key] === 'boolean') normalized[key] = source[key];
  }
  for (const key of ['customRequestBody', 'systemPrompt']) {
    if (typeof source[key] === 'string') normalized[key] = source[key];
  }
  if (Array.isArray(source.messages)) normalized.messages = source.messages;
  return normalized;
}
