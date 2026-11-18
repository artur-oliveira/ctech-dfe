export const SESSION_KEY_REFRESH = 'pydfe_rt';
export const STORAGE_KEY_USER = 'pydfe_user';
export const STORAGE_KEY_ORG = 'pydfe_org';

/** In-progress emission drafts. One entry per document type + organization —
 *  see lib/hooks/useEmitDraft.ts for the `${prefix}_${docType}_${orgPk}` shape. */
export const STORAGE_KEY_EMIT_DRAFT_PREFIX = 'pydfe_emit_draft';

/** Onboarding layers the user was offered and chose to skip. A preference, not
 *  fiscal state: the server has no business knowing someone decided to register
 *  products later. JSON array of step ids, per organization —
 *  `${prefix}_${orgPk}`; see lib/hooks/useOnboarding.ts. */
export const STORAGE_KEY_ONBOARDING_SKIPPED_PREFIX = 'pydfe_onboarding_skipped';

/** Last-sync stamp of this browser's push subscription, kept by `@aoctech/push-client` under this exact key (a test
 *  compares it with the package). Removing it makes `syncOnLoad` resend the subscription on the next load. */
export const STORAGE_KEY_PUSH_STAMP = 'ctech-push:last-sync';

/** Id of the person this browser's push subscription was last synced for (per-browser bookkeeping, no token). When a
 *  different person signs in the stamp above is dropped, so the server moves the endpoint to the new person. */
export const STORAGE_KEY_PUSH_USER = 'ctech-dfe:push-user';

/** Saved filter views (named NSU filters) per distribution page. JSON map
 *  keyed by page id → {name, nsu}[]; see lib/hooks/useSavedFilterViews.ts. */
export const STORAGE_KEY_SAVED_FILTER_VIEWS = 'pydfe_saved_filter_views';
