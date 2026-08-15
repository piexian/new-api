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
import { RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

/**
 * 查询失败态：展示后端 i18n message（queryFn throw 的 Error.message）+ 重试按钮
 */
export function QueryErrorState({
  message,
  onRetry,
}: {
  message?: string
  onRetry: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex min-h-32 flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-4 text-center'>
      <p className='text-muted-foreground text-sm break-words'>
        {message || t('Request failed')}
      </p>
      <Button variant='outline' size='sm' onClick={onRetry}>
        <RotateCcw className='h-4 w-4' />
        {t('Retry')}
      </Button>
    </div>
  )
}