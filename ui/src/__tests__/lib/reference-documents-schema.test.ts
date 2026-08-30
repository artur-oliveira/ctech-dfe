import {describe, expect, it} from 'vitest'
import {referenceDocumentSchema} from '@/lib/schemas/reference-documents'

const base = {
  name: 'NF-e do fornecedor',
  kind: 'dfe' as const,
  issued_at: '2026-08-01',
  competence_at: '', description: '', supplier_person_id: '',
  tipo_chave_dfe: '2', chave_dfe: '3'.repeat(44),
  c_mun_nfse_mun: '', n_nfse_mun: '', c_verif_nfse_mun: '',
  n_nfs: '', mod_nfs: '', serie_nfs: '',
  n_doc_fiscal: '', c_mun_doc_fiscal: '', x_doc_fiscal: '',
  n_doc: '', x_doc: '',
}

describe('referenceDocumentSchema', () => {
  it('aceita um DF-e nacional com chave do tamanho certo', () => {
    expect(referenceDocumentSchema.safeParse(base).success).toBe(true)
  })

  it('exige 50 dígitos quando a chave é de NFS-e', () => {
    // chNFSe tem 50 dígitos e chNFe tem 44: tipo e comprimento têm de concordar,
    // ou a dedução aponta para um documento que não existe.
    const result = referenceDocumentSchema.safeParse({...base, tipo_chave_dfe: '1'})
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[0] === 'chave_dfe')).toBe(true)
  })

  it('cobra apenas os campos da família escolhida', () => {
    const result = referenceDocumentSchema.safeParse({
      ...base, kind: 'nf_nfs', tipo_chave_dfe: '', chave_dfe: '',
      n_nfs: '1234567', mod_nfs: '1'.repeat(15), serie_nfs: 'A1',
    })
    expect(result.success).toBe(true)
  })

  it('recusa a família municipal sem código de verificação', () => {
    const result = referenceDocumentSchema.safeParse({
      ...base, kind: 'nfse_municipal', tipo_chave_dfe: '', chave_dfe: '',
      c_mun_nfse_mun: '2211001', n_nfse_mun: '1'.repeat(15), c_verif_nfse_mun: '',
    })
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[0] === 'c_verif_nfse_mun')).toBe(true)
  })

  it('recusa competência anterior à emissão', () => {
    const result = referenceDocumentSchema.safeParse({...base, competence_at: '2026-07-01'})
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[0] === 'competence_at')).toBe(true)
  })
})
