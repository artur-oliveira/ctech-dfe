import {describe, expect, it, vi, beforeEach} from 'vitest'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'

// Casts through unknown: these hit the private post/get/put wrappers, the
// same pattern the rest of the client's tests use — see danfe-client.test.ts.
const priv = apiClient as unknown as {
  get: (url: string, config?: unknown) => Promise<unknown>
  post: (url: string, body?: unknown) => Promise<unknown>
  put: (url: string, body?: unknown) => Promise<unknown>
  del: (url: string) => Promise<unknown>
}
describe('nfse api client', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('emitNfse posts to /v1.0/nfses', async () => {
    const spy = vi.spyOn(priv, 'post').mockResolvedValue({} as never)
    await apiClient.emitNfse({tp_emit: 1, competence: '2026-08-01', service: {service_id: 'SERVICE_x'}})
    expect(spy).toHaveBeenCalledWith('/v1.0/nfses', {tp_emit: 1, competence: '2026-08-01', service: {service_id: 'SERVICE_x'}})
  })

  it('getNfse hits /v1.0/nfses/:id with the id_dps, not the access key', async () => {
    const spy = vi.spyOn(priv, 'get').mockResolvedValue({} as never)
    await apiClient.getNfse('DPS2211001...')
    expect(spy).toHaveBeenCalledWith('/v1.0/nfses/DPS2211001...')
  })

  it('cancelNfse posts reason_code/reason_description/sequence_number to /cancel', async () => {
    const spy = vi.spyOn(priv, 'post').mockResolvedValue({} as never)
    await apiClient.cancelNfse('DPS123', '2', 'erro de valor', 1)
    expect(spy).toHaveBeenCalledWith('/v1.0/nfses/DPS123/cancel', {
      reason_code: '2', reason_description: 'erro de valor', sequence_number: 1,
    })
  })

  it('getNfseConfig hits the org-scoped /nfse-config path, not a flat one', async () => {
    const spy = vi.spyOn(priv, 'get').mockResolvedValue({} as never)
    await apiClient.getNfseConfig('11222333000181')
    expect(spy).toHaveBeenCalledWith('/v1.0/organizations/11222333000181/nfse-config')
  })

  it('listNfseDistributions hits the dedicated /nfse/distributions route, not /distributions/nfse/*', async () => {
    const spy = vi.spyOn(priv, 'get').mockResolvedValue({} as never)
    await apiClient.listNfseDistributions({limit: 20})
    expect(spy).toHaveBeenCalledWith('/v1.0/nfse/distributions', {params: {limit: 20}})
  })

  it('getMunicipalParameters hits /v1.0/nfse/municipal-parameters/:city/:kind', async () => {
    const spy = vi.spyOn(priv, 'get').mockResolvedValue({} as never)
    await apiClient.getMunicipalParameters('2211001', 'aliquota', {service: '010101', competence: '01/2026'})
    expect(spy).toHaveBeenCalledWith('/v1.0/nfse/municipal-parameters/2211001/aliquota', {
      params: {service: '010101', competence: '01/2026'},
    })
  })

  it('downloadNfseXml requests a signed URL from /v1.0/nfses/:id/xml', async () => {
    const download = {url: 'https://s3.invalid/nfse.xml', expires_at: '2026-08-30T18:00:00Z', filename: 'nfse.xml', content_type: 'application/xml'}
    const spy = vi.spyOn(priv, 'get').mockResolvedValue(download)
    const result = await apiClient.downloadNfseXml('DPS123')
    expect(spy).toHaveBeenCalledWith('/v1.0/nfses/DPS123/xml')
    expect(result).toBe(download)
  })

  it('createService posts to /v1.0/services (org header injected, not path-scoped)', async () => {
    const spy = vi.spyOn(priv, 'post').mockResolvedValue({} as never)
    await apiClient.createService({
      code: 'SVC-001', description: 'Consultoria', trib_nacional_code: '10101',
      unit: 'UN', value: '1000.00', iss: {trib_issqn: 1, tax_rate: '2.00'}, ibs_cbs:{
        c_ind_op: '',
        cst: '',
        c_class_trib: '',
        ind_dest: 0,
        tp_oper: null,
        fin_nfse: 0,
      }
    })
    expect(spy).toHaveBeenCalledWith('/v1.0/services', {
        code: 'SVC-001', description: 'Consultoria', trib_nacional_code: '10101',
        unit: 'UN', value: '1000.00', iss: {trib_issqn: 1, tax_rate: '2.00'}, ibs_cbs:{
            c_ind_op: '',
            cst: '',
            c_class_trib: '',
            ind_dest: 0,
            tp_oper: null,
            fin_nfse: 0,
        }
    })
  })

  it('deleteService hits DELETE /v1.0/services/:id', async () => {
    const spy = vi.spyOn(priv, 'del').mockResolvedValue(undefined)
    await apiClient.deleteService('SERVICE_x')
    expect(spy).toHaveBeenCalledWith('/v1.0/services/SERVICE_x')
  })

  it('query keys stay distinct across orgs and ids', () => {
    expect(queryKeys.services.list('org-a')).not.toEqual(queryKeys.services.list('org-b'))
    expect(queryKeys.nfses.detail('DPS1')).not.toEqual(queryKeys.nfses.detail('DPS2'))
    expect(queryKeys.nfseConfig('org-a')).not.toEqual(queryKeys.nfseConfig('org-b'))
  })
})
