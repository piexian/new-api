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

import React, { useEffect, useMemo, useState } from 'react';
import { Button, TabPane, Tabs, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { copy, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const PROTOCOL = [
  {
    key: 'chat',
    label: 'Chat',
    method: 'POST',
    path: '/v1/chat/completions',
    body: '"model": "your-model"\n"messages": [{ "role": "user", "content": "..." }]',
  },
  {
    key: 'responses',
    label: 'Responses',
    method: 'POST',
    path: '/v1/responses',
    body: '"model": "your-model"\n"input": "..."',
  },
  {
    key: 'claude',
    label: 'Claude',
    method: 'POST',
    path: '/v1/messages',
    body: '"model": "your-model"\n"max_tokens": 1024\n"messages": [{ "role": "user", "content": "..." }]',
  },
  {
    key: 'gemini',
    label: 'Gemini',
    method: 'POST',
    path: '/v1beta/models/{model}:generateContent',
    body: '"contents": [{ "role": "user", "parts": [{ "text": "..." }] }]',
  },
];

function normalizeBase(url) {
  return String(url || '').replace(/\/+$/, '');
}

export default function HomeCapabilityTabs({ serverAddress }) {
  const { t } = useTranslation();
  const base = normalizeBase(serverAddress || window.location.origin);
  const openaiBase = `${base}/v1`;
  const [active, setActive] = useState('chat');

  const codexConfig = useMemo(
    () => `[model_providers.OpenAI]
name = "OpenAI"
base_url = "${openaiBase}"
wire_api = "responses"
requires_openai_auth = true`,
    [openaiBase],
  );
  const codexAuth = `{
  "auth_mode": "apikey",
  "OPENAI_API_KEY": "sk-your-new-api-token"
}`;
  const claudeSettings = useMemo(
    () => `{
  "env": {
    "ANTHROPIC_BASE_URL": "${base}",
    "ANTHROPIC_AUTH_TOKEN": "sk-your-new-api-token",
    "ANTHROPIC_MODEL": "your-gateway-model",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "your-gateway-model",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "your-gateway-model",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "your-gateway-model",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
    "CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING": "1",
    "API_TIMEOUT_MS": "600000"
  }
}`,
    [base],
  );

  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return undefined;
    if (!PROTOCOL.some((p) => p.key === active)) return undefined;
    const timer = window.setInterval(() => {
      setActive((prev) => {
        const idx = PROTOCOL.findIndex((p) => p.key === prev);
        if (idx < 0) return prev;
        return PROTOCOL[(idx + 1) % PROTOCOL.length].key;
      });
    }, 4500);
    return () => window.clearInterval(timer);
  }, [active]);

  const onCopy = async (text) => {
    const ok = await copy(text);
    if (ok) showSuccess(t('已复制到剪切板'));
    else showError(t('复制失败'));
  };

  const preStyle = {
    margin: 0,
    padding: 14,
    borderRadius: 12,
    background: 'var(--semi-color-fill-0)',
    border: '1px solid var(--semi-color-border)',
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    fontSize: 12.5,
    lineHeight: 1.55,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
  };

  return (
    <div
      className='rounded-2xl border border-semi-color-border bg-semi-color-bg-1 overflow-hidden'
      style={{ boxShadow: '0 20px 50px -25px rgba(15,23,42,.18)' }}
    >
      <Tabs
        type='line'
        activeKey={active}
        onChange={(key) => setActive(key)}
        collapsible
      >
        {PROTOCOL.map((item) => (
          <TabPane tab={item.label} itemKey={item.key} key={item.key}>
            <div style={{ padding: 16 }}>
              <div style={{ marginBottom: 10, fontFamily: 'monospace', fontSize: 13 }}>
                <strong style={{ marginRight: 8 }}>{item.method}</strong>
                {item.path}
              </div>
              <pre style={preStyle}>{item.body}</pre>
            </div>
          </TabPane>
        ))}
        <TabPane tab='Codex' itemKey='codex'>
          <div style={{ padding: 16, display: 'grid', gap: 12 }}>
            <pre style={preStyle}>{codexConfig}</pre>
            <Button onClick={() => onCopy(codexConfig)}>{t('复制 config')}</Button>
            <pre style={preStyle}>{codexAuth}</pre>
            <Button onClick={() => onCopy(codexAuth)}>{t('复制 auth')}</Button>
            <Text type='tertiary' size='small'>
              {t('写入 ~/.codex/config.toml 与 ~/.codex/auth.json；wire_api 必须为 responses')}
            </Text>
          </div>
        </TabPane>
        <TabPane tab='Claude Code' itemKey='claude-code'>
          <div style={{ padding: 16, display: 'grid', gap: 12 }}>
            <pre style={preStyle}>{claudeSettings}</pre>
            <Button onClick={() => onCopy(claudeSettings)}>{t('复制')}</Button>
            <Text type='tertiary' size='small'>
              {t('写入 ~/.claude/settings.json；模型 ID 使用网关实际名称')}
            </Text>
          </div>
        </TabPane>
      </Tabs>
    </div>
  );
}
