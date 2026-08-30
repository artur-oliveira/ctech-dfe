/**
 * Documento referenciado da NFS-e. Espelha ReferenceDocumentBody
 * (api/internal/api/v1/dto.go).
 *
 * O mesmo cadastro alimenta `vDedRed/documentos` e `gReeRepRes/documentos`: o
 * leiaute pede formas diferentes do mesmo documento nos dois grupos, e
 * cadastrar duas vezes convidaria a divergência. `kind` decide qual grupo de
 * campos é obrigatório.
 */
import {z} from 'zod'

export const REFERENCE_DOCUMENT_KINDS = [
  {value: 'dfe', label: 'DF-e nacional (por chave de acesso)'},
  {value: 'nfse_municipal', label: 'NFS-e municipal anterior'},
  {value: 'nf_nfs', label: 'NF / NFS não eletrônica'},
  {value: 'doc_fiscal_outro', label: 'Outro documento fiscal'},
  {value: 'doc_nao_fiscal', label: 'Documento não fiscal'},
] as const

/** Domínio TSRTCTipoChaveDFe. NFS-e tem 50 dígitos; NF-e, 44. */
export const REFERENCE_DFE_KEY_TYPES = [
  {value: '1', label: 'NFS-e'},
  {value: '2', label: 'NF-e'},
  {value: '3', label: 'CT-e'},
  {value: '9', label: 'Outro'},
] as const

const DFE_KEY_LENGTH: Record<string, number> = {'1': 50, '2': 44}

export const referenceDocumentSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  kind: z.enum(['dfe', 'nfse_municipal', 'nf_nfs', 'doc_fiscal_outro', 'doc_nao_fiscal']),
  issued_at: z.string().min(1, 'Data de emissão obrigatória'),
  competence_at: z.string().optional().or(z.literal('')),
  description: z.string().max(150).optional().or(z.literal('')),
  supplier_person_id: z.string().optional().or(z.literal('')),

  tipo_chave_dfe: z.string().optional().or(z.literal('')),
  chave_dfe: z.string().optional().or(z.literal('')),
  c_mun_nfse_mun: z.string().optional().or(z.literal('')),
  n_nfse_mun: z.string().optional().or(z.literal('')),
  c_verif_nfse_mun: z.string().optional().or(z.literal('')),
  n_nfs: z.string().optional().or(z.literal('')),
  mod_nfs: z.string().optional().or(z.literal('')),
  serie_nfs: z.string().optional().or(z.literal('')),
  n_doc_fiscal: z.string().optional().or(z.literal('')),
  c_mun_doc_fiscal: z.string().optional().or(z.literal('')),
  x_doc_fiscal: z.string().optional().or(z.literal('')),
  n_doc: z.string().optional().or(z.literal('')),
  x_doc: z.string().optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  const require = (field: keyof typeof v, message: string, pattern?: RegExp) => {
    const value = (v[field] ?? '') as string
    if (!value) {
      ctx.addIssue({code: 'custom', path: [field], message})
      return
    }
    if (pattern && !pattern.test(value)) {
      ctx.addIssue({code: 'custom', path: [field], message})
    }
  }

  switch (v.kind) {
    case 'dfe': {
      require('tipo_chave_dfe', 'Escolha o tipo do documento')
      require('chave_dfe', 'Chave de acesso obrigatória (somente dígitos)', /^\d+$/)
      const want = DFE_KEY_LENGTH[v.tipo_chave_dfe ?? '']
      if (want && v.chave_dfe && v.chave_dfe.length !== want) {
        ctx.addIssue({code: 'custom', path: ['chave_dfe'], message: `A chave deve ter ${want} dígitos para este tipo`})
      }
      break
    }
    case 'nfse_municipal':
      require('c_mun_nfse_mun', 'Escolha o município', /^\d{7}$/)
      require('n_nfse_mun', 'Número deve ter 15 dígitos', /^\d{15}$/)
      require('c_verif_nfse_mun', 'Código de verificação obrigatório (até 9 caracteres)', /^[A-Za-z0-9]{1,9}$/)
      break
    case 'nf_nfs':
      require('n_nfs', 'Número deve ter 7 dígitos', /^\d{7}$/)
      require('mod_nfs', 'Modelo deve ter 15 dígitos', /^\d{15}$/)
      require('serie_nfs', 'Série obrigatória (até 15 caracteres)', /^[A-Za-z0-9]{1,15}$/)
      break
    case 'doc_fiscal_outro':
      require('n_doc_fiscal', 'Número do documento obrigatório')
      break
    case 'doc_nao_fiscal':
      require('n_doc', 'Número do documento obrigatório')
      break
  }

  // Mesma regra do backend: a competência não pode preceder a emissão.
  if (v.competence_at && v.issued_at && v.competence_at < v.issued_at) {
    ctx.addIssue({code: 'custom', path: ['competence_at'], message: 'A competência não pode ser anterior à emissão'})
  }
})

export type ReferenceDocumentFormData = z.infer<typeof referenceDocumentSchema>
