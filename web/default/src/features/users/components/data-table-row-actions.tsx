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
import type { Row } from '@tanstack/react-table'
import {
  Pencil,
  Trash2,
  Power,
  PowerOff,
  ArrowUp,
  ArrowDown,
  KeyRound,
  ShieldAlert,
  Link2,
  CreditCard,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { UserSubscriptionsDialog } from '@/features/subscriptions/components/dialogs/user-subscriptions-dialog'

import { manageUser, resetUserPasskey, resetUserTwoFA } from '../api'
import {
  USER_STATUS,
  USER_ROLE,
  ERROR_MESSAGES,
  isUserDeleted,
} from '../constants'
import { DISABLE_REASON_MAX_LENGTH, getUserActionMessage } from '../lib'
import type { User, ManageUserAction } from '../types'
import { UserBindingDialog } from './dialogs/user-binding-dialog'
import { UserTokensDialog } from './user-tokens-dialog'
import { useUsers } from './users-provider'

interface DataTableRowActionsProps {
  row: Row<User>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const user = row.original
  const { setOpen, setCurrentRow, triggerRefresh } = useUsers()
  const [resetPasskeyOpen, setResetPasskeyOpen] = useState(false)
  const [resetTwoFAOpen, setResetTwoFAOpen] = useState(false)
  const [bindingDialogOpen, setBindingDialogOpen] = useState(false)
  const [subscriptionsDialogOpen, setSubscriptionsDialogOpen] = useState(false)
  const [tokensDialogOpen, setTokensDialogOpen] = useState(false)
  const [disableDialogOpen, setDisableDialogOpen] = useState(false)
  const [disableReason, setDisableReason] = useState(user.disable_reason || '')
  const [disableType, setDisableType] = useState<'permanent' | 'temporary'>(
    'permanent'
  )
  const [disableDurationMinutes, setDisableDurationMinutes] = useState(60)

  const handleEdit = () => {
    setCurrentRow(user)
    setOpen('update')
  }

  const handleDelete = () => {
    setCurrentRow(user)
    setOpen('delete')
  }

  const handleManage = async (action: Exclude<ManageUserAction, 'delete'>) => {
    try {
      const result =
        action === 'disable'
          ? await manageUser({
              id: user.id,
              action,
              disable_reason: disableReason.trim(),
              duration_minutes:
                disableType === 'temporary'
                  ? Math.max(1, disableDurationMinutes)
                  : 0,
            })
          : await manageUser(user.id, action)
      if (result.success) {
        toast.success(t(getUserActionMessage(action)))
        triggerRefresh()
        if (action === 'disable') {
          setDisableDialogOpen(false)
        }
      } else {
        toast.error(
          result.message || t('Failed to {{action}} user', { action })
        )
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    }
  }

  const handleResetPasskey = async () => {
    try {
      const result = await resetUserPasskey(user.id)
      if (result.success) {
        toast.success(t('Passkey reset successfully'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to reset Passkey'))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setResetPasskeyOpen(false)
    }
  }

  const handleResetTwoFA = async () => {
    try {
      const result = await resetUserTwoFA(user.id)
      if (result.success) {
        toast.success(t('Two-factor authentication reset'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to reset 2FA'))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setResetTwoFAOpen(false)
    }
  }

  const isDisabled = user.status === USER_STATUS.DISABLED
  const isAdmin = user.role >= USER_ROLE.ADMIN
  const isRoot = user.role === USER_ROLE.ROOT

  if (isUserDeleted(user)) {
    return null
  }

  return (
    <div className='-ml-1.5 flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleEdit}
              aria-label={t('Edit')}
            />
          }
        >
          <Pencil />
        </TooltipTrigger>
        <TooltipContent>{t('Edit')}</TooltipContent>
      </Tooltip>

      <DataTableRowActionMenu
        ariaLabel={t('Open menu')}
        contentClassName='w-48'
      >
        {isDisabled ? (
          <DropdownMenuItem onClick={() => handleManage('enable')}>
            {t('Enable')}
            <DropdownMenuShortcut>
              <Power size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setDisableReason(user.disable_reason || '')
              setDisableType('permanent')
              setDisableDurationMinutes(60)
              setDisableDialogOpen(true)
            }}
            disabled={isRoot}
          >
            {t('Disable')}
            <DropdownMenuShortcut>
              <PowerOff size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        {isAdmin && !isRoot && (
          <DropdownMenuItem onClick={() => handleManage('demote')}>
            {t('Demote')}
            <DropdownMenuShortcut>
              <ArrowDown size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        {!isAdmin && (
          <DropdownMenuItem onClick={() => handleManage('promote')}>
            {t('Promote')}
            <DropdownMenuShortcut>
              <ArrowUp size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setBindingDialogOpen(true)
          }}
        >
          {t('Manage Bindings')}
          <DropdownMenuShortcut>
            <Link2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setSubscriptionsDialogOpen(true)
          }}
        >
          {t('Manage Subscriptions')}
          <DropdownMenuShortcut>
            <CreditCard size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setTokensDialogOpen(true)
          }}
        >
          {t('Manage Tokens')}
          <DropdownMenuShortcut>
            <KeyRound size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setResetPasskeyOpen(true)
          }}
          disabled={isRoot}
        >
          {t('Reset Passkey')}
          <DropdownMenuShortcut>
            <KeyRound size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setResetTwoFAOpen(true)
          }}
          disabled={isRoot}
        >
          {t('Reset 2FA')}
          <DropdownMenuShortcut>
            <ShieldAlert size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          onClick={handleDelete}
          className='text-destructive focus:text-destructive'
          disabled={isRoot}
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DataTableRowActionMenu>

      <ConfirmDialog
        open={disableDialogOpen}
        onOpenChange={setDisableDialogOpen}
        title={t('Disable User')}
        desc={t(
          'Please fill in the disable reason. The user will see this reason the next time they log in.'
        )}
        confirmText={t('Disable')}
        destructive
        handleConfirm={() => handleManage('disable')}
      >
        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label>{t('Ban type')}</Label>
            <Select
              value={disableType}
              onValueChange={(value) =>
                setDisableType(value as 'permanent' | 'temporary')
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='permanent'>{t('Permanent ban')}</SelectItem>
                <SelectItem value='temporary'>{t('Temporary ban')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {disableType === 'temporary' && (
            <div className='space-y-2'>
              <Label htmlFor={`disable-duration-${user.id}`}>
                {t('Ban duration (minutes)')}
              </Label>
              <Input
                id={`disable-duration-${user.id}`}
                type='number'
                min={1}
                max={525600}
                value={disableDurationMinutes}
                onChange={(event) =>
                  setDisableDurationMinutes(Number(event.target.value))
                }
              />
            </div>
          )}
          <div className='space-y-2'>
            <Label htmlFor={`disable-reason-${user.id}`}>
              {t('Disable reason')}
            </Label>
            <Textarea
              id={`disable-reason-${user.id}`}
              value={disableReason}
              onChange={(event) => setDisableReason(event.target.value)}
              placeholder={t('Enter disable reason')}
              maxLength={DISABLE_REASON_MAX_LENGTH}
              rows={6}
            />
          </div>
        </div>
      </ConfirmDialog>

      <ConfirmDialog
        open={resetPasskeyOpen}
        onOpenChange={setResetPasskeyOpen}
        title={t('Reset Passkey')}
        desc={t(
          'Reset Passkey for {{username}}? The user will need to register a new Passkey before using passwordless login.',
          { username: user.username }
        )}
        confirmText={t('Reset Passkey')}
        handleConfirm={handleResetPasskey}
      />

      <ConfirmDialog
        open={resetTwoFAOpen}
        onOpenChange={setResetTwoFAOpen}
        title={t('Reset Two-Factor Authentication')}
        desc={t(
          'Reset 2FA for {{username}}? The user must set up 2FA again to continue using it.',
          { username: user.username }
        )}
        confirmText={t('Reset 2FA')}
        handleConfirm={handleResetTwoFA}
      />

      <UserBindingDialog
        open={bindingDialogOpen}
        onOpenChange={setBindingDialogOpen}
        userId={user.id}
        onUnbindSuccess={triggerRefresh}
      />

      <UserSubscriptionsDialog
        open={subscriptionsDialogOpen}
        onOpenChange={setSubscriptionsDialogOpen}
        user={{ id: user.id, username: user.username }}
        onSuccess={triggerRefresh}
      />

      <UserTokensDialog
        open={tokensDialogOpen}
        onOpenChange={setTokensDialogOpen}
        user={{ id: user.id, username: user.username }}
      />
    </div>
  )
}
