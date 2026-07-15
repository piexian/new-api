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
import { AiMagicIcon, Copy01Icon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useDebounce } from "@/hooks";
import { copyToClipboard } from "@/lib/copy-to-clipboard";

import {
  getEmailTemplate,
  getEmailTemplateCatalog,
  previewEmailTemplate,
  restoreEmailTemplate,
  updateEmailTemplate,
} from "../api";
import { SettingsPageFormActions } from "../components/settings-page-context";
import { SettingsSection } from "../components/settings-section";
import type { EmailTemplatePreviewRequest } from "../types";

const DEFAULT_EVENT = "auth.verify_code";
const DEFAULT_LOCALE = "zh-CN";

const EVENT_LABELS: Record<string, string> = {
  "auth.verify_code": "Email verification code",
  "auth.password_reset": "Password reset",
  "balance.low": "Low balance reminder",
  "subscription.balance_low": "Low subscription balance reminder",
  "channel.auto_disabled": "Channel automatically disabled",
  "channel.auto_enabled": "Channel automatically restored",
  "channel.quota_cooldown": "Channel quota cooldown",
  "channel.test_result": "Channel test completed",
  "channel.model_updates": "Upstream model inspection",
  "notification.general": "General notification",
  "system.test": "Test email",
};

const LOCALE_LABELS: Record<string, string> = {
  "zh-CN": "简体中文",
  "zh-TW": "繁體中文",
  en: "English",
};

function buildAITemplatePrompt(
  event: string,
  locale: string,
  placeholders: string[],
) {
  const tokens = placeholders
    .map((placeholder) => `{{ ${placeholder} }}`)
    .join(", ");
  return `You are a senior transactional-email designer. Help me create a polished email template for the event "${event}" in locale "${locale}".

Before generating the template, ask me one concise question about the desired brand style, tone, colors, and call to action. After I answer, return only one valid JSON object with this exact shape:
{"subject":"...","html":"..."}

Requirements:
- The subject must be no longer than 200 characters and must not contain line breaks.
- The HTML must be a complete responsive email document using table-based layout and inline CSS.
- Use a restrained, professional visual style with accessible contrast and mobile-friendly spacing.
- Do not use JavaScript, forms, external fonts, external stylesheets, embedded images, or remote tracking assets.
- Use {{ logo_url }} as the only remote image and keep the layout usable when it is unavailable.
- Only use these placeholders, preserving their exact syntax: ${tokens}.
- Do not invent any other placeholders.
- Escape normal HTML text correctly, but keep placeholders unchanged.
- When a *_url placeholder is available, use it as the href of a clear action button.
- Keep the HTML under 50,000 characters.
- Do not wrap the JSON in Markdown code fences.`;
}

function ensureSuccess<T>(response: {
  success: boolean;
  message: string;
  data: T;
}): T {
  if (!response.success) {
    throw new Error(response.message);
  }
  return response.data;
}

