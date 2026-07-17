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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { IconCopy, IconCode, IconSave, IconUndo } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const DEFAULT_EVENT = 'auth.verify_code';
const DEFAULT_LOCALE = 'zh-CN';

const EVENT_LABELS = {
  'auth.verify_code': 'Email verification code',
  'auth.password_reset': 'Password reset',
  'balance.low': 'Low balance reminder',
  'subscription.balance_low': 'Low subscription balance reminder',
  'channel.auto_disabled': 'Channel automatically disabled',
  'channel.auto_enabled': 'Channel automatically restored',
  'channel.quota_cooldown': 'Channel quota cooldown',
  'channel.test_result': 'Channel test completed',
  'channel.model_updates': 'Upstream model inspection',
  'notification.general': 'General notification',
  'system.test': 'Test email',
};

const LOCALE_LABELS = {
  'zh-CN': '简体中文',
  'zh-TW': '繁體中文',
  en: 'English',
};

const buildAITemplatePrompt = (event, locale, placeholders) => {
  const tokens = placeholders
    .map((placeholder) => `{{ ${placeholder} }}`)
    .join(', ');
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
- Keep the HTML under 50,000 characters.`;
};

const resolveEmailPreviewLink = (rawHref, baseHref) => {
  try {
    const url = new URL(rawHref.trim(), baseHref);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
    return url.href;
  } catch {
    return null;
  }
};

const EmailTemplateSetting = () => {
  const { t } = useTranslation();
  const [catalog, setCatalog] = useState({ events: [], locales: [] });
  const [event, setEvent] = useState(DEFAULT_EVENT);
  const [locale, setLocale] = useState(DEFAULT_LOCALE);
  const [template, setTemplate] = useState(null);
  const [subject, setSubject] = useState('');
  const [html, setHtml] = useState('');
  const [previewHtml, setPreviewHtml] = useState('');
  const [previewSubject, setPreviewSubject] = useState('');
  const [previewError, setPreviewError] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const previewLinkCleanupRef = useRef(null);

  useEffect(
    () => () => {
      previewLinkCleanupRef.current?.();
    },
    [],
  );

  useEffect(() => {
    const loadCatalog = async () => {
      try {
        const response = await API.get('/api/option/email_templates');
        if (!response.data.success) {
          throw new Error(response.data.message);
        }
        setCatalog(response.data.data);
      } catch (error) {
        showError(error.message || t('Failed to load email templates'));
      }
    };
    loadCatalog();
  }, []);

  useEffect(() => {
    let cancelled = false;
    const loadTemplate = async () => {
      setLoading(true);
      setTemplate(null);
      setPreviewHtml('');
      setPreviewSubject('');
      setPreviewError('');
      try {
        const response = await API.get(
          `/api/option/email_templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}`,
        );
        if (!response.data.success) {
          throw new Error(response.data.message);
        }
        if (cancelled) return;
        const nextTemplate = response.data.data;
        setTemplate(nextTemplate);
        setSubject(nextTemplate.subject);
        setHtml(nextTemplate.html);
      } catch (error) {
        if (cancelled) return;
        showError(error.message || t('Failed to load email template'));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    loadTemplate();
    return () => {
      cancelled = true;
    };
  }, [event, locale]);

  useEffect(() => {
    let cancelled = false;
    let timeout;
    const templateMatchesSelection =
      template?.event === event && template?.locale === locale;

    if (!templateMatchesSelection || !subject.trim() || !html.trim()) {
      setPreviewHtml('');
      setPreviewSubject('');
      setPreviewError('');
      setPreviewing(false);
      return () => {
        cancelled = true;
      };
    }

    setPreviewing(true);
    timeout = window.setTimeout(async () => {
      try {
        const response = await API.post('/api/option/email_templates/preview', {
          event,
          locale,
          subject,
          html,
        });
        if (!response.data.success) {
          throw new Error(response.data.message);
        }
        if (cancelled) return;
        setPreviewHtml(response.data.data.html);
        setPreviewSubject(response.data.data.subject);
        setPreviewError('');
      } catch (error) {
        if (cancelled) return;
        setPreviewHtml('');
        setPreviewSubject('');
        setPreviewError(error.message || t('Failed to preview email template'));
      } finally {
        if (!cancelled) setPreviewing(false);
      }
    }, 450);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [event, locale, subject, html, template, t]);

  const isDirty = useMemo(
    () =>
      Boolean(
        template && (subject !== template.subject || html !== template.html),
      ),
    [template, subject, html],
  );

  const saveTemplate = async () => {
    setSaving(true);
    try {
      const response = await API.put(
        `/api/option/email_templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}`,
        { subject, html },
      );
      if (!response.data.success) {
        throw new Error(response.data.message);
      }
      setTemplate(response.data.data);
      setSubject(response.data.data.subject);
      setHtml(response.data.data.html);
      showSuccess(t('Email template saved'));
    } catch (error) {
      showError(error.message || t('Failed to save email template'));
    } finally {
      setSaving(false);
    }
  };

  const restoreTemplate = async () => {
    setSaving(true);
    try {
      const response = await API.delete(
        `/api/option/email_templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}`,
      );
      if (!response.data.success) {
        throw new Error(response.data.message);
      }
      setTemplate(response.data.data);
      setSubject(response.data.data.subject);
      setHtml(response.data.data.html);
      showSuccess(t('Default email template restored'));
    } catch (error) {
      showError(error.message || t('Failed to restore email template'));
    } finally {
      setSaving(false);
    }
  };

  const copyPlaceholder = async (placeholder) => {
    if (await copy(`{{ ${placeholder} }}`)) {
      showSuccess(t('Placeholder copied'));
    } else {
      showError(t('Failed to copy placeholder'));
    }
  };

  const copyAITemplatePrompt = async () => {
    if (!template) return;
    const prompt = buildAITemplatePrompt(event, locale, template.placeholders);
    if (await copy(prompt)) {
      showSuccess(t('AI template prompt copied'));
    } else {
      showError(t('Failed to copy AI template prompt'));
    }
  };

  const handlePreviewFrameLoad = (loadEvent) => {
    previewLinkCleanupRef.current?.();
    previewLinkCleanupRef.current = null;

    const frameDocument = loadEvent.currentTarget.contentDocument;
    if (!frameDocument) return;

    const handlePreviewClick = (clickEvent) => {
      const targetNode = clickEvent.target;
      const targetElement =
        targetNode?.nodeType === 1 ? targetNode : targetNode?.parentElement;
      const anchor = targetElement?.closest?.('a[href]');
      const rawHref = anchor?.getAttribute('href')?.trim();
      if (!rawHref || rawHref.startsWith('#')) return;

      clickEvent.preventDefault();
      clickEvent.stopPropagation();

      const resolvedLink = resolveEmailPreviewLink(
        rawHref,
        frameDocument.baseURI,
      );
      if (!resolvedLink) {
        showError(t('This link cannot be opened from the preview.'));
        return;
      }

      Modal.confirm({
        title: t('Open external link?'),
        content: (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <Text>
              {t(
                'This link will open in a new window. Do you want to continue?',
              )}
            </Text>
            <Text type='secondary' style={{ wordBreak: 'break-all' }}>
              {resolvedLink}
            </Text>
          </div>
        ),
        okText: t('Open'),
        cancelText: t('Cancel'),
        onOk: () => {
          window.open(resolvedLink, '_blank', 'noopener,noreferrer');
        },
      });
    };

    frameDocument.addEventListener('click', handlePreviewClick);
    previewLinkCleanupRef.current = () => {
      frameDocument.removeEventListener('click', handlePreviewClick);
    };
  };

  return (
    <Card>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
        <div>
          <Title heading={5} style={{ margin: 0 }}>
            {t('Email Templates')}
          </Title>
          <Text type='secondary'>
            {t('Customize notification emails by event and language')}
          </Text>
        </div>

        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Text strong>{t('Notification event')}</Text>
            <Select
              value={event}
              onChange={setEvent}
              style={{ width: '100%', marginTop: 8 }}
            >
              {catalog.events.map((item) => (
                <Select.Option key={item.event} value={item.event}>
                  {t(EVENT_LABELS[item.event] || item.event)}
                </Select.Option>
              ))}
            </Select>
          </Col>
          <Col xs={24} sm={12}>
            <Text strong>{t('Template language')}</Text>
            <Select
              value={locale}
              onChange={setLocale}
              style={{ width: '100%', marginTop: 8 }}
            >
              {catalog.locales.map((item) => (
                <Select.Option key={item} value={item}>
                  {LOCALE_LABELS[item] || item}
                </Select.Option>
              ))}
            </Select>
          </Col>
        </Row>

        <Spin spinning={loading}>
          {template ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <Space>
                <Tag color={template.is_custom ? 'blue' : 'grey'}>
                  {template.is_custom
                    ? t('Custom template')
                    : t('Built-in template')}
                </Tag>
                {isDirty ? (
                  <Tag color='orange'>{t('Unsaved changes')}</Tag>
                ) : null}
              </Space>

              <Row gutter={24}>
                <Col xs={24} xl={12} style={{ paddingBottom: 16 }}>
                  <div
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 16,
                      minWidth: 0,
                    }}
                  >
                    <div>
                      <Text strong>{t('Email subject')}</Text>
                      <Input
                        value={subject}
                        maxLength={200}
                        onChange={setSubject}
                        style={{ marginTop: 8 }}
                      />
                    </div>

                    <div>
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 12,
                        }}
                      >
                        <Text strong>HTML</Text>
                        <Text type='tertiary' size='small'>
                          {html.length.toLocaleString()} / 50,000
                        </Text>
                      </div>
                      <textarea
                        id='classic-email-template-html'
                        aria-label='HTML'
                        value={html}
                        onChange={(changeEvent) =>
                          setHtml(changeEvent.target.value)
                        }
                        rows={18}
                        maxLength={50000}
                        spellCheck={false}
                        style={{
                          boxSizing: 'border-box',
                          width: '100%',
                          minHeight: 448,
                          marginTop: 8,
                          resize: 'vertical',
                          padding: '8px 12px',
                          border: '1px solid var(--semi-color-border)',
                          borderRadius: 6,
                          background: 'var(--semi-color-fill-0)',
                          color: 'var(--semi-color-text-0)',
                          outline: 'none',
                          fontFamily: 'monospace',
                          fontSize: 14,
                          lineHeight: '24px',
                        }}
                      />
                    </div>

                    <div>
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          flexWrap: 'wrap',
                          gap: 8,
                        }}
                      >
                        <Text strong>{t('Available placeholders')}</Text>
                        <Button
                          size='small'
                          icon={<IconCode />}
                          onClick={copyAITemplatePrompt}
                        >
                          {t('Copy AI template prompt')}
                        </Button>
                      </div>
                      <Space wrap style={{ marginTop: 8 }}>
                        {template.placeholders.map((placeholder) => (
                          <Tooltip
                            key={placeholder}
                            content={t('Copy placeholder')}
                          >
                            <Button
                              size='small'
                              theme='borderless'
                              icon={<IconCopy />}
                              onClick={() => copyPlaceholder(placeholder)}
                            >
                              {`{{ ${placeholder} }}`}
                            </Button>
                          </Tooltip>
                        ))}
                      </Space>
                    </div>
                  </div>
                </Col>

                <Col xs={24} xl={12}>
                  <div
                    style={{
                      border: '1px solid var(--semi-color-border)',
                      borderRadius: 6,
                      overflow: 'hidden',
                    }}
                  >
                    <div
                      style={{
                        minHeight: 64,
                        padding: '12px 16px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 12,
                        borderBottom: '1px solid var(--semi-color-border)',
                        background: 'var(--semi-color-fill-0)',
                      }}
                    >
                      <div style={{ minWidth: 0 }}>
                        <Text strong>{t('Live preview')}</Text>
                        <div
                          title={previewSubject || subject}
                          style={{
                            marginTop: 3,
                            color: 'var(--semi-color-text-2)',
                            fontSize: 12,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {previewSubject || subject}
                        </div>
                      </div>
                      {previewing ? <Spin size='small' /> : null}
                    </div>
                    <div
                      aria-busy={previewing}
                      style={{
                        padding: 12,
                        background: 'var(--semi-color-fill-0)',
                      }}
                    >
                      {previewHtml ? (
                        <iframe
                          title={t('Email template preview')}
                          sandbox='allow-same-origin'
                          srcDoc={previewHtml}
                          onLoad={handlePreviewFrameLoad}
                          style={{
                            width: '100%',
                            height: 576,
                            border: '1px solid var(--semi-color-border)',
                            borderRadius: 6,
                          }}
                        />
                      ) : previewing ? (
                        <div
                          style={{
                            height: 576,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                          }}
                        >
                          <Spin />
                        </div>
                      ) : previewError ? (
                        <div
                          style={{
                            height: 576,
                            padding: 24,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            color: 'var(--semi-color-danger)',
                            textAlign: 'center',
                          }}
                        >
                          {previewError}
                        </div>
                      ) : null}
                    </div>
                  </div>
                </Col>
              </Row>

              <Space wrap>
                <Button
                  theme='solid'
                  type='primary'
                  icon={<IconSave />}
                  loading={saving}
                  disabled={!isDirty || !subject.trim() || !html.trim()}
                  onClick={saveTemplate}
                >
                  {t('Save email template')}
                </Button>
                <Button
                  icon={<IconUndo />}
                  disabled={!template.is_custom || isDirty || saving}
                  onClick={restoreTemplate}
                >
                  {t('Restore default')}
                </Button>
              </Space>
            </div>
          ) : null}
        </Spin>
      </div>
    </Card>
  );
};

export default EmailTemplateSetting;
