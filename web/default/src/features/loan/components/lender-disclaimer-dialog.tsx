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

import { agreeLenderDisclaimer } from '../api'

interface LenderDisclaimerDialogProps {
  open: boolean
  onAgreed: () => void
  onCancel: () => void
}

/**
 * 放贷免责声明弹窗：首次创建挂单前必须确认（未同意时后端会拒绝创建）。
 * 与词元贷 18+ 声明弹窗同构：勾选确认后调用 disclaimer 接口，成功后由父组件关闭。
 */
export function LenderDisclaimerDialog(props: LenderDisclaimerDialogProps) {
  const { t } = useTranslation()
  const [confirmed, setConfirmed] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const handleAgree = async () => {
    setSubmitting(true)
    try {
      const res = await agreeLenderDisclaimer()
      if (res.success) {
        toast.success(t('Lender disclaimer agreed'))
        setConfirmed(false)
        props.onAgreed()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Failed to agree to lender disclaimer'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) props.onCancel()
      }}
      showCloseButton={false}
      title={t('Lender Disclaimer')}
      description={t(
        'Please read and agree to the following disclaimer before offering loans.'
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
      <div className='bg-muted/30 max-h-64 space-y-2 overflow-y-auto rounded-lg border p-3 text-sm'>
        <p>
          {t('Lending in the token loan market is purely for entertainment.')}
        </p>
        <p>
          {t(
            'It is not real finance. Lent quota may be lost entirely if the borrower defaults.'
          )}
        </p>
        <p>
          {t(
            'The platform does not guarantee repayment or pursue debts on behalf of lenders.'
          )}
        </p>
      </div>
      <label className='flex cursor-pointer items-start gap-2 text-sm'>
        <Checkbox
          checked={confirmed}
          onCheckedChange={(checked) => setConfirmed(checked === true)}
          className='mt-0.5'
        />
        <span>
          {t(
            'I have read and agree to the disclaimer above. I understand the risks and accept them voluntarily.'
          )}
        </span>
      </label>
    </Dialog>
  )
}
