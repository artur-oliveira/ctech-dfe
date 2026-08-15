import { OAuthClient, decodeIdToken as sdkDecodeIdToken } from '@aoctech/auth-client'
import type { UnverifiedIdTokenClaims } from '@aoctech/auth-client'

const CTECH_URL = process.env.NEXT_PUBLIC_CTECH_URL!
const CLIENT_ID = process.env.NEXT_PUBLIC_CTECH_CLIENT_ID!

export const DFE_USER_SCOPES = [
  'dfe:nfes:read',
  'dfe:nfes:write',
  'dfe:nfces:read',
  'dfe:nfces:write',
  'dfe:ctes:read',
  'dfe:ctes:write',
  'dfe:mdfes:read',
  'dfe:mdfes:write',
  'dfe:nfses:read',
  'dfe:nfses:write',
  'dfe:organization_services:read',
  'dfe:organization_services:write',
  'dfe:organization_products:read',
  'dfe:organization_products:write',
  'dfe:organization_vehicles:read',
  'dfe:organization_vehicles:write',
  'dfe:organization_persons:read',
  'dfe:organization_persons:write',
  'dfe:organizations:read',
  'dfe:organizations:write',
  'dfe:organization_certificates:read',
  'dfe:organization_certificates:write',
] as const

export const DFE_OAUTH_SCOPE = ['openid', 'profile', ...DFE_USER_SCOPES].join(' ')

const client = new OAuthClient({
  baseUrl: CTECH_URL,
  clientId: CLIENT_ID,
  redirectUri: typeof window !== 'undefined' ? `${window.location.origin}/callback` : '',
  scope: DFE_OAUTH_SCOPE,
})

export type { UnverifiedIdTokenClaims }
/** @deprecated Use UnverifiedIdTokenClaims instead */
export type IdTokenClaims = UnverifiedIdTokenClaims

/**
 * Decodes the name-related profile claims from an OIDC id_token.
 *
 * The id_token's audience is the OAuth client itself, so reading profile data from it
 * client-side avoids the ctech-account /userinfo audience block on the DFe access token.
 */
export const decodeIdToken = sdkDecodeIdToken

export async function startOAuthFlow(returnTo = '/'): Promise<void> {
  await client.startOAuthFlow(returnTo)
}

export async function exchangeCode(
  code: string,
  state: string,
): Promise<{ accessToken: string; idToken: string | null; returnTo: string }> {
  const result = await client.exchangeCode(code, state)
  return { accessToken: result.accessToken, idToken: result.idToken ?? null, returnTo: result.returnTo }
}

// M2: refresh_token is no longer passed in the request body — ctech-account
// issues it as an HttpOnly ctech_rt cookie the browser sends automatically via
// credentials:'include'. The SPA only ever sees the short-lived access token.
// Guarded + single-flight (ctech-oauth-client): skips the request entirely
// without the ctech_auth hint cookie or after a local revoked mark, so it
// never burns the shared /token rate limit on a doomed request.
export async function doRefresh(): Promise<{ accessToken: string } | null> {
  const result = await client.refresh()
  return result ? { accessToken: result.accessToken } : null
}

/**
 * Redirects the browser to ctech-account's RP-Initiated-Logout endpoint,
 * ending the SSO session cookie (ctech_session) — not just this app's local
 * tokens. Without this, ctech_session stays valid and /authorize silently
 * re-authenticates on the very next login attempt, bouncing the user straight
 * back in instead of showing a fresh login.
 */
export function endSessionRedirect(returnTo = '/login'): void {
  client.endSessionRedirect(returnTo)
}

// M2: the refresh token lives in the HttpOnly ctech_rt cookie; we don't have it
// in JS. credentials:'include' sends the cookie and ctech-account's /revoke
// clears it, ending the refresh chain.
export async function revokeToken(): Promise<void> {
  await client.revoke()
}

/** Closes the OAuth client's BroadcastChannel to prevent memory/event-loop leaks in tests. */
export function close(): void {
  client.close()
}
