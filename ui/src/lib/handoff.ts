/**
 * Sending somebody to the CTech account to register a company.
 *
 * A company's identity — its CNPJ and legal name — belongs to ctech-account
 * (ctech-billing ADR 0022). This product does not create one; it adopts the one
 * the account created, and asks only for what is its own: the certificate, the
 * inscrição estadual, the série.
 *
 * One function because there are two doors into the same handoff (the companies
 * list and the onboarding step) and two copies of this would drift on the day
 * the parameters change.
 */

/** Where the account sends the browser back to. */
export const HANDOFF_RETURN_PATH = '/organizations/link'

/** The state we sent, kept for the return leg to check. */
export const HANDOFF_STATE_KEY = 'dfe:handoff-state'

export function startCompanyHandoff(): void {
  const accountUrl = process.env.NEXT_PUBLIC_CTECH_CLIENT_URL ?? ''
  // state is ours and opaque to the account: it echoes it back untouched, and
  // it is what tells a real return apart from somebody opening the landing URL
  // by hand with ids they typed.
  const state = crypto.randomUUID()
  try {
    sessionStorage.setItem(HANDOFF_STATE_KEY, state)
  } catch {
    // Private modes refuse storage. The return leg treats a missing expected
    // state as "cannot check" rather than as a mismatch, so the handoff still
    // completes — it just loses this one guard.
  }

  const url = new URL(`${accountUrl}/account/organizations/new`)
  url.searchParams.set('client_id', 'dfe')
  url.searchParams.set('return_to', `${window.location.origin}${HANDOFF_RETURN_PATH}`)
  url.searchParams.set('state', state)
  // replace, not assign: the person must not land back on the departure screen
  // with the browser Back button and start a second handoff.
  window.location.replace(url.toString())
}
