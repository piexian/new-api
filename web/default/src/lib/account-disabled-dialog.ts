export type AccountDisabledDialogPayload = {
  title?: string
  message?: string
  reason?: string
  userId?: number | string
  username?: string
}

export const ACCOUNT_DISABLED_DIALOG_EVENT = 'newapi:account-disabled-dialog'

export function showAccountDisabledDialog(
  payload: AccountDisabledDialogPayload
) {
  if (typeof window === 'undefined') return

  window.dispatchEvent(
    new CustomEvent<AccountDisabledDialogPayload>(
      ACCOUNT_DISABLED_DIALOG_EVENT,
      {
        detail: payload,
      }
    )
  )
}
