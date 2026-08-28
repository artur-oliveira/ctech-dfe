/**
 * Tabelas de domínio dos grupos de nicho da NF-e (infNFe/cana e
 * infNFe/agropecuario). Todos os valores vêm do próprio XSD
 * (leiauteNFe_v4.00), não de convenção nossa.
 */
import type {NfeAgroGuiaIn} from '@/lib/types/api'

/** Limites do leiaute — o XSD trava, e o botão de adicionar respeita. */
export const MAX_CANA_DELIVERIES = 31
export const MAX_CANA_DEDUCOES = 10
export const MAX_AGRO_RECEITUARIOS = 20

/**
 * Dias do mês aceitos em cana/forDia/@dia. O padrão do XSD é sem zero à
 * esquerda (`[1-9]|[1][0-9]|[2][0-9]|[3][0-1]`), então "01" é rejeição.
 */
export const CANA_DIA_OPTIONS = Array.from({length: MAX_CANA_DELIVERIES}, (_, i) => {
  const dia = String(i + 1)
  return {value: dia, label: dia}
})

/** Tipos de guia de trânsito (agropecuario/guiaTransito/tpGuia). */
export const AGRO_TP_GUIA_OPTIONS: { value: NfeAgroGuiaIn['tp_guia']; label: string }[] = [
  {value: '1', label: '1 – GTA (Guia de Trânsito Animal)'},
  {value: '2', label: '2 – TTA (Termo de Trânsito Animal)'},
  {value: '3', label: '3 – DTA (Documento de Trânsito Animal)'},
  {value: '4', label: '4 – ATV (Autorização de Trânsito Vegetal)'},
  {value: '5', label: '5 – PTV (Permissão de Trânsito Vegetal)'},
  {value: '6', label: '6 – GTV (Guia de Trânsito Vegetal)'},
  {value: '7', label: '7 – Guia Florestal (DOF, SisFlora, SIAM)'},
]

/** Modo do grupo agropecuario — o XSD é um choice, então o seletor é exclusivo. */
export type AgroMode = 'none' | 'defensivo' | 'guia'

export const AGRO_MODE_OPTIONS: { value: AgroMode; label: string }[] = [
  {value: 'none', label: 'Não se aplica'},
  {value: 'defensivo', label: 'Receituário de defensivo'},
  {value: 'guia', label: 'Guia de trânsito'},
]

const MONTH_NAMES = [
  'Janeiro', 'Fevereiro', 'Março', 'Abril', 'Maio', 'Junho',
  'Julho', 'Agosto', 'Setembro', 'Outubro', 'Novembro', 'Dezembro',
]

/**
 * Meses de referência de cana/ref, no formato MM/AAAA do XSD. Um select
 * elimina a única forma de errar o campo: escrever "9/26" ou "2026-09".
 *
 * A janela é o mês atual e os 12 anteriores — registro de cana é mensal e
 * retroativo, nunca futuro.
 */
export function canaRefOptions(now: Date = new Date()): { value: string; label: string }[] {
  const out: { value: string; label: string }[] = []
  for (let back = 0; back <= 12; back++) {
    const date = new Date(now.getFullYear(), now.getMonth() - back, 1)
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const year = date.getFullYear()
    out.push({value: `${month}/${year}`, label: `${MONTH_NAMES[date.getMonth()]}/${year}`})
  }
  return out
}
