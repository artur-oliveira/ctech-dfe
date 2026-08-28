import {act, renderHook, waitFor} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {apiClient} from '@/lib/api/client'
import {
  CNPJ_LOOKUP_SOURCE,
  mergeCnpjLookupResults,
  openCnpjOfficeToLookup,
  useCnpjLookup,
} from '@/lib/hooks/useCnpjLookup'
import type {LookupOrganizationOut, OpenCnpjOffice} from '@/lib/types/api'

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_00000000000191', state_federation: 'PI'}}),
}))

const OFFICE: OpenCnpjOffice = {
  taxId: '00000000000191',
  updated: '2026-08-20T12:00:00.000Z',
  alias: 'Empresa Teste',
  company: {name: 'EMPRESA DE TESTE LTDA', simples: {optant: true}, simei: {optant: false}},
  status: {text: 'Ativa'},
  address: {
    municipality: 2211001,
    street: 'Rua Pública',
    number: '10',
    district: 'Centro',
    city: 'Teresina',
    state: 'PI',
    zip: '64000-000',
  },
  phones: [{area: '86', number: '999999999'}],
  emails: [{address: 'fiscal@example.com'}],
  mainActivity: {id: 6201501, text: 'Desenvolvimento de programas'},
}

const SEFAZ: LookupOrganizationOut = {
  cpf_cnpj: OFFICE.taxId,
  name: 'EMPRESA DE TESTE LTDA',
  crt: '1',
  uf: 'PI',
  status: 'Habilitado',
  addresses: [{
    city_ibge_code: '2211001',
    street: 'Rua Fiscal',
    neighborhood: 'Centro',
    number: '10',
    city: 'Teresina',
    state_federation: 'PI',
    postal_code: '64000000',
    complement: null,
  }],
  state_registrations: [{uf: 'PI', state_registration: '123456'}],
}

beforeEach(() => vi.restoreAllMocks())

describe('consulta combinada de CNPJ', () => {
  it('normaliza cadastro, contato e regime do CNPJá', () => {
    const result = openCnpjOfficeToLookup(OFFICE, OFFICE.taxId)

    expect(result).toMatchObject({
      name: 'EMPRESA DE TESTE LTDA',
      fantasyName: 'Empresa Teste',
      crt: '1',
      cnae: '6201501',
      nfseSimpleOption: '3',
      contacts: {emails: ['fiscal@example.com'], phones: ['86999999999']},
    })
  })

  it('prioriza dados fiscais da SEFAZ e sinaliza divergências', () => {
    const publicResult = openCnpjOfficeToLookup(OFFICE, OFFICE.taxId)
    const result = mergeCnpjLookupResults(publicResult, SEFAZ)

    expect(result?.addresses[0].street).toBe('Rua Fiscal')
    expect(result?.state_registrations).toEqual(SEFAZ.state_registrations)
    expect(result?.sources).toEqual([CNPJ_LOOKUP_SOURCE.OPEN_CNPJ, CNPJ_LOOKUP_SOURCE.SEFAZ])
    expect(result?.conflicts).toEqual([
      {field: 'address', message: 'O endereço diverge entre CNPJá e SEFAZ.'},
    ])
  })

  it('aceita respostas SEFAZ legadas sem arrays de endereço e IE', () => {
    const publicResult = openCnpjOfficeToLookup(OFFICE, OFFICE.taxId)
    const legacySefaz = {...SEFAZ, addresses: undefined, state_registrations: undefined} as unknown as LookupOrganizationOut

    expect(() => mergeCnpjLookupResults(publicResult, legacySefaz)).not.toThrow()
    expect(mergeCnpjLookupResults(publicResult, legacySefaz)).toMatchObject({
      addresses: publicResult.addresses,
      state_registrations: [],
    })
  })

  it('integra as duas consultas em uma única ação', async () => {
    vi.spyOn(apiClient, 'lookupOpenCnpjOffice').mockResolvedValue(OFFICE)
    vi.spyOn(apiClient, 'lookupOrganization').mockResolvedValue(SEFAZ)
    const {result} = renderHook(() => useCnpjLookup())

    await act(async () => result.current.lookup(OFFICE.taxId, 'SP'))

    await waitFor(() => expect(result.current.state.status).toBe('found'))
    expect(apiClient.lookupOpenCnpjOffice).toHaveBeenCalledOnce()
    expect(apiClient.lookupOrganization).toHaveBeenCalledWith(OFFICE.taxId, 'PI')
    expect(result.current.state.result?.sources).toHaveLength(2)
  })
})
