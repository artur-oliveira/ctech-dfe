import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiClient} from '@/lib/api/client';
import {mdfeSefazStatusFixture, mdfeUnclosedFixture} from '@/lib/mock/fixtures';

// Mesmo padrão de distributions-client.test.ts: espiona o wrapper privado `get`.
const priv = apiClient as unknown as { get: (url: string, config?: unknown) => Promise<unknown> };

describe('mdfe api client: consultas na SEFAZ', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('getMdfeSefazStatus lê /v1.0/mdfes/{key}/sefaz-status', async () => {
    const spy = vi.spyOn(priv, 'get').mockResolvedValue(mdfeSefazStatusFixture as never);
    expect(await apiClient.getMdfeSefazStatus('KEY')).toEqual(mdfeSefazStatusFixture);
    expect(spy).toHaveBeenCalledWith('/v1.0/mdfes/KEY/sefaz-status');
  });

  it('getMdfeUnclosed lê /v1.0/mdfes/unclosed', async () => {
    const spy = vi.spyOn(priv, 'get').mockResolvedValue(mdfeUnclosedFixture as never);
    expect(await apiClient.getMdfeUnclosed()).toEqual(mdfeUnclosedFixture);
    expect(spy).toHaveBeenCalledWith('/v1.0/mdfes/unclosed');
  });
});
