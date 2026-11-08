import {describe, it, expect, vi, beforeEach} from 'vitest';
import {apiClient} from '@/lib/api/client';
import type {SignedFileDownload} from '@/lib/types/api';

const client = apiClient as unknown as {
  get: (path: string, config?: {timeout: number}) => Promise<SignedFileDownload>
};

describe('PDF download client methods', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('downloadNfceDanfe returns the cached S3 download descriptor', async () => {
    const download = {
      url: 'https://s3.example/nfce.pdf', expires_at: '2026-08-29T12:00:00Z',
      filename: 'nfce.pdf', content_type: 'application/pdf', cached: true,
    };
    const spy = vi.spyOn(client, 'get').mockResolvedValue(download);
    const result = await apiClient.downloadNfceDanfe('KEY123');
    expect(spy).toHaveBeenCalledWith('/v1.0/nfces/KEY123/danfce', {timeout: 12_000});
    expect(result).toBe(download);
  });

  it('downloadMdfeDamdfe returns the cached S3 download descriptor', async () => {
    const download = {
      url: 'https://s3.example/mdfe.pdf', expires_at: '2026-08-29T12:00:00Z',
      filename: 'mdfe.pdf', content_type: 'application/pdf', cached: false,
    };
    const spy = vi.spyOn(client, 'get').mockResolvedValue(download);
    const result = await apiClient.downloadMdfeDamdfe('KEY456');
    expect(spy).toHaveBeenCalledWith('/v1.0/mdfes/KEY456/damdfe', {timeout: 12_000});
    expect(result).toBe(download);
  });

  it('downloadNfeDanfe returns the cached S3 download descriptor', async () => {
    const download = {
      url: 'https://s3.example/nfe.pdf', expires_at: '2026-08-29T12:00:00Z',
      filename: 'nfe.pdf', content_type: 'application/pdf', cached: true,
    };
    const spy = vi.spyOn(client, 'get').mockResolvedValue(download);
    const result = await apiClient.downloadNfeDanfe('KEY789');
    expect(spy).toHaveBeenCalledWith('/v1.0/nfes/KEY789/danfe', {timeout: 12_000});
    expect(result).toBe(download);
  });

  it('sends format only when chosen and never as a per-format method', async () => {
    const spy = vi.spyOn(client, 'get').mockResolvedValue({} as SignedFileDownload);
    await apiClient.downloadNfeDanfe('K1', 'etiqueta');
    await apiClient.downloadNfceDanfe('K2', 'resumido');
    await apiClient.downloadMdfeDamdfe('K3', 'paisagem');
    await apiClient.downloadCteDacte('K4');
    expect(spy).toHaveBeenNthCalledWith(1, '/v1.0/nfes/K1/danfe', {timeout: 12_000, params: {format: 'etiqueta'}});
    expect(spy).toHaveBeenNthCalledWith(2, '/v1.0/nfces/K2/danfce', {timeout: 12_000, params: {format: 'resumido'}});
    expect(spy).toHaveBeenNthCalledWith(3, '/v1.0/mdfes/K3/damdfe', {timeout: 12_000, params: {format: 'paisagem'}});
    expect(spy).toHaveBeenNthCalledWith(4, '/v1.0/ctes/K4/dacte', {timeout: 12_000});
  });
});
