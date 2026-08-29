import type {AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig} from 'axios'
import {afterEach, describe, expect, it, vi} from 'vitest'
import {apiClient, ORG_HEADER} from '@/lib/api/client'
import {STORAGE_KEY_ORG} from '@/lib/constants/storage'
import type {OpenCnpjOffice} from '@/lib/types/api'

const OFFICE: OpenCnpjOffice = {
  taxId: '00000000000191',
  company: {name: 'EMPRESA DE TESTE', simples: {optant: true}},
}

function response(config: InternalAxiosRequestConfig): AxiosResponse<OpenCnpjOffice> {
  return {data: OFFICE, status: 200, statusText: 'OK', headers: {}, config}
}

afterEach(() => {
  apiClient.setToken(null)
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('ApiClient.lookupOpenCnpjOffice', () => {
  it('isola credenciais e deduplica consultas simultâneas ao serviço público', async () => {
    const adapter = vi.fn<AxiosAdapter>(async (config) => response(config))
    apiClient.setOpenCnpjAdapter(adapter)
    apiClient.setToken('token-que-nao-deve-sair')
    localStorage.setItem(STORAGE_KEY_ORG, JSON.stringify({pk: 'CNPJ_00000000000191'}))

    const [first, second] = await Promise.all([
      apiClient.lookupOpenCnpjOffice(OFFICE.taxId),
      apiClient.lookupOpenCnpjOffice(OFFICE.taxId),
    ])

    expect(first).toEqual(OFFICE)
    expect(second).toEqual(OFFICE)
    expect(adapter).toHaveBeenCalledTimes(1)
    const request = adapter.mock.calls[0][0]
    expect(request.baseURL).toBe('https://open.cnpja.com')
    expect(request.headers?.get?.('Authorization')).toBeUndefined()
    expect(request.headers?.get?.(ORG_HEADER)).toBeUndefined()
  })
})
