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
import { GlobeIcon, PaperclipIcon, TerminalIcon, Trash2Icon, ZapIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInputButton,
  PromptInputTools,
} from '@/components/ai-elements/prompt-input'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { CHAT_INTERFACE_OPTIONS } from '../../constants'
import {
  ATTACHMENT_ACTIONS,
  getAttachmentActionNotice,
} from '../../lib'
import type { ParameterEnabled, PlaygroundConfig } from '../../types'
import { PlaygroundParameterPanel } from './playground-parameter-panel'


type PlaygroundInputToolsProps = {
  config: PlaygroundConfig
  disabled?: boolean
  hasMessages?: boolean
  onClearMessages?: () => void
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

export function PlaygroundInputTools({
  config,
  disabled,
  hasMessages = false,
  onClearMessages,
  onConfigChange,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputToolsProps) {
  const { t } = useTranslation()
  const [clearConfirmOpen, setClearConfirmOpen] = useState(false)

  const handleFileAction = (action: string) => {
    const notice = getAttachmentActionNotice(action)
    toast.info(t(notice.title), {
      description: notice.description,
    })
  }

  const handleClearMessages = () => {
    onClearMessages?.()
    setClearConfirmOpen(false)
    toast.success(t('Conversation cleared'))
  }

  return (
    <>
      <PromptInputTools className='bg-background/70 border-border/60 rounded-lg border p-1 shadow-xs'>
        <Tooltip>
          <DropdownMenu>
            <TooltipTrigger
              render={
                <DropdownMenuTrigger
                  render={
                    <PromptInputButton
                      aria-label={t('Attach')}
                      className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
                      disabled={disabled}
                      variant='ghost'
                    />
                  }
                >
                  <PaperclipIcon size={16} />
                </DropdownMenuTrigger>
              }
            />
            <TooltipContent>
              <p>{t('Attach')}</p>
            </TooltipContent>
            <DropdownMenuContent align='start'>
              {ATTACHMENT_ACTIONS.map(({ action, icon: Icon, label }) => (
                <DropdownMenuItem
                  key={action}
                  onClick={() => handleFileAction(action)}
                >
                  <Icon className='mr-2' size={16} />
                  {t(label)}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </Tooltip>

        {/* Web Search 开关 */}
        <Tooltip>
          <TooltipTrigger
            render={
              <PromptInputButton
                aria-label={t('Web Search')}
                className={cn(
                  'font-medium transition-colors',
                  config.webSearchEnabled
                    ? 'text-foreground bg-muted/70'
                    : 'text-muted-foreground hover:text-foreground hover:bg-muted/70'
                )}
                disabled={disabled}
                onClick={() => onConfigChange('webSearchEnabled', !config.webSearchEnabled)}
                variant='ghost'
              >
                <GlobeIcon size={16} />
              </PromptInputButton>
            }
          />
          <TooltipContent>
            <p>{t('Web Search')}</p>
          </TooltipContent>
        </Tooltip>

        {/* Code Interpreter 开关 */}
        <Tooltip>
          <TooltipTrigger
            render={
              <PromptInputButton
                aria-label={t('Code Interpreter')}
                className={cn(
                  'font-medium transition-colors',
                  config.codeInterpreterEnabled
                    ? 'text-foreground bg-muted/70'
                    : 'text-muted-foreground hover:text-foreground hover:bg-muted/70'
                )}
                disabled={disabled}
                onClick={() => onConfigChange('codeInterpreterEnabled', !config.codeInterpreterEnabled)}
                variant='ghost'
              >
                <TerminalIcon size={16} />
              </PromptInputButton>
            }
          />
          <TooltipContent>
            <p>{t('Code Interpreter')}</p>
          </TooltipContent>
        </Tooltip>

        {/* Reasoning 思考等级 */}
        <Select
          value={config.reasoningEffort}
          onValueChange={(v) => v && onConfigChange('reasoningEffort', v as PlaygroundConfig['reasoningEffort'])}
        >
          <Tooltip>
            <TooltipTrigger
              render={
                <SelectTrigger className='h-8 w-[36px] justify-center px-0' aria-label={t('Reasoning')}>
                  <ZapIcon size={16} className={cn(config.reasoningEffort !== 'none' && 'text-foreground')} />
                </SelectTrigger>
              }
            />
            <TooltipContent>
              <p>{t('Reasoning Effort')}</p>
            </TooltipContent>
          </Tooltip>
          <SelectContent>
            <SelectItem value='none'>{t('Off')}</SelectItem>
            <SelectItem value='low'>{t('Low')}</SelectItem>
            <SelectItem value='medium'>{t('Medium')}</SelectItem>
            <SelectItem value='high'>{t('High')}</SelectItem>
            <SelectItem value='max'>{t('Max')}</SelectItem>
          </SelectContent>
        </Select>

        <Select
          value={config.chatInterface}
          onValueChange={(v) => v && onConfigChange('chatInterface', v as PlaygroundConfig['chatInterface'])}
        >
          <SelectTrigger className='h-8 w-[130px] text-xs'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CHAT_INTERFACE_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.labelKey}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <PlaygroundParameterPanel
          config={config}
          disabled={disabled}
          onConfigChange={onConfigChange}
          onParameterEnabledChange={onParameterEnabledChange}
          parameterEnabled={parameterEnabled}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <PromptInputButton
                aria-label={t('Clear chat history')}
                className='text-muted-foreground hover:text-destructive hover:bg-destructive/10 font-medium'
                disabled={disabled || !hasMessages || !onClearMessages}
                onClick={() => setClearConfirmOpen(true)}
                variant='ghost'
              >
                <Trash2Icon size={16} />
              </PromptInputButton>
            }
          />
          <TooltipContent>
            <p>{t('Clear chat history')}</p>
          </TooltipContent>
        </Tooltip>
      </PromptInputTools>

      <ConfirmDialog
        destructive
        desc={t(
          'All playground messages saved in this browser will be removed. This cannot be undone.'
        )}
        confirmText={t('Clear')}
        handleConfirm={handleClearMessages}
        open={clearConfirmOpen}
        onOpenChange={setClearConfirmOpen}
        title={t('Clear chat history?')}
      />
    </>
  )
}
