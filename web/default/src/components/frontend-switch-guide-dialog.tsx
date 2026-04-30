import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FRONTEND_RETURN_TIP_PENDING_KEY } from '@/lib/constants'
import { switchToClassicFrontend } from '@/lib/frontend-theme'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

function clearPendingTip() {
  try {
    window.localStorage.removeItem(FRONTEND_RETURN_TIP_PENDING_KEY)
  } catch {
    /* empty */
  }
}

export function FrontendSwitchGuideDialog() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  useEffect(() => {
    try {
      if (window.localStorage.getItem(FRONTEND_RETURN_TIP_PENDING_KEY)) {
        setOpen(true)
      }
    } catch {
      /* empty */
    }
  }, [])

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) clearPendingTip()
  }

  const handleSwitchToClassic = () => {
    clearPendingTip()
    switchToClassicFrontend()
  }

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
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
        <AlertDialogFooter>
          <Button variant='outline' onClick={handleSwitchToClassic}>
            {t('Switch to classic frontend')}
          </Button>
          <AlertDialogAction onClick={clearPendingTip}>
            {t('Confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
