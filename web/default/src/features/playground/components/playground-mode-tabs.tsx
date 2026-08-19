import { useTranslation } from 'react-i18next'

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { PLAYGROUND_MODES } from '../constants'
import type { PlaygroundMode } from '../types'

interface PlaygroundModeTabsProps {
  mode: PlaygroundMode
  onModeChange: (mode: PlaygroundMode) => void
}

export function PlaygroundModeTabs({ mode, onModeChange }: PlaygroundModeTabsProps) {
  const { t } = useTranslation()
  return (
    <Tabs value={mode} onValueChange={(v) => onModeChange(v as PlaygroundMode)}>
      <TabsList>
        {PLAYGROUND_MODES.map((m: { mode: PlaygroundMode; labelKey: string }) => (
          <TabsTrigger key={m.mode} value={m.mode}>
            {t(m.labelKey)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}
