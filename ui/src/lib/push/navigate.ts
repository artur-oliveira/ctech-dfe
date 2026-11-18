/** Query parameter a push link uses to name the company the notification is about (`navigate` in the payload). */
export const PUSH_ORG_PARAM = 'org';

/**
 * The company key a link names, or null. The path is already a same-origin path (`safePath` in the push client
 * guarantees it); the base only lets `URL` parse it.
 */
export function orgOfPath(path: string): string | null {
  try {
    return new URL(path, 'http://app.invalid').searchParams.get(PUSH_ORG_PARAM);
  } catch {
    return null;
  }
}
