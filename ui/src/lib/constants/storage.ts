export const SESSION_KEY_REFRESH = 'pydfe_rt'
export const STORAGE_KEY_USER = 'pydfe_user'
export const STORAGE_KEY_ORG = 'pydfe_org'

/** Tab-scoped auth state gate. When 'revoked', the app skips /token calls it
 *  knows will 400 (refresh cookie cleared by logout). Set from /token and
 *  /revoke responses. sessionStorage — same lifetime as the ctech_rt cookie. */
export const SESSION_KEY_AUTH_STATE = 'pydfe_auth'
export const AUTH_STATE_ACTIVE = 'active'
export const AUTH_STATE_REVOKED = 'revoked'

/** Saved filter views (named NSU filters) per distribution page. JSON map
 *  keyed by page id → {name, nsu}[]; see lib/hooks/useSavedFilterViews.ts. */
export const STORAGE_KEY_SAVED_FILTER_VIEWS = 'pydfe_saved_filter_views'
