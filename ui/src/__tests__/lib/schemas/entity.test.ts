import {describe, it, expect} from 'vitest'
import {CRT_NONE_VALUE, entitySchema, organizationSchema, type EntityFormData} from '@/lib/schemas/entity'

// Valid PF (pessoa física) form payload. CPF 05213732399 is a real-valid CPF.
const basePF: EntityFormData = {
  tipo: 'pf',
  cpf_or_cnpj: '05213732399',
  name: 'LARISSA OLIVEIRA CARVALHO',
  description: '',
  roles: [],
  person: {
    fantasy_name: '',
    crt: '4',
    state_registrations: [],
    addresses: [{
      city_ibge_code: '2211001',
      street: 'Rua Assis Davis',
      neighborhood: 'Monte Castelo',
      number: '7',
      city: 'Teresina',
      state_federation: 'PI',
      postal_code: '64016275',
      complement: '',
    }],
    contacts: {emails: ['carvalholarissa_@hotmail.com'], phones: ['86995373408']},
    nfse: {im: '', op_simp_nac: '', reg_ap_trib_sn: '', reg_esp_trib: ''},
    bank: {pix_key: '', bank_code: '', branch_code: '', cnpj_ipef: ''},
    freight_retention: {v_serv: '', v_bc_ret: '', p_icms_ret: '', cfop: '', c_mun_fg: ''},
  },
}

describe('entitySchema — PF edit', () => {
  it('valida PF com fantasy_name vazio', () => {
    expect(entitySchema.safeParse(basePF).success).toBe(true)
  })

  // Regression: backend returns fantasy_name=null for PF. fromPersonOut must coerce
  // null -> '' because the schema rejects null, and the field is hidden for PF, so the
  // validation error would be invisible and the form would silently refuse to submit.
  it('rejeita fantasy_name null (deve ser coagido para "" no transform)', () => {
    const withNull = {...basePF, person: {...basePF.person, fantasy_name: null}}
    expect(entitySchema.safeParse(withNull).success).toBe(false)
  })

  // CRT é opcional para PF ("Não especificar").
  it('valida PF sem CRT (undefined)', () => {
    const noCrt = {...basePF, person: {...basePF.person, crt: undefined}}
    expect(entitySchema.safeParse(noCrt).success).toBe(true)
  })

  it('valida PF com sentinela "Não especificar"', () => {
    const noneCrt = {...basePF, person: {...basePF.person, crt: CRT_NONE_VALUE}}
    expect(entitySchema.safeParse(noneCrt).success).toBe(true)
  })

  it('exige CRT para PJ (rejeita undefined e sentinela)', () => {
    const pj: EntityFormData = {
      ...basePF,
      tipo: 'pj',
      cpf_or_cnpj: '11647612000197',
      person: {...basePF.person, fantasy_name: 'Loja', crt: undefined},
    }
    expect(entitySchema.safeParse(pj).success).toBe(false)
    expect(entitySchema.safeParse({...pj, person: {...pj.person, crt: CRT_NONE_VALUE}}).success).toBe(false)
  })
})

describe('organizationSchema — IE obrigatória para PJ, UFs duplicadas rejeitadas', () => {
  const pj: EntityFormData = {
    ...basePF,
    tipo: 'pj',
    cpf_or_cnpj: '11647612000197',
    person: {...basePF.person, fantasy_name: 'Loja', crt: '1', state_registrations: []},
  }

  // Empresa de serviço é contribuinte do município, emite NFS-e e não tem
  // inscrição estadual nenhuma para digitar. Exigir uma tornava o cadastro
  // impossível de enviar.
  it('organizationSchema aceita PJ sem inscrição estadual', () => {
    expect(organizationSchema.safeParse(pj).success).toBe(true)
  })

  it('organizationSchema aceita PJ com ao menos uma inscrição estadual', () => {
    const withIE = {...pj, person: {...pj.person, state_registrations: [{uf: 'PI' as const, state_registration: '123456'}]}}
    expect(organizationSchema.safeParse(withIE).success).toBe(true)
  })

  it('organizationSchema rejeita UF duplicada', () => {
    const dup = {...pj, person: {...pj.person, state_registrations: [
      {uf: 'PI' as const, state_registration: '111'},
      {uf: 'PI' as const, state_registration: '222'},
    ]}}
    expect(organizationSchema.safeParse(dup).success).toBe(false)
  })

  it('entitySchema (pessoas) aceita PJ sem inscrição estadual', () => {
    expect(entitySchema.safeParse(pj).success).toBe(true)
  })

  it('organizationSchema aceita PF sem inscrição estadual (regra é só para CNPJ)', () => {
    expect(organizationSchema.safeParse(basePF).success).toBe(true)
  })
})

describe('entitySchema — grupo NFS-e', () => {
  it('exige regime de apuração quando optante ME/EPP (op_simp_nac = 3)', () => {
    const data = {...basePF, person: {...basePF.person, nfse: {im: '123456', op_simp_nac: '3' as const, reg_ap_trib_sn: '' as const, reg_esp_trib: ''  as const}}}
    expect(entitySchema.safeParse(data).success).toBe(false)
  })

  it('aceita inscrição municipal com regime simples informado', () => {
    const data = {...basePF, person: {...basePF.person, nfse: {im: '123456', op_simp_nac: '1' as const, reg_ap_trib_sn: '' as const, reg_esp_trib: '0' as const}}}
    expect(entitySchema.safeParse(data).success).toBe(true)
  })
})

describe('entitySchema — CNAE e retenção do frete', () => {
  const pathsOf = (data: EntityFormData): string[] => {
    const result = entitySchema.safeParse(data)
    return result.success ? [] : result.error.issues.map((i) => i.path.join('.'))
  }

  it('aceita CNAE existente e recusa código inexistente', () => {
    const valido = {...basePF, person: {...basePF.person, cnae: '0111301'}}
    expect(entitySchema.safeParse(valido).success).toBe(true)
    const invalido = {...basePF, person: {...basePF.person, cnae: '9999999'}}
    expect(pathsOf(invalido)).toContain('person.cnae')
  })

  it('retenção do frete é grupo: metade preenchida é recusada', () => {
    const meio = {
      ...basePF,
      person: {
        ...basePF.person,
        freight_retention: {v_serv: '150.00', v_bc_ret: '', p_icms_ret: '', cfop: '', c_mun_fg: ''},
      },
    }
    const paths = pathsOf(meio)
    expect(paths).toEqual(expect.arrayContaining([
      'person.freight_retention.v_bc_ret',
      'person.freight_retention.p_icms_ret',
      'person.freight_retention.cfop',
      'person.freight_retention.c_mun_fg',
    ]))
  })

  it('aceita a retenção do frete completa e o grupo inteiro vazio', () => {
    const completo = {
      ...basePF,
      person: {
        ...basePF.person,
        freight_retention: {
          v_serv: '150.00', v_bc_ret: '150.00', p_icms_ret: '12.0000',
          cfop: '5353', c_mun_fg: '2211001',
        },
      },
    }
    expect(entitySchema.safeParse(completo).success).toBe(true)
    expect(entitySchema.safeParse(basePF).success).toBe(true)
  })

  it('recusa valor de serviço fora do formato decimal', () => {
    const errado = {
      ...basePF,
      person: {
        ...basePF.person,
        freight_retention: {
          v_serv: 'cento e cinquenta', v_bc_ret: '150.00', p_icms_ret: '12.0000',
          cfop: '5353', c_mun_fg: '2211001',
        },
      },
    }
    expect(pathsOf(errado)).toContain('person.freight_retention.v_serv')
  })
})
