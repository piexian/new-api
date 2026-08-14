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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'

import { agreeLoanTerms } from '../api'

interface TermsDialogProps {
  open: boolean
  termsText: string
  onAgreed: () => void
}

/**
 * Forced, non-dismissible terms dialog: shown when terms are enabled but not
 * yet agreed. The user must confirm the 18+ declaration before agreeing.
 */
export function TermsDialog(props: TermsDialogProps) {
  const { t } = useTranslation()
  const [confirmed, setConfirmed] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const handleAgree = async () => {
    setSubmitting(true)
    try {
      const res = await agreeLoanTerms()
      if (res.success) {
        toast.success(t('Terms agreed'))
        props.onAgreed()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Failed to agree to terms'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      // 不可跳过：仅在同意成功后由父组件关闭
      onOpenChange={() => {}}
      showCloseButton={false}
      title={t('Token Loan Terms')}
      description={t(
        'Please read and agree to the following terms before using the token loan feature.'
      )}
      contentClassName='sm:max-w-lg'
      bodyClassName='space-y-4'
      footer={
        <Button
          onClick={handleAgree}
          disabled={!confirmed || submitting}
          className='w-full sm:w-auto'
        >
          {submitting ? t('Submitting...') : t('I agree')}
        </Button>
      }
    >
      <div className='bg-muted/30 max-h-64 overflow-y-auto rounded-lg border p-3 text-sm break-words whitespace-pre-wrap'>
        {props.termsText}
      </div>
      <label className='flex cursor-pointer items-start gap-2 text-sm'>
        <Checkbox
          checked={confirmed}
          onCheckedChange={(checked) => setConfirmed(checked === true)}
          className='mt-0.5'
        />
        <span>
          {t(
            'I confirm that I am 18 years of age or older and I have read and agree to the terms above.'
          )}
        </span>
      </label>
    </Dialog>
  )
}
