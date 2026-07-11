import {describe, it, expect, vi, beforeEach} from 'vitest'
import {apiClient} from '@/lib/api/client'

// The download methods are thin wrappers over the Axios instance; verify each
// hits the correct endpoint and requests a Blob response.
const http = (apiClient as unknown as {
  http: { get: (url: string, config?: unknown) => Promise<{ data: Blob }> }
}).http

describe('PDF download client methods', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('downloadNfceDanfe hits /nfces/:key/danfce as a blob', async () => {
    const blob = new Blob(['%PDF'], {type: 'application/pdf'})
    const spy = vi.spyOn(http, 'get').mockResolvedValue({data: blob})
    const result = await apiClient.downloadNfceDanfe('KEY123')
    expect(spy).toHaveBeenCalledWith('/v1.0/nfces/KEY123/danfce', {responseType: 'blob'})
    expect(result).toBe(blob)
  })

  it('downloadMdfeDamdfe hits /mdfes/:key/damdfe as a blob', async () => {
    const blob = new Blob(['%PDF'], {type: 'application/pdf'})
    const spy = vi.spyOn(http, 'get').mockResolvedValue({data: blob})
    const result = await apiClient.downloadMdfeDamdfe('KEY456')
    expect(spy).toHaveBeenCalledWith('/v1.0/mdfes/KEY456/damdfe', {responseType: 'blob'})
    expect(result).toBe(blob)
  })
})
