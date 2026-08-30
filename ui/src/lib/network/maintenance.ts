/**
 * Maintenance: the server answering 503.
 *
 * Distinct from the outage the liveness probe owns. An unreachable API is
 * ambiguous — a dead load balancer, a lost network, a laptop that slept — and
 * the banner keeps the person on the screen they were on while it polls. A 503
 * is not ambiguous: the server is up, answering, and saying it will not serve.
 * There is nothing to keep working on, so it gets a screen rather than a
 * banner.
 */
export const MAINTENANCE_PATH = '/unavailable'

/** Where to go back to once it is over. */
const RETURN_KEY = 'dfe:return-after-maintenance'

const FALLBACK_DESTINATION = '/dashboard'

/**
 * Sends the browser to the maintenance screen, remembering where it was.
 *
 * Returns whether the status was a 503 at all, so a caller can tell "handled"
 * from "not mine". Idempotent: already on the screen, it stays there rather
 * than replacing the URL again and losing the remembered destination.
 */
export function redirectOnMaintenance(status?: number): boolean {
  if (status !== 503 || typeof window === 'undefined') return false
  if (window.location.pathname === MAINTENANCE_PATH) return true
  try {
    sessionStorage.setItem(RETURN_KEY, `${window.location.pathname}${window.location.search}`)
  } catch {
    // Private modes refuse storage. The fallback destination still works.
  }
  window.location.replace(MAINTENANCE_PATH)
  return true
}

/** The remembered destination, consumed. Never the maintenance screen itself. */
export function takeMaintenanceReturn(): string {
  try {
    const stored = sessionStorage.getItem(RETURN_KEY)
    sessionStorage.removeItem(RETURN_KEY)
    if (stored && !stored.startsWith(MAINTENANCE_PATH)) return stored
  } catch {
    // Same as above: fall through to the safe destination.
  }
  return FALLBACK_DESTINATION
}
