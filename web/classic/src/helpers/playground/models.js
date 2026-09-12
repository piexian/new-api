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

const CHAT_ENDPOINTS = new Set([
  'openai',
  'openai-response',
  'anthropic',
  'gemini',
  'cohere-chat',
]);

export function normalizeModelOptions(data) {
  if (!Array.isArray(data)) return [];
  return data.flatMap((model) => {
    const id = typeof model === 'string' ? model : model?.id;
    if (typeof id !== 'string') return [];
    return [
      {
        label: id,
        value: id,
        ...(Array.isArray(model?.supported_endpoint_types)
          ? { supportedEndpointTypes: model.supported_endpoint_types }
          : {}),
      },
    ];
  });
}

export function filterChatModels(models, inputs, catalog = {}) {
  const nativeEndpoint =
    inputs.webSearchEnabled ||
    (inputs.codeInterpreterEnabled && inputs.chatInterface !== 'openai')
      ? inputs.chatInterface
      : undefined;
  return models.filter((model) => {
    const endpoints = model.supportedEndpointTypes;
    if (endpoints?.length) {
      return (
        endpoints.some((endpoint) => CHAT_ENDPOINTS.has(endpoint)) &&
        (!nativeEndpoint || endpoints.includes(nativeEndpoint))
      );
    }
    const entry = catalog[model.value] || catalog[model.label];
    const modalities = [
      ...(entry?.modalities?.input ?? []),
      ...(entry?.modalities?.output ?? []),
    ];
    return !modalities.length || modalities.includes('text');
  });
}
