import {describe, expect, it} from 'vitest'
import {httpRetryDelay} from '@/lib/api/client'

describe('httpRetryDelay', () => {
  it('honours Retry-After when the server names a delay', () => {
    // 429 with a number is the server telling us exactly when to come back;
    // guessing a shorter backoff is how a rate limit turns into a ban.
    expect(httpRetryDelay(1, '2', () => 0)).toBe(2_000)
    expect(httpRetryDelay(1, '2', () => 0.999)).toBeGreaterThanOrEqual(2_000)
    expect(httpRetryDelay(1, '2', () => 0.999)).toBeLessThan(2_250)
  })

  it('ignores a malformed Retry-After', () => {
    expect(httpRetryDelay(1, 'Wed, 21 Oct 2026 07:28:00 GMT', () => 0)).toBe(0)
  })

  it('grows the jitter ceiling per attempt and caps it', () => {
    expect(httpRetryDelay(1, undefined, () => 0.999)).toBeLessThan(250)
    expect(httpRetryDelay(2, undefined, () => 0.999)).toBeLessThan(500)
    for (let attempt = 1; attempt < 20; attempt++) {
      expect(httpRetryDelay(attempt, undefined, () => 0.999)).toBeLessThanOrEqual(3_000)
    }
  })
})
