import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {
  ApiUnavailableError,
  HEALTHY_POLL_INTERVAL_MS,
  MAX_UNAVAILABLE_POLL_INTERVAL_MS,
  checkApiLiveness,
  getApiLivenessSnapshot,
  livenessPollDelay,
  requireApiLiveness,
  resetApiLivenessForTests,
} from '@/lib/network/liveness'

function mockFetch(impl: () => Promise<Response>) {
  vi.stubGlobal('fetch', vi.fn(impl))
}

describe('api liveness', () => {
  beforeEach(() => resetApiLivenessForTests())
  afterEach(() => vi.unstubAllGlobals())

  it('treats a degraded (207) health answer as available', async () => {
    // A warn only means a non-load-bearing dependency is slow. Taking the whole
    // product away for that is a worse outage than the one being reported.
    mockFetch(async () => new Response('{}', {status: 207}))
    await expect(checkApiLiveness()).resolves.toBe(true)
    expect(getApiLivenessSnapshot().status).toBe('available')
  })

  it('marks the API unavailable on a 503', async () => {
    mockFetch(async () => new Response('{}', {status: 503}))
    await expect(checkApiLiveness()).resolves.toBe(false)
    expect(getApiLivenessSnapshot()).toMatchObject({status: 'unavailable', reason: 'server'})
  })

  it('treats a transport rejection like a failed response', async () => {
    // A dead load balancer answers without CORS headers, so the browser reports
    // a TypeError rather than a status.
    mockFetch(async () => {
      throw new TypeError('Failed to fetch')
    })
    await expect(checkApiLiveness()).resolves.toBe(false)
    expect(getApiLivenessSnapshot().status).toBe('unavailable')
  })

  it('shares one in-flight probe between concurrent callers', async () => {
    const fetchMock = vi.fn(async () => new Response('{}', {status: 200}))
    vi.stubGlobal('fetch', fetchMock)
    await Promise.all([checkApiLiveness(), checkApiLiveness(), checkApiLiveness()])
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('fails fast instead of probing again while the API is known to be down', async () => {
    const fetchMock = vi.fn(async () => new Response('{}', {status: 503}))
    vi.stubGlobal('fetch', fetchMock)
    await checkApiLiveness()
    fetchMock.mockClear()

    await expect(requireApiLiveness()).rejects.toBeInstanceOf(ApiUnavailableError)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('lets requests through once the API answers', async () => {
    mockFetch(async () => new Response('{}', {status: 200}))
    await expect(requireApiLiveness()).resolves.toBeUndefined()
  })

  it('backs off with equal jitter and stays inside the ceiling', () => {
    expect(livenessPollDelay(0)).toBe(HEALTHY_POLL_INTERVAL_MS)
    // Equal jitter: never a busy loop near zero, never a synchronized fleet.
    expect(livenessPollDelay(1, () => 0)).toBe(500)
    expect(livenessPollDelay(1, () => 0.999)).toBeLessThan(1_000)
    for (let failures = 1; failures < 20; failures++) {
      const delay = livenessPollDelay(failures, () => 0.999)
      expect(delay).toBeGreaterThanOrEqual(500)
      expect(delay).toBeLessThanOrEqual(MAX_UNAVAILABLE_POLL_INTERVAL_MS)
    }
  })
})