export function EmailTemplateSettingsSection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [event, setEvent] = useState(DEFAULT_EVENT);
  const [locale, setLocale] = useState(DEFAULT_LOCALE);
  const [subject, setSubject] = useState("");
  const [html, setHTML] = useState("");

  const catalogQuery = useQuery({
    queryKey: ["email-template-catalog"],
    queryFn: async () => ensureSuccess(await getEmailTemplateCatalog()),
    staleTime: 5 * 60 * 1000,
  });

  const templateQueryKey = useMemo(
    () => ["email-template", event, locale] as const,
    [event, locale],
  );
  const templateQuery = useQuery({
    queryKey: templateQueryKey,
    queryFn: async () => ensureSuccess(await getEmailTemplate(event, locale)),
  });
  const template = templateQuery.data;

  useEffect(() => {
    if (!templateQuery.data) return;
    setSubject(templateQuery.data.subject);
    setHTML(templateQuery.data.html);
  }, [templateQuery.data]);

  const previewRequest = useMemo<EmailTemplatePreviewRequest>(
    () => ({ event, locale, subject, html }),
    [event, locale, subject, html],
  );
  const debouncedPreviewRequest = useDebounce(previewRequest, 450);
  const canPreview = Boolean(
    template &&
    template.event === event &&
    template.locale === locale &&
    debouncedPreviewRequest.event === event &&
    debouncedPreviewRequest.locale === locale &&
    debouncedPreviewRequest.subject.trim() &&
    debouncedPreviewRequest.html.trim(),
  );
  const previewQuery = useQuery({
    queryKey: ["email-template-preview", debouncedPreviewRequest],
    queryFn: async () =>
      ensureSuccess(await previewEmailTemplate(debouncedPreviewRequest)),
    enabled: canPreview,
    retry: false,
    staleTime: Infinity,
    gcTime: 30 * 1000,
  });

  const saveMutation = useMutation({
    mutationFn: async () =>
      ensureSuccess(
        await updateEmailTemplate(event, locale, { subject, html }),
      ),
    onSuccess: (savedTemplate) => {
      queryClient.setQueryData(templateQueryKey, savedTemplate);
      setSubject(savedTemplate.subject);
      setHTML(savedTemplate.html);
      toast.success(t("Email template saved"));
    },
    onError: (error: Error) => {
      toast.error(error.message || t("Failed to save email template"));
    },
  });

  const restoreMutation = useMutation({
    mutationFn: async () =>
      ensureSuccess(await restoreEmailTemplate(event, locale)),
    onSuccess: (restoredTemplate) => {
      queryClient.setQueryData(templateQueryKey, restoredTemplate);
      setSubject(restoredTemplate.subject);
      setHTML(restoredTemplate.html);
      toast.success(t("Default email template restored"));
    },
    onError: (error: Error) => {
      toast.error(error.message || t("Failed to restore email template"));
    },
  });

  const isDirty = Boolean(
    template && (subject !== template.subject || html !== template.html),
  );
  const isMutating = saveMutation.isPending || restoreMutation.isPending;
  const preview = canPreview ? previewQuery.data : undefined;
  const isPreviewing =
    Boolean(subject.trim() && html.trim()) &&
    (!canPreview ||
      previewRequest !== debouncedPreviewRequest ||
      previewQuery.isFetching);

  const copyPlaceholder = async (placeholder: string) => {
    const token = `{{ ${placeholder} }}`;
    if (await copyToClipboard(token)) {
      toast.success(t("Placeholder copied"));
    } else {
      toast.error(t("Failed to copy placeholder"));
    }
  };

  const copyAITemplatePrompt = async () => {
    if (!template) return;
    const prompt = buildAITemplatePrompt(event, locale, template.placeholders);
    if (await copyToClipboard(prompt)) {
      toast.success(t("AI template prompt copied"));
    } else {
      toast.error(t("Failed to copy AI template prompt"));
    }
  };

  return (
    <SettingsSection
      title={t("Email Templates")}
      className="h-full min-h-0 w-full"
    >
      <SettingsPageFormActions
        onSave={() => saveMutation.mutate()}
        onReset={
          template?.is_custom ? () => restoreMutation.mutate() : undefined
        }
        isSaving={isMutating}
        isSaveDisabled={!isDirty || !subject.trim() || !html.trim()}
        isResetDisabled={isDirty}
        saveLabel="Save email template"
        resetLabel="Restore default"
        resetVariant="destructive"
      />

      <div className="grid gap-5 md:grid-cols-2">
        <div className="space-y-2">
          <Label>{t("Notification event")}</Label>
          <Select
            value={event}
            onValueChange={(value) => value && setEvent(value)}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(catalogQuery.data?.events ?? []).map((item) => (
                <SelectItem key={item.event} value={item.event}>
                  {t(EVENT_LABELS[item.event] ?? item.event)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <Label>{t("Template language")}</Label>
          <Select
            value={locale}
            onValueChange={(value) => value && setLocale(value)}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(catalogQuery.data?.locales ?? []).map((item) => (
                <SelectItem key={item} value={item}>
                  {LOCALE_LABELS[item] ?? item}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {templateQuery.isLoading ? (
        <div className="text-muted-foreground flex min-h-64 items-center justify-center gap-2 text-sm">
          <Spinner />
          {t("Loading email template...")}
        </div>
      ) : null}
      {!templateQuery.isLoading && templateQuery.isError ? (
        <div className="border-destructive/40 text-destructive rounded-lg border px-4 py-3 text-sm">
          {templateQuery.error.message}
        </div>
      ) : null}
      {!templateQuery.isLoading && !templateQuery.isError && template ? (
        <div className="flex min-h-0 w-full flex-1 flex-col gap-5">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={template.is_custom ? "default" : "outline"}>
              {template.is_custom
                ? t("Custom template")
                : t("Built-in template")}
            </Badge>
            {isDirty ? (
              <Badge variant="secondary">{t("Unsaved changes")}</Badge>
            ) : null}
          </div>

          <div className="grid w-full min-w-0 gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] xl:items-start">
            <div className="min-w-0 space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email-template-subject">
                  {t("Email subject")}
                </Label>
                <Input
                  id="email-template-subject"
                  value={subject}
                  maxLength={200}
                  onChange={(changeEvent) =>
                    setSubject(changeEvent.target.value)
                  }
                />
              </div>

              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <Label htmlFor="email-template-html">{t("HTML")}</Label>
                  <span className="text-muted-foreground text-xs tabular-nums">
                    {html.length.toLocaleString()} / 50,000
                  </span>
                </div>
                <textarea
                  id="email-template-html"
                  aria-label={t("Email HTML content")}
                  className="border-input placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:bg-input/30 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 min-h-[28rem] w-full resize-y overflow-auto rounded-lg border bg-transparent px-3 py-2 font-mono text-sm leading-6 transition-colors outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-3"
                  value={html}
                  rows={18}
                  maxLength={50000}
                  spellCheck={false}
                  onChange={(changeEvent) => setHTML(changeEvent.target.value)}
                />
              </div>

              <div className="space-y-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <Label>{t("Available placeholders")}</Label>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={copyAITemplatePrompt}
                  >
                    <HugeiconsIcon
                      icon={AiMagicIcon}
                      strokeWidth={2}
                      data-icon="inline-start"
                    />
                    {t("Copy AI template prompt")}
                  </Button>
                </div>
                <TooltipProvider>
                  <div className="flex flex-wrap gap-2">
                    {template.placeholders.map((placeholder) => (
                      <Tooltip key={placeholder}>
                        <TooltipTrigger
                          render={
                            <Button
                              type="button"
                              size="xs"
                              variant="outline"
                              onClick={() => copyPlaceholder(placeholder)}
                            />
                          }
                        >
                          <HugeiconsIcon
                            icon={Copy01Icon}
                            strokeWidth={2}
                            data-icon="inline-start"
                          />
                          {`{{ ${placeholder} }}`}
                        </TooltipTrigger>
                        <TooltipContent>{t("Copy placeholder")}</TooltipContent>
                      </Tooltip>
                    ))}
                  </div>
                </TooltipProvider>
              </div>
            </div>

            <div className="min-w-0 overflow-hidden rounded-lg border">
              <div className="bg-muted/30 flex min-h-16 items-center justify-between gap-3 border-b px-4 py-3">
                <div className="min-w-0">
                  <div className="text-sm font-medium">{t("Live preview")}</div>
                  <div
                    className="text-muted-foreground truncate text-xs"
                    title={preview?.subject ?? subject}
                  >
                    {preview?.subject ?? subject}
                  </div>
                </div>
                {isPreviewing ? <Spinner className="shrink-0" /> : null}
              </div>
              <div
                className="bg-muted/20 relative p-3"
                aria-busy={isPreviewing}
              >
                {preview ? (
                  <iframe
                    title={t("Email template preview")}
                    className="h-[36rem] w-full rounded-md border bg-white"
                    sandbox=""
                    srcDoc={preview.html}
                  />
                ) : null}
                {!preview && isPreviewing ? (
                  <div className="text-muted-foreground flex h-[36rem] items-center justify-center gap-2 text-sm">
                    <Spinner />
                    {t("Rendering preview...")}
                  </div>
                ) : null}
                {!preview && !isPreviewing && previewQuery.error ? (
                  <div className="text-destructive flex h-[36rem] items-center justify-center px-6 text-center text-sm">
                    {previewQuery.error.message ||
                      t("Failed to preview email template")}
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </SettingsSection>
  );
}
