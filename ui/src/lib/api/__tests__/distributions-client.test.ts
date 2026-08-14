import {describe, expect, it, vi, beforeEach} from 'vitest'
import {apiClient} from '@/lib/api/client'

// Casts through unknown: these hit the private post/get wrappers, the same
// pattern the rest of the client's tests use — see nfse-client.test.ts.
const priv = apiClient as unknown as {
  get: (url: string, config?: unknown) => Promise<unknown>
  post: (url: string, body?: unknown) => Promise<unknown>
}

describe('distributions api client', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('importNfeByKey posts to /v1.0/distributions/nfe/key', async () => {
    const spy = vi.spyOn(priv, 'post').mockResolvedValue({status: 'enqueued'} as never)
    const result = await apiClient.importNfeByKey('35250512345678000195550010000000011000000015')
    expect(spy).toHaveBeenCalledWith('/v1.0/distributions/nfe/key', {access_key: '35250512345678000195550010000000011000000015'})
    expect(result).toEqual({status: 'enqueued'})
  })

  it('importXML posts multipart with the file field to the doc_type-specific route', async () => {
    const httpPriv = apiClient as unknown as {
      http: { post: (url: string, body?: unknown, config?: unknown) => Promise<{ data: unknown }> }
    }
    const spy = vi.spyOn(httpPriv.http, 'post').mockResolvedValue({data: {status: 'enqueued'}})

    const file = new File(['<nfeProc/>'], 'nota.xml', {type: 'application/xml'})
    const result = await apiClient.importXML('nfe', file)

    expect(result).toEqual({status: 'enqueued'})
    expect(spy).toHaveBeenCalledWith(
      '/v1.0/distributions/nfe/import-xml',
      expect.any(FormData),
      {headers: {'Content-Type': undefined}},
    )
  })
})
