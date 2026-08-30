import {describe, it, expect, vi, beforeEach} from 'vitest'
import {apiClient} from '@/lib/api/client'
import type {SignedFileDownload} from '@/lib/types/api'

const client = apiClient as unknown as {
  get: (path: string, config?: {timeout: number}) => Promise<SignedFileDownload>
}

describe('PDF download client methods', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('downloadNfceDanfe returns the cached S3 download descriptor', async () => {
    const download = {
      url: 'https://s3.example/nfce.pdf', expires_at: '2026-08-29T12:00:00Z',
      filename: 'nfce.pdf', content_type: 'application/pdf', cached: true,
    }
    const spy = vi.spyOn(client, 'get').mockResolvedValue(download)
    const result = await apiClient.downloadNfceDanfe('KEY123')
    expect(spy).toHaveBeenCalledWith('/v1.0/nfces/KEY123/danfce', {timeout: 12_000})
    expect(result).toBe(download)
  })

  it('downloadMdfeDamdfe returns the cached S3 download descriptor', async () => {
    const download = {
      url: 'https://s3.example/mdfe.pdf', expires_at: '2026-08-29T12:00:00Z',
      filename: 'mdfe.pdf', content_type: 'application/pdf', cached: false,
    }
    const spy = vi.spyOn(client, 'get').mockResolvedValue(download)
    const result = await apiClient.downloadMdfeDamdfe('KEY456')
    expect(spy).toHaveBeenCalledWith('/v1.0/mdfes/KEY456/damdfe', {timeout: 12_000})
    expect(result).toBe(download)
  })

  it('downloadNfeDanfe returns the cached S3 download descriptor', async () => {
    const download = {
      url: 'https://s3.example/nfe.pdf', expires_at: '2026-08-29T12:00:00Z',
      filename: 'nfe.pdf', content_type: 'application/pdf', cached: true,
    }
    const spy = vi.spyOn(client, 'get').mockResolvedValue(download)
    const result = await apiClient.downloadNfeDanfe('KEY789')
    expect(spy).toHaveBeenCalledWith('/v1.0/nfes/KEY789/danfe', {timeout: 12_000})
    expect(result).toBe(download)
  })
})
