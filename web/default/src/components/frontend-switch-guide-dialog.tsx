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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  FRONTEND_RETURN_TIP_DISMISSED_KEY,
  FRONTEND_RETURN_TIP_PENDING_KEY,
} from '@/lib/constants'
import { switchToClassicFrontend } from '@/lib/frontend-theme'

function clearPendingTip() {
  try {
    window.localStorage.removeItem(FRONTEND_RETURN_TIP_PENDING_KEY)
  } catch {
    /* empty */
  }
}

function setDismissed() {
  try {
    window.localStorage.setItem(FRONTEND_RETURN_TIP_DISMISSED_KEY, '1')
  } catch {
    /* empty */
  }
}

export function FrontendSwitchGuideDialog() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [dontShowAgain, setDontShowAgain] = useState(false)

  useEffect(() => {
    try {
      if (window.localStorage.getItem(FRONTEND_RETURN_TIP_DISMISSED_KEY)) {
        clearPendingTip()
        return
      }
      if (window.localStorage.getItem(FRONTEND_RETURN_TIP_PENDING_KEY)) {
        setOpen(true)
      }
    } catch {
      /* empty */
    }
  }, [])

  const handleClose = () => {
    if (dontShowAgain) {
      setDismissed()
    }
    clearPendingTip()
    setOpen(false)
  }

  const handleSwitchToClassic = () => {
    clearPendingTip()
    switchToClassicFrontend()
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) handleClose()
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t('You are using the modern frontend')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'To switch back to the classic frontend, use the button below or open the user menu in the top-right corner and choose Switch to classic frontend.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className='flex items-center gap-2 px-1'>
          <Checkbox
            id='dont-show-again'
            checked={dontShowAgain}
            onCheckedChange={(checked) => setDontShowAgain(Boolean(checked))}
          />
          <label
            htmlFor='dont-show-again'
            className='text-muted-foreground cursor-pointer text-sm leading-none font-medium'
          >
            {t("Don't show this again")}
          </label>
        </div>
        <AlertDialogFooter>
          <Button variant='outline' onClick={handleSwitchToClassic}>
            {t('Switch to classic frontend')}
          </Button>
          <Button onClick={handleClose}>{t('Confirm')}</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
