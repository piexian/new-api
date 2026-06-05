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
import { IPBansBatchDialog } from './ip-bans-batch-dialog'
import { IPBansDeleteDialog } from './ip-bans-delete-dialog'
import { IPBansMutateDrawer } from './ip-bans-mutate-drawer'
import { useIPBans } from './ip-bans-provider'

export function IPBansDialogs() {
  const { open, setOpen, currentRow } = useIPBans()
  const isUpdate = open === 'update'

  return (
    <>
      <IPBansMutateDrawer
        open={open === 'create' || isUpdate}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={isUpdate ? currentRow || undefined : undefined}
      />
      <IPBansBatchDialog
        open={open === 'batch'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
      />
      <IPBansDeleteDialog />
    </>
  )
}
