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
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { User } from '../types'
import { UserTokensPanel } from './user-tokens-panel'

interface UserTokensDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: Pick<User, 'id' | 'username'>
}

export function UserTokensDialog({
  open,
  onOpenChange,
  user,
}: UserTokensDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] gap-0 overflow-hidden p-0 sm:max-w-2xl'>
        <DialogHeader className='border-b px-4 py-3'>
          <DialogTitle>{t('Manage Tokens')}</DialogTitle>
          <DialogDescription>
            {user.username} (ID: {user.id})
          </DialogDescription>
        </DialogHeader>
        <div className='max-h-[calc(90vh-4rem)] overflow-y-auto p-3'>
          <UserTokensPanel user={user} />
        </div>
      </DialogContent>
    </Dialog>
  )
}
