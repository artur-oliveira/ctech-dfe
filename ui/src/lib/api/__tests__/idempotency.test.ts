import {describe, expect, it} from 'vitest';
import {withIdempotencyKey, IDEMPOTENCY_HEADER} from '@/lib/api/client';

describe('withIdempotencyKey', () => {
  it('adds a key to a POST and keeps it on replay', () => {
    const config = {method: 'post', headers: {}};
    withIdempotencyKey(config, () => 'k1');
    withIdempotencyKey(config, () => 'k2');
    expect((config.headers as Record<string, string>)[IDEMPOTENCY_HEADER]).toBe('k1');
  });

  it('leaves reads alone', () => {
    const config = {method: 'get', headers: {}};
    withIdempotencyKey(config, () => 'k1');
    expect(config.headers).toEqual({});
  });
});
