import {describe, expect, it} from 'vitest'
import {
  entitySchema,
  PERSON_ROLE_DEFAULT,
  PERSON_ROLE_LABELS,
  PERSON_ROLE_OPTIONS,
  PERSON_ROLES,
} from '@/lib/schemas/entity'

const baseEntity = {
  tipo: 'pj' as const,
  cpf_or_cnpj: '11222333000181',
  name: 'TRANSPORTES ACME',
  description: '',
  person: {
    fantasy_name: '',
    crt: '1' as const,
    state_registrations: [],
    addresses: [{
      city_ibge_code: '3550308',
      street: 'Av. Paulista',
      neighborhood: 'Bela Vista',
      number: '1000',
      city: 'São Paulo',
      state_federation: 'SP' as const,
      postal_code: '01310100',
      complement: '',
    }],
    contacts: {emails: [], phones: []},
    nfse: {im: '', op_simp_nac: '' as const, reg_ap_trib_sn: '' as const, reg_esp_trib: '' as const},
  },
}

describe('papéis de pessoa', () => {
  it('aceita ausência de papéis e assume lista vazia', () => {
    const parsed = entitySchema.parse(baseEntity)
    expect(parsed.roles).toEqual([])
  })

  it('aceita múltiplos papéis ao mesmo tempo', () => {
    const parsed = entitySchema.parse({...baseEntity, roles: ['customer', 'carrier']})
    expect(parsed.roles).toEqual(['customer', 'carrier'])
  })

  it('aceita todos os papéis conhecidos', () => {
    const parsed = entitySchema.parse({...baseEntity, roles: [...PERSON_ROLES]})
    expect(parsed.roles).toHaveLength(PERSON_ROLES.length)
  })

  it('rejeita papel desconhecido', () => {
    expect(entitySchema.safeParse({...baseEntity, roles: ['shareholder']}).success).toBe(false)
  })

  it('todo papel tem rótulo e opção de seleção', () => {
    expect(PERSON_ROLE_OPTIONS.map((o) => o.value)).toEqual([...PERSON_ROLES])
    PERSON_ROLES.forEach((r) => expect(PERSON_ROLE_LABELS[r]).toBeTruthy())
  })

  it('o papel padrão de um cadastro novo é um papel válido', () => {
    expect(PERSON_ROLES).toContain(PERSON_ROLE_DEFAULT)
  })
})
