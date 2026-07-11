const CTECH_URL = process.env.NEXT_PUBLIC_CTECH_URL!
const CLIENT_ID = process.env.NEXT_PUBLIC_CTECH_CLIENT_ID!

/** Name-related profile claims decoded from the OIDC id_token. */
export interface IdTokenClaims {
  username?: string
  first_name?: string
  last_name?: string
}

/**
 * Decodes the name-related profile claims from an OIDC id_token.
 *
 * The id_token's audience is the OAuth client itself, so reading profile data from it
 * client-side avoids the ctech-account /userinfo audience block on the DFe access token.
 * Maps preferred_username -> username, given_name -> first_name, family_name -> last_name.
 * Returns null on any parse failure or when no name claim is present.
 */
export function decodeIdToken(idToken: string): IdTokenClaims | null {
  try {
    const payload = idToken.split('.')[1]
    if (!payload) return null

    let b64 = payload.replace(/-/g, '+').replace(/_/g, '/')
    b64 += '='.repeat((4 - (b64.length % 4)) % 4)

    // atob yields a binary string; re-decode as UTF-8 so accented names survive.
    const json = decodeURIComponent(
      Array.from(atob(b64), (c) => '%' + c.charCodeAt(0).toString(16).padStart(2, '0')).join(''),
    )
    const claims = JSON.parse(json) as Record<string, unknown>

    const firstName = typeof claims.given_name === 'string' ? claims.given_name : undefined
    const lastName = typeof claims.family_name === 'string' ? claims.family_name : undefined
    const username = typeof claims.preferred_username === 'string' ? claims.preferred_username : undefined

    if (!firstName && !lastName && !username) return null
    return { username, first_name: firstName, last_name: lastName }
  } catch {
    return null
  }
}

function randomHex(n: number): string {
  const arr = new Uint8Array(n)
  crypto.getRandomValues(arr)
  return Array.from(arr, (b) => b.toString(16).padStart(2, '0')).join('')
}

async function sha256base64url(s: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s))
  return btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

export async function startOAuthFlow(returnTo = '/'): Promise<void> {
  const state = randomHex(16)
  const verifier = randomHex(32)
  const challenge = await sha256base64url(verifier)

  sessionStorage.setItem('oauth_state', state)
  sessionStorage.setItem('oauth_verifier', verifier)
  sessionStorage.setItem('oauth_return_to', returnTo)

  const params = new URLSearchParams({
    response_type: 'code',
    client_id: CLIENT_ID,
    redirect_uri: `${window.location.origin}/callback`,
    scope: 'openid profile',
    state,
    code_challenge: challenge,
    code_challenge_method: 'S256',
  })

  window.location.href = `${CTECH_URL}/v1.0/authorize?${params}`
}

export async function exchangeCode(
  code: string,
  state: string,
): Promise<{ accessToken: string; refreshToken: string; idToken: string | null; returnTo: string }> {
  const storedState = sessionStorage.getItem('oauth_state')
  if (storedState !== state) throw new Error('OAuth state mismatch')

  const verifier = sessionStorage.getItem('oauth_verifier') ?? ''
  const returnTo = sessionStorage.getItem('oauth_return_to') ?? '/'

  sessionStorage.removeItem('oauth_state')
  sessionStorage.removeItem('oauth_verifier')
  sessionStorage.removeItem('oauth_return_to')

  const body = new URLSearchParams({
    grant_type: 'authorization_code',
    code,
    code_verifier: verifier,
    client_id: CLIENT_ID,
    redirect_uri: `${window.location.origin}/callback`,
  })

  const res = await fetch(`${CTECH_URL}/v1.0/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    credentials: 'include',
    body: body.toString(),
  })

  if (!res.ok) {
    const text = await res.text()
    throw new Error(`Token exchange failed (${res.status}): ${text}`)
  }

  const data = await res.json()
  return {
    accessToken: data.access_token,
    refreshToken: data.refresh_token,
    idToken: data.id_token ?? null,
    returnTo,
  }
}

export async function doRefresh(
  refreshToken: string,
): Promise<{ accessToken: string; refreshToken: string } | null> {
  try {
    const body = new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: refreshToken,
      client_id: CLIENT_ID,
    })

    const res = await fetch(`${CTECH_URL}/v1.0/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      credentials: 'include',
      body: body.toString(),
    })

    if (!res.ok) return null
    const data = await res.json()
    return { accessToken: data.access_token, refreshToken: data.refresh_token }
  } catch {
    return null
  }
}

/**
 * Redirects the browser to ctech-account's RP-Initiated-Logout endpoint,
 * ending the SSO session cookie (ctech_session) — not just this app's local
 * tokens. Without this, ctech_session stays valid and /authorize silently
 * re-authenticates on the very next login attempt, bouncing the user straight
 * back in instead of showing a fresh login.
 */
export function endSessionRedirect(returnTo = '/login'): void {
  const params = new URLSearchParams({
    client_id: CLIENT_ID,
    post_logout_redirect_uri: `${window.location.origin}${returnTo}`,
  })
  window.location.href = `${CTECH_URL}/v1.0/auth/end-session?${params}`
}

export async function revokeToken(refreshToken: string): Promise<void> {
  try {
    const body = new URLSearchParams({ token: refreshToken })
    await fetch(`${CTECH_URL}/v1.0/revoke`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      credentials: 'include',
      body: body.toString(),
    })
  } catch {
    // best-effort
  }
}
