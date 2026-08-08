export const SESSION_KEY_REFRESH = 'pydfe_rt'
export const STORAGE_KEY_USER = 'pydfe_user'
export const STORAGE_KEY_ORG = 'pydfe_org'

/** In-progress emission drafts. One entry per document type + organization —
 *  see lib/hooks/useEmitDraft.ts for the `${prefix}_${docType}_${orgPk}` shape. */
export const STORAGE_KEY_EMIT_DRAFT_PREFIX = 'pydfe_emit_draft'

/** Saved filter views (named NSU filters) per distribution page. JSON map
 *  keyed by page id → {name, nsu}[]; see lib/hooks/useSavedFilterViews.ts. */
export const STORAGE_KEY_SAVED_FILTER_VIEWS = 'pydfe_saved_filter_views'
