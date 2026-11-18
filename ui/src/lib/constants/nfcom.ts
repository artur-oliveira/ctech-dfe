// Opções fechadas da NFCom (modelo 62), espelho do leiaute NT 2026.005 e do DTO da API (nfcoms/dto.go).
// A UI mostra seletores; texto livre só onde o campo é livre de verdade.

export interface NfcomOption {
  value: string
  label: string
}

/** cClass: 7 dígitos (tabela de classificação do MOC, item 7.8). */
export const NFCOM_CLASS_CODE_LENGTH = 7;

/** indIEDest. */
export const NFCOM_IE_INDICATORS: readonly NfcomOption[] = [
  {value: '1', label: 'Contribuinte do ICMS'},
  {value: '2', label: 'Contribuinte isento de inscrição'},
  {value: '9', label: 'Não contribuinte'},
];

/** tpAssinante. */
export const NFCOM_SUBSCRIBER_TYPES: readonly NfcomOption[] = [
  {value: '1', label: 'Comercial'},
  {value: '2', label: 'Industrial'},
  {value: '3', label: 'Residencial'},
  {value: '4', label: 'Produtor rural'},
  {value: '5', label: 'Órgão público estadual'},
  {value: '6', label: 'Prestador de telecomunicação'},
  {value: '7', label: 'Órgão público municipal'},
  {value: '8', label: 'Órgão público federal'},
  {value: '99', label: 'Outros'},
];

/** tpServUtil. */
export const NFCOM_SERVICE_TYPES: readonly NfcomOption[] = [
  {value: '1', label: 'Telefonia'},
  {value: '2', label: 'Comunicação de dados'},
  {value: '3', label: 'TV por assinatura'},
  {value: '4', label: 'Provimento de acesso à internet'},
  {value: '5', label: 'Multimídia'},
  {value: '6', label: 'Outros'},
  {value: '7', label: 'Vários'},
];

/** uMed. */
export const NFCOM_UNIT_OPTIONS: readonly NfcomOption[] = [
  {value: '1', label: 'Minuto'},
  {value: '2', label: 'MB'},
  {value: '3', label: 'GB'},
  {value: '4', label: 'Unidade'},
];

/** Situação do ICMS do item: CST do leiaute, `sn` (Simples Nacional) e `sem_cst` (indSemCST). */
export const NFCOM_ICMS_OPTIONS: readonly NfcomOption[] = [
  {value: '00', label: '00 - Tributada integralmente'},
  {value: '40', label: '40 - Isenta'},
  {value: '41', label: '41 - Não tributada'},
  {value: '51', label: '51 - Diferimento'},
  {value: '90', label: '90 - Outras'},
  {value: 'sn', label: 'Simples Nacional'},
  {value: 'sem_cst', label: 'Sem situação tributária (sem CFOP)'},
];

/** ICMS que leva alíquota. */
export const NFCOM_ICMS_WITH_RATE: readonly string[] = ['00', '90'];

/** CST de PIS e COFINS aceitos pelo leiaute. */
export const NFCOM_PIS_COFINS_CST: readonly NfcomOption[] = [
  {value: '01', label: '01 - Tributável'},
  {value: '02', label: '02 - Tributável, alíquota diferenciada'},
  {value: '06', label: '06 - Alíquota zero'},
  {value: '07', label: '07 - Isenta'},
  {value: '08', label: '08 - Sem incidência'},
  {value: '09', label: '09 - Com suspensão'},
  {value: '49', label: '49 - Outras operações'},
];

export const NFCOM_ITEMS_MAX = 990;
export const NFCOM_CANCEL_JUSTIFICATION_MIN = 15;
export const NFCOM_CANCEL_JUSTIFICATION_MAX = 255;

/** CFOP da NFCom (MOC Visão Geral, item 7.6): lista fechada, então a UI mostra seletor e nunca texto livre. */
export const NFCOM_CFOP_OPTIONS: readonly NfcomOption[] = [
  {value: '5301', label: '5.301 - Serviço da mesma natureza (dentro da UF)'},
  {value: '5302', label: '5.302 - Estabelecimento industrial (dentro da UF)'},
  {value: '5303', label: '5.303 - Estabelecimento comercial (dentro da UF)'},
  {value: '5304', label: '5.304 - Prestador de serviço de transporte (dentro da UF)'},
  {value: '5305', label: '5.305 - Geradora ou distribuidora de energia elétrica (dentro da UF)'},
  {value: '5306', label: '5.306 - Produtor rural (dentro da UF)'},
  {value: '5307', label: '5.307 - Não contribuinte (dentro da UF)'},
  {value: '6301', label: '6.301 - Serviço da mesma natureza (fora da UF)'},
  {value: '6302', label: '6.302 - Estabelecimento industrial (fora da UF)'},
  {value: '6303', label: '6.303 - Estabelecimento comercial (fora da UF)'},
  {value: '6304', label: '6.304 - Prestador de serviço de transporte (fora da UF)'},
  {value: '6305', label: '6.305 - Geradora ou distribuidora de energia elétrica (fora da UF)'},
  {value: '6306', label: '6.306 - Produtor rural (fora da UF)'},
  {value: '6307', label: '6.307 - Não contribuinte (fora da UF)'},
  {value: '7301', label: '7.301 - Serviço da mesma natureza (exterior)'},
];

/** Competências oferecidas na fatura: do mês corrente para trás, 12 meses (AAAAMM). */
export function nfcomCompetenceOptions(now = new Date(), months = 12): NfcomOption[] {
  const monthNames = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro'];
  return Array.from({length: months}, (_, i) => {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    return {value: `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}`, label: `${monthNames[d.getMonth()]} de ${d.getFullYear()}`};
  });
}

/** tpEvento que a NFCom registra, para a lista de eventos do detalhe. */
export const NFCOM_EVENT_LABELS: Record<string, string> = {
  emission: 'Emissão',
  '110111': 'Cancelamento',
  '110300': 'Vinculação de pagamento',
  '110301': 'Cancelamento da vinculação',
};
