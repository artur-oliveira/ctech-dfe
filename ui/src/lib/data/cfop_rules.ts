export const CFOP_FISCAL_HINTS: Record<string, { label: string; icms_cst?: string, csosn?: string }> = {
  '5910': {
    label: 'Remessa em bonificação, doação ou brinde → ICMS suspenso / CSOSN Não tributado',
    icms_cst: '50',
    csosn: '400'
  },
  '6910': {
    label: 'Remessa em bonificação, doação ou brinde → ICMS suspenso / CSOSN Não tributado',
    icms_cst: '50',
    csosn: '400'
  },
  '5911': {label: 'Remessa de amostra grátis → ICMS suspenso / CSOSN Não tributado', icms_cst: '50', csosn: '400'},
  '6911': {label: 'Remessa de amostra grátis → ICMS suspenso / CSOSN Não tributado', icms_cst: '50', csosn: '400'},
  '5920': {label: 'Remessa de vasilhame → ICMS isento / CSOSN Não tributado', icms_cst: '40', csosn: '400'},
  '6920': {label: 'Remessa de vasilhame → ICMS isento / CSOSN Não tributado', icms_cst: '40', csosn: '400'},
  '5915': {label: 'Remessa para conserto → ICMS suspenso / CSOSN Não tributado', icms_cst: '50', csosn: '400'},
  '6915': {label: 'Remessa para conserto → ICMS suspenso / CSOSN Não tributado', icms_cst: '50', csosn: '400'},
  '5916': {label: 'Retorno de conserto → ICMS isento / CSOSN Não tributado', icms_cst: '40', csosn: '400'},
  '6916': {label: 'Retorno de conserto → ICMS isento / CSOSN Não tributado', icms_cst: '40', csosn: '400'},
}

export function getCfopHint(cfop: string) {
  return CFOP_FISCAL_HINTS[cfop] ?? null
}
