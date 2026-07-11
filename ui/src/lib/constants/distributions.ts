import type {NFeDistributionOut} from '@/lib/types/api'

const CTE_SCHEMA_LABELS: Record<string, string> = {
  resCTe: 'Resumo CT-e',
  procCTe: 'CT-e Completo',
  cteProc: 'CT-e Completo',
  resEvento: 'Resumo Evento',
  procEventoCTe: 'Evento CT-e',
}

const MDFE_SCHEMA_LABELS: Record<string, string> = {
  resMDFe: 'Resumo MDF-e',
  procMDFe: 'MDF-e Completo',
  resEvento: 'Resumo Evento',
  procEventoMDFe: 'Evento MDF-e',
}

export function cteSchemaLabel(item: NFeDistributionOut): string {
  return (item.schema_type && CTE_SCHEMA_LABELS[item.schema_type]) ? CTE_SCHEMA_LABELS[item.schema_type] : item.doc_schema
}

export function mdfeSchemaLabel(item: NFeDistributionOut): string {
  return (item.schema_type && MDFE_SCHEMA_LABELS[item.schema_type]) ? MDFE_SCHEMA_LABELS[item.schema_type] : item.doc_schema
}
