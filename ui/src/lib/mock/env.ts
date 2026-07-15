/**
 * Dev-only mock API gate. NEVER enabled in production builds: the value is
 * inlined at build time from `NEXT_PUBLIC_MOCK_API` and the flag is never set
 * in deployed environments. When true, the app bypasses real OAuth and serves
 * fixtures from `handler.ts` so authenticated surfaces can be exercised in the
 * browser without a backend or valid credentials.
 */
export const MOCK_ENABLED = process.env.NEXT_PUBLIC_MOCK_API === 'true'
