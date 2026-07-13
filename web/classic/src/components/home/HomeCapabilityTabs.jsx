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
import { Button, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { copy, showError, showSuccess } from '../../helpers';
import { useActualTheme } from '../../context/Theme';

const { Text } = Typography;

const PROTOCOL = [
  {
    key: 'chat',
    label: 'Chat',
    method: 'POST',
    path: '/v1/chat/completions',
    body: '"model": "your-model",\n"messages": [\n  { "role": "user", "content": "..." }\n]',
    accent: '#059669',
  },
  {
    key: 'responses',
    label: 'Responses',
    method: 'POST',
    path: '/v1/responses',
    body: '"model": "your-model",\n"input": "..."',
    accent: '#d97706',
  },
  {
    key: 'claude',
    label: 'Claude',
    method: 'POST',
    path: '/v1/messages',
    body: '"model": "your-model",\n"max_tokens": 1024,\n"messages": [\n  { "role": "user", "content": "..." }\n]',
    accent: '#2563eb',
  },
  {
    key: 'gemini',
    label: 'Gemini',
    method: 'POST',
    path: '/v1beta/models/{model}:generateContent',
    body: '"contents": [\n  { "role": "user",\n    "parts": [{ "text": "..." }] }\n]',
    accent: '#7c3aed',
  },
];

function normalizeBase(url) {
  return String(url || '').replace(/\/+$/, '');
}

function isDarkTheme(theme) {
  return (
    theme === 'dark' || document.body.getAttribute('theme-mode') === 'dark'
  );
}

export default function HomeCapabilityTabs({ serverAddress }) {
  const { t } = useTranslation();
  const actualTheme = useActualTheme();
  const dark = isDarkTheme(actualTheme);
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
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      return undefined;
    }
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

  const cardStyle = {
    overflow: 'hidden',
    borderRadius: 16,
    border: dark
      ? '1px solid rgba(255,255,255,0.06)'
      : '1px solid rgba(15,23,42,0.12)',
    background: dark ? 'rgba(11,15,23,0.95)' : 'rgba(255,255,255,0.95)',
    boxShadow: dark
      ? '0 20px 60px -25px rgba(0,0,0,0.7)'
      : '0 20px 50px -25px rgba(15,23,42,0.18)',
    backdropFilter: 'blur(10px)',
    textAlign: 'left',
  };

  const tabRowStyle = {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    padding: '0 8px',
    borderBottom: dark
      ? '1px solid rgba(255,255,255,0.05)'
      : '1px solid rgba(15,23,42,0.08)',
    overflowX: 'auto',
  };

  const tabs = [
    ...PROTOCOL.map((p) => ({ key: p.key, label: p.label, kind: 'protocol' })),
    { key: 'sep', label: '|', kind: 'sep' },
    { key: 'codex', label: 'Codex', kind: 'setup' },
    { key: 'claude-code', label: 'Claude Code', kind: 'setup' },
  ];

  const activeProtocol = PROTOCOL.find((p) => p.key === active);
  const preStyle = {
    margin: 0,
    padding: 14,
    borderRadius: 12,
    background: dark ? 'rgba(255,255,255,0.03)' : 'rgba(248,250,252,0.92)',
    border: dark
      ? '1px solid rgba(255,255,255,0.06)'
      : '1px solid rgba(15,23,42,0.12)',
    color: dark ? 'rgba(238,243,255,0.88)' : '#0f172a',
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    fontSize: 12.5,
    lineHeight: 1.55,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
  };

  return (
    <div style={cardStyle}>
      <div style={tabRowStyle}>
        {tabs.map((item) => {
          if (item.kind === 'sep') {
            return (
              <span
                key='sep'
                style={{
                  width: 1,
                  height: 16,
                  margin: '0 6px',
                  background: dark
                    ? 'rgba(255,255,255,0.1)'
                    : 'rgba(15,23,42,0.12)',
                  flexShrink: 0,
                }}
              />
            );
          }
          const on = item.key === active;
          const accent =
            PROTOCOL.find((p) => p.key === item.key)?.accent ||
            (dark ? '#a78bfa' : '#2563eb');
          return (
            <button
              key={item.key}
              type='button'
              onClick={() => setActive(item.key)}
              style={{
                border: 'none',
                background: 'transparent',
                color: on
                  ? accent
                  : dark
                    ? 'rgba(238,243,255,0.4)'
                    : 'rgba(15,23,42,0.4)',
                fontWeight: 600,
                fontSize: 11,
                padding: '12px 10px',
                borderBottom: on
                  ? `2px solid ${accent}`
                  : '2px solid transparent',
                marginBottom: -1,
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              {item.label}
            </button>
          );
        })}
        <div
          style={{
            marginLeft: 'auto',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            paddingRight: 8,
            fontFamily:
              'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 10,
            color: dark ? 'rgba(238,243,255,0.4)' : 'rgba(15,23,42,0.45)',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            flexShrink: 0,
          }}
        >
          <span
            style={{
              width: 6,
              height: 6,
              borderRadius: 999,
              background: '#22c55e',
              boxShadow: '0 0 8px rgba(34,197,94,0.45)',
            }}
          />
          {activeProtocol ? '200 ok' : 'setup'}
        </div>
      </div>

      {activeProtocol ? (
        <>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 10,
              padding: '12px 20px',
              borderBottom: dark
                ? '1px solid rgba(255,255,255,0.04)'
                : '1px solid rgba(15,23,42,0.08)',
              fontFamily:
                'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
              fontSize: 12.5,
            }}
          >
            <span
              style={{
                fontSize: 10,
                fontWeight: 700,
                letterSpacing: '0.06em',
                padding: '3px 7px',
                borderRadius: 6,
                background: `${activeProtocol.accent}22`,
                color: activeProtocol.accent,
              }}
            >
              {activeProtocol.method}
            </span>
            <code style={{ opacity: 0.85 }}>{activeProtocol.path}</code>
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateRows: '1fr 1fr',
              minHeight: 280,
              fontFamily:
                'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
              fontSize: 12.5,
              lineHeight: 1.55,
            }}
          >
            <div style={{ padding: '16px 20px' }}>
              <div
                style={{
                  fontSize: 10,
                  fontWeight: 700,
                  letterSpacing: '0.08em',
                  textTransform: 'uppercase',
                  opacity: 0.45,
                  marginBottom: 10,
                }}
              >
                Request
              </div>
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
                {activeProtocol.body}
              </pre>
            </div>
            <div
              style={{
                padding: '16px 20px',
                borderTop: dark
                  ? '1px solid rgba(255,255,255,0.04)'
                  : '1px solid rgba(15,23,42,0.08)',
              }}
            >
              <div
                style={{
                  fontSize: 10,
                  fontWeight: 700,
                  letterSpacing: '0.08em',
                  textTransform: 'uppercase',
                  opacity: 0.45,
                  marginBottom: 10,
                }}
              >
                Response
              </div>
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', opacity: 0.85 }}>
                {`{\n  "ok": true\n}`}
              </pre>
            </div>
          </div>
        </>
      ) : null}

      {active === 'codex' ? (
        <div style={{ padding: '16px 20px', display: 'grid', gap: 12 }}>
          <pre style={preStyle}>{codexConfig}</pre>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button size='small' onClick={() => onCopy(codexConfig)}>
              {t('复制 config')}
            </Button>
          </div>
          <pre style={preStyle}>{codexAuth}</pre>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button size='small' onClick={() => onCopy(codexAuth)}>
              {t('复制 auth')}
            </Button>
          </div>
          <Text type='tertiary' size='small'>
            {t(
              '写入 ~/.codex/config.toml 与 ~/.codex/auth.json；wire_api 必须为 responses',
            )}
          </Text>
        </div>
      ) : null}

      {active === 'claude-code' ? (
        <div style={{ padding: '16px 20px', display: 'grid', gap: 12 }}>
          <pre style={preStyle}>{claudeSettings}</pre>
          <div>
            <Button size='small' onClick={() => onCopy(claudeSettings)}>
              {t('复制')}
            </Button>
          </div>
          <Text type='tertiary' size='small'>
            {t('写入 ~/.claude/settings.json；模型 ID 使用网关实际名称')}
          </Text>
        </div>
      ) : null}
    </div>
  );
}
