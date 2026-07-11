import {describe, it, expect} from 'vitest'
import {CRT_NONE_VALUE, entitySchema, type EntityFormData} from '@/lib/schemas/entity'

// Valid PF (pessoa física) form payload. CPF 05213732399 is a real-valid CPF.
const basePF: EntityFormData = {
  tipo: 'pf',
  cpf_or_cnpj: '05213732399',
  name: 'LARISSA OLIVEIRA CARVALHO',
  description: '',
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
