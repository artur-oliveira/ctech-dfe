/**
 * Declaração de importação (NF-e prod/DI). Espelha ImportDeclarationBody
 * (api/internal/api/v1/dto.go). Uma DI cobre várias notas e vários itens: na
 * emissão o item só aponta qual adição o representa, e nAdicao/nSeqAdic saem
 * desse vínculo.
 */
import {z} from 'zod'

/** tpViaTransp — via de transporte internacional. */
export const TP_VIA_TRANSP_OPTIONS = [
  {value: '01', label: '01 – Marítima'},
  {value: '02', label: '02 – Fluvial'},
  {value: '03', label: '03 – Lacustre'},
  {value: '04', label: '04 – Aérea'},
  {value: '05', label: '05 – Postal'},
  {value: '06', label: '06 – Ferroviária'},
  {value: '07', label: '07 – Rodoviária'},
  {value: '08', label: '08 – Conduto / rede transmissão'},
  {value: '09', label: '09 – Meios próprios'},
  {value: '10', label: '10 – Entrada / saída ficta'},
  {value: '11', label: '11 – Courier'},
  {value: '12', label: '12 – Handcarry'},
]

/** tpIntermedio — forma de intermediação da importação. */
export const TP_INTERMEDIO_OPTIONS = [
  {value: '1', label: '1 – Importação por conta própria'},
  {value: '2', label: '2 – Importação por conta e ordem'},
  {value: '3', label: '3 – Importação por encomenda'},
]

/** A via marítima é a única que exige o AFRMM. */
export const TP_VIA_TRANSP_MARITIMA = '01'

export const importAdditionSchema = z.object({
  n_adicao: z.string().regex(/^\d{1,3}$/, 'Número da adição: até 3 dígitos'),
  c_fabricante: z.string().min(1, 'Fabricante obrigatório').max(60),
  v_desc_di: z.string().optional().or(z.literal('')),
  n_draw: z.string().max(20).optional().or(z.literal('')),
})

export const importDeclarationSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  n_di: z.string().min(1, 'Número da DI obrigatório').max(15),
  d_di: z.string().min(1, 'Data de registro obrigatória'),
  x_loc_desemb: z.string().min(1, 'Local de desembaraço obrigatório').max(60),
  uf_desemb: z.string().length(2, 'UF inválida'),
  d_desemb: z.string().min(1, 'Data de desembaraço obrigatória'),
  tp_via_transp: z.string().regex(/^\d{2}$/, 'Via de transporte inválida'),
  v_afrmm: z.string().optional().or(z.literal('')),
  tp_intermedio: z.enum(['1', '2', '3']),
  cnpj: z.string().optional().or(z.literal('')),
  uf_terceiro: z.string().optional().or(z.literal('')),
  c_exportador: z.string().min(1, 'Código do exportador obrigatório').max(60),
  additions: z.array(importAdditionSchema).min(1, 'Ao menos uma adição'),
}).superRefine((v, ctx) => {
  // Mesma regra do backend: sem AFRMM, a DI marítima seria recusada na SEFAZ.
  if (v.tp_via_transp === TP_VIA_TRANSP_MARITIMA && !v.v_afrmm) {
    ctx.addIssue({code: 'custom', path: ['v_afrmm'], message: 'AFRMM é obrigatório na via marítima'})
  }
})

export type ImportDeclarationFormData = z.infer<typeof importDeclarationSchema>
