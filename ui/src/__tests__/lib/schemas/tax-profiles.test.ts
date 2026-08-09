import {describe, expect, it} from 'vitest'
import {taxProfileSchema} from '@/lib/schemas/tax-profiles'

const base = {
  name: 'Venda de mercadoria',
  description: '',
  cfops: ['5102', '6102'],
  pis: '01',
  cofins: '01',
  ibs_cbs_cst: '000',
  ibs_cbs_class_trib: '000001',
  ibs_uf_aliq: '8.0000',
  ibs_mun_aliq: '1.0000',
  cbs_aliq: '9.0000',
}

describe('perfil fiscal', () => {
  it('aceita um perfil completo cobrindo vários CFOPs', () => {
    const parsed = taxProfileSchema.parse(base)
    expect(parsed.cfops).toEqual(['5102', '6102'])
  })

  it('exige ao menos um CFOP — perfil sem CFOP não se aplica a nada', () => {
    expect(taxProfileSchema.safeParse({...base, cfops: []}).success).toBe(false)
  })

  it('rejeita CFOP malformado', () => {
    expect(taxProfileSchema.safeParse({...base, cfops: ['51']}).success).toBe(false)
  })

  it('exige nome com ao menos 2 caracteres', () => {
    expect(taxProfileSchema.safeParse({...base, name: 'A'}).success).toBe(false)
  })

  // O perfil carrega tratamento completo, exatamente como uma linha de
  // cfop_config — o que muda é só não ter CFOP preso a ele. IBS/CBS é
  // opcional (grupo tudo-ou-nada validado pelo backend) desde que a vigência
  // obrigatória ainda não cobre todo mundo.
  it('aceita o bloco IBS/CBS totalmente ausente', () => {
    const {ibs_cbs_cst, ibs_cbs_class_trib, ibs_uf_aliq, ibs_mun_aliq, cbs_aliq, ...rest} = base
    void ibs_cbs_cst; void ibs_cbs_class_trib; void ibs_uf_aliq; void ibs_mun_aliq; void cbs_aliq
    expect(taxProfileSchema.safeParse(rest).success).toBe(true)
  })

  it('não tem campo cfop — os CFOPs são uma lista à parte', () => {
    expect('cfop' in taxProfileSchema.shape).toBe(false)
    expect('cfops' in taxProfileSchema.shape).toBe(true)
  })
})
