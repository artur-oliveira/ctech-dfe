import {z} from 'zod'

/**
 * Tipos de referência de ide/NFref (leiauteNFe_v4.00). Espelha os `refKind*`
 * de `api/internal/services/nfes/references.go`.
 */
export const NFE_REF_KINDS = ['nfe', 'nfesig', 'nf', 'nfp', 'cte', 'ecf'] as const
export type NfeRefKind = (typeof NFE_REF_KINDS)[number]

/** Rótulo de cada tipo, para o seletor de documento externo. */
export const NFE_REF_KIND_LABELS: Record<NfeRefKind, string> = {
  nfe: 'NF-e / NFC-e (chave de 44 dígitos)',
  nfesig: 'NF-e com destinatário em sigilo',
  cte: 'CT-e (chave de 44 dígitos)',
  nf: 'NF modelo 1/1A (papel)',
  nfp: 'NF de produtor rural',
  ecf: 'Cupom fiscal (ECF)',
}

/** Tipos cuja identificação é só a chave de acesso. */
export const NFE_REF_KEY_KINDS: readonly NfeRefKind[] = ['nfe', 'nfesig', 'cte']

/**
 * Espelha `NfeRefBody` do backend. Ou `nfe_id` (uma nota da própria base, de
 * onde chave e tipo são derivados), ou os campos do documento externo.
 */
export const nfeRefSchema = z
  .object({
    nfe_id: z.string().optional(),
    kind: z.enum(NFE_REF_KINDS).optional(),
    access_key: z.string().regex(/^\d{44}$/, 'A chave tem 44 dígitos').optional(),
    c_uf: z.string().regex(/^\d{2}$/, 'cUF tem 2 dígitos').optional(),
    aamm: z.string().regex(/^\d{4}$/, 'AAMM tem 4 dígitos').optional(),
    cnpj: z.string().optional(),
    cpf: z.string().optional(),
    ie: z.string().max(14).optional(),
    mod: z.string().max(2).optional(),
    serie: z.string().regex(/^\d{1,3}$/).optional(),
    n_nf: z.string().regex(/^\d{1,9}$/).optional(),
    n_ecf: z.string().regex(/^\d{1,3}$/).optional(),
    n_coo: z.string().regex(/^\d{1,6}$/).optional(),
  })
  .superRefine((v, ctx) => {
    if (v.nfe_id) return
    if (!v.kind) {
      ctx.addIssue({code: 'custom', path: ['kind'], message: 'Escolha uma nota da base ou o tipo do documento externo'})
      return
    }
    if (NFE_REF_KEY_KINDS.includes(v.kind) && !v.access_key) {
      ctx.addIssue({code: 'custom', path: ['access_key'], message: 'Informe a chave de acesso'})
    }
    if (v.kind === 'nf' && !(v.c_uf && v.aamm && v.cnpj && v.mod && v.serie && v.n_nf)) {
      ctx.addIssue({code: 'custom', path: ['n_nf'], message: 'NF modelo 1/1A exige cUF, AAMM, CNPJ, modelo, série e número'})
    }
    if (v.kind === 'nfp' && !(v.c_uf && v.aamm && (v.cnpj || v.cpf) && v.ie && v.mod && v.serie && v.n_nf)) {
      ctx.addIssue({code: 'custom', path: ['n_nf'], message: 'NF de produtor exige cUF, AAMM, CNPJ ou CPF, IE, modelo, série e número'})
    }
    if (v.kind === 'ecf' && !(v.mod && v.n_ecf && v.n_coo)) {
      ctx.addIssue({code: 'custom', path: ['n_coo'], message: 'Cupom fiscal exige modelo, nECF e nCOO'})
    }
  })

export type NfeRefFormData = z.infer<typeof nfeRefSchema>

/** finNFe 2 (complementar), 3 (ajuste) e 4 (devolução) exigem ao menos um NFref. */
export function finNFeRequiresRef(finNFe: string | null | undefined): boolean {
  return finNFe === '2' || finNFe === '3' || finNFe === '4'
}
