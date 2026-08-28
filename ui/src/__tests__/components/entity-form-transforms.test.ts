import {describe, expect, it} from 'vitest'
import {organizationFormToApi} from '@/components/organizations/OrganizationForm'
import {personFormToApi} from '@/components/persons/PersonForm'
import type {EntityFormData} from '@/lib/schemas/entity'

const formData: EntityFormData = {
  tipo: 'pj',
  cpf_or_cnpj: '11647612000197',
  id_estrangeiro: '',
  name: 'EMPRESA DE TESTE LTDA',
  description: '',
  roles: ['carrier'],
  person: {
    fantasy_name: 'Empresa de teste',
    crt: '1',
    cnae: '6201501',
    isuf_emit: '123456789',
    state_registrations: [{uf: 'PI', state_registration: '123456'}],
    addresses: [{
      city_ibge_code: '2211001',
      street: 'Rua de Teste',
      neighborhood: 'Centro',
      number: '10',
      city: 'Teresina',
      state_federation: 'PI',
      postal_code: '64000-000',
      complement: '',
    }],
    contacts: {emails: ['fiscal@example.com'], phones: ['86999999999']},
    nfse: {im: '1234', op_simp_nac: '3', reg_ap_trib_sn: '1', reg_esp_trib: '0'},
    bank: {pix_key: 'financeiro@example.com', bank_code: '', branch_code: '', cnpj_ipef: ''},
    freight_retention: {
      v_serv: '100.00',
      v_bc_ret: '100.00',
      p_icms_ret: '12.00',
      cfop: '5353',
      c_mun_fg: '2211001',
    },
  },
}

describe('cadastro compartilhado — transformação para a API', () => {
  it('preserva dados bancários, retenção de frete, CNAE e Suframa da pessoa', () => {
    const payload = personFormToApi(formData)

    expect(payload.person).toMatchObject({
      cnae: '6201501',
      isuf_emit: '123456789',
      bank: {pix_key: 'financeiro@example.com'},
      freight_retention: {p_icms_ret: '12.00', cfop: '5353'},
    })
  })

  it('preserva CNAE e Suframa da organização', () => {
    const payload = organizationFormToApi(formData)

    expect(payload.person).toMatchObject({
      cnae: '6201501',
      isuf_emit: '123456789',
    })
  })

  it('converte grupos opcionais vazios da pessoa para null', () => {
    const payload = personFormToApi({
      ...formData,
      person: {
        ...formData.person,
        bank: {pix_key: '', bank_code: '', branch_code: '', cnpj_ipef: ''},
        freight_retention: {v_serv: '', v_bc_ret: '', p_icms_ret: '', cfop: '', c_mun_fg: ''},
      },
    })

    expect(payload.person.bank).toBeNull()
    expect(payload.person.freight_retention).toBeNull()
  })
})
