export const SESSION_KEY_REFRESH = 'pydfe_rt'
export const STORAGE_KEY_USER = 'pydfe_user'
export const STORAGE_KEY_ORG = 'pydfe_org'

/** In-progress emission drafts. One entry per document type + organization —
 *  see lib/hooks/useEmitDraft.ts for the `${prefix}_${docType}_${orgPk}` shape. */
export const STORAGE_KEY_EMIT_DRAFT_PREFIX = 'pydfe_emit_draft'

/** Onboarding layers the user was offered and chose to skip. A preference, not
 *  fiscal state: the server has no business knowing someone decided to register
 *  products later. JSON array of step ids, per organization —
 *  `${prefix}_${orgPk}`; see lib/hooks/useOnboarding.ts. */
export const STORAGE_KEY_ONBOARDING_SKIPPED_PREFIX = 'pydfe_onboarding_skipped'

/** Saved filter views (named NSU filters) per distribution page. JSON map
 *  keyed by page id → {name, nsu}[]; see lib/hooks/useSavedFilterViews.ts. */
export const STORAGE_KEY_SAVED_FILTER_VIEWS = 'pydfe_saved_filter_views'
