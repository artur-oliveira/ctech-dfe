/**
 * API liveness: one shared answer to "is the backend reachable right now?".
 *
 * Every screen in this product is a query against the same API, so an outage
 * that each query discovers on its own becomes N failing requests, N toasts and
 * N retry loops for one cause. The health probe is the only request allowed
 * while the API is down; everything else fails fast against this snapshot and
 * waits for the probe to say the API came back.
 *
 * A fetch rejection is treated exactly like a failed response on purpose: a
 * dead load balancer answers without CORS headers, and the browser exposes that
 * outage as a TypeError rather than as an HTTP status.
 */

/** The API is fronted by a load balancer and answers fast or not at all. */
export const HTTP_TIMEOUT_MS = 5_000
/** Health path — public (no auth, not gated by the subscription middleware). */
export const HEALTH_PATH = '/v1.0/health-check'
export const HEALTHY_POLL_INTERVAL_MS = 30_000
export const MAX_UNAVAILABLE_POLL_INTERVAL_MS = 30_000
const FIRST_BACKOFF_MS = 1_000

export type ApiLivenessStatus = 'checking' | 'available' | 'unavailable'
export type ApiUnavailableReason = 'offline' | 'server' | null

export interface ApiLivenessSnapshot {
  status: ApiLivenessStatus
  reason: ApiUnavailableReason
  checkedAt: number | null
}

export class ApiUnavailableError extends Error {
  constructor(public readonly reason: Exclude<ApiUnavailableReason, null>) {
    super(reason === 'offline' ? 'Sem conexão com a internet' : 'Servidor temporariamente indisponível')
    this.name = 'ApiUnavailableError'
  }
}

const INITIAL_SNAPSHOT: ApiLivenessSnapshot = {status: 'checking', reason: null, checkedAt: null}

let snapshot = INITIAL_SNAPSHOT
let inFlightCheck: Promise<boolean> | null = null
const listeners = new Set<() => void>()

function healthURL(): string {
  const base = (process.env.NEXT_PUBLIC_API_URL ?? '').replace(/\/$/, '')
  return `${base}${HEALTH_PATH}`
}

function publish(next: ApiLivenessSnapshot): void {
  if (
    snapshot.status === next.status &&
    snapshot.reason === next.reason &&
    snapshot.checkedAt === next.checkedAt
  ) return
  snapshot = next
  listeners.forEach((l) => l())
}

function browserIsOffline(): boolean {
  return typeof navigator !== 'undefined' && navigator.onLine === false
}

export function getApiLivenessSnapshot(): ApiLivenessSnapshot {
  return snapshot
}

/** Static export renders on the server with no network — always "checking". */
export function getServerApiLivenessSnapshot(): ApiLivenessSnapshot {
  return INITIAL_SNAPSHOT
}

export function subscribeApiLiveness(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function markApiOffline(): void {
  publish({status: 'unavailable', reason: 'offline', checkedAt: Date.now()})
}

/** Probes the health endpoint. Concurrent callers share one in-flight request. */
export function checkApiLiveness(): Promise<boolean> {
  if (browserIsOffline()) {
    markApiOffline()
    return Promise.resolve(false)
  }
  if (inFlightCheck) return inFlightCheck

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), HTTP_TIMEOUT_MS)
  inFlightCheck = fetch(healthURL(), {
    method: 'GET',
    cache: 'no-store',
    credentials: 'omit',
    headers: {Accept: 'application/json'},
    signal: controller.signal,
  })
    .then((response) => {
      // 207 is "warn": a degraded dependency the customer cannot act on, and
      // not a reason to take the whole product away from them.
      const available = response.ok || response.status === 207
      publish({
        status: available ? 'available' : 'unavailable',
        reason: available ? null : 'server',
        checkedAt: Date.now(),
      })
      return available
    })
    .catch(() => {
      publish({
        status: 'unavailable',
        reason: browserIsOffline() ? 'offline' : 'server',
        checkedAt: Date.now(),
      })
      return false
    })
    .finally(() => {
      clearTimeout(timeout)
      inFlightCheck = null
    })
  return inFlightCheck
}

/**
 * What every API call waits on: the first health result, and then a fast
 * failure for as long as the API is down. This keeps the probe — not each
 * mounted query — responsible for discovering that the API came back.
 */
export async function requireApiLiveness(): Promise<void> {
  if (snapshot.status === 'available') return
  if (snapshot.status === 'unavailable') throw new ApiUnavailableError(snapshot.reason ?? 'server')
  if (!(await checkApiLiveness())) throw new ApiUnavailableError(snapshot.reason ?? 'server')
}

/** Equal jitter: no busy loop near zero, no fleet of clients retrying in step. */
export function livenessPollDelay(failureCount: number, random: () => number = Math.random): number {
  if (failureCount <= 0) return HEALTHY_POLL_INTERVAL_MS
  const ceiling = Math.min(MAX_UNAVAILABLE_POLL_INTERVAL_MS, FIRST_BACKOFF_MS * 2 ** (failureCount - 1))
  return Math.floor(ceiling / 2 + (random() * ceiling) / 2)
}

/** Test-only reset, kept explicit so runtime code cannot silently hide an outage. */
export function resetApiLivenessForTests(): void {
  snapshot = INITIAL_SNAPSHOT
  inFlightCheck = null
  listeners.clear()
}
