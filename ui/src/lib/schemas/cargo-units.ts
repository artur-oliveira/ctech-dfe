/**
 * Unidade de transporte (carreta, vagão) ou de carga (contêiner, pallet).
 * Espelha CargoUnitBody (api/internal/api/v1/dto.go). O rateio (`qtdRat`) não
 * fica aqui: é calculado dos pesos dos documentos a cada manifesto.
 */
import {z} from 'zod'

export const CARGO_UNIT_KINDS = ['transport', 'cargo'] as const
export type CargoUnitKind = typeof CARGO_UNIT_KINDS[number]

export const CARGO_UNIT_KIND_OPTIONS = [
  {value: 'transport', label: 'Unidade de transporte (carreta, vagão)'},
  {value: 'cargo', label: 'Unidade de carga (contêiner, pallet)'},
]

/** tpUnidTransp — tipo da unidade de transporte. */
export const TP_UNID_TRANSP_OPTIONS = [
  {value: '1', label: '1 – Rodoviário tração'},
  {value: '2', label: '2 – Rodoviário reboque'},
  {value: '3', label: '3 – Navio'},
  {value: '4', label: '4 – Balsa'},
  {value: '5', label: '5 – Aeronave'},
  {value: '6', label: '6 – Vagão'},
  {value: '7', label: '7 – Outros'},
]

/** tpUnidCarga — tipo da unidade de carga. */
export const TP_UNID_CARGA_OPTIONS = [
  {value: '1', label: '1 – Contêiner'},
  {value: '2', label: '2 – ULD'},
  {value: '3', label: '3 – Pallet'},
  {value: '4', label: '4 – Outros'},
]

export const cargoUnitSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  kind: z.enum(CARGO_UNIT_KINDS),
  tp_unid: z.enum(['1', '2', '3', '4', '5', '6', '7']),
  id_unid: z.string().min(1, 'Identificação obrigatória').max(20),
  /** Lacres fixos da unidade, digitados numa linha e separados por vírgula. */
  seals: z.string().optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  // O domínio de tpUnidCarga vai só até 4; usar 5–7 aqui geraria XML recusado.
  if (v.kind === 'cargo' && !['1', '2', '3', '4'].includes(v.tp_unid)) {
    ctx.addIssue({code: 'custom', path: ['tp_unid'], message: 'Unidade de carga aceita apenas os tipos 1 a 4'})
  }
})

export type CargoUnitFormData = z.infer<typeof cargoUnitSchema>
