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

/** tpEvento que a NFCom registra, para a lista de eventos do detalhe. */
export const NFCOM_EVENT_LABELS: Record<string, string> = {
  emission: 'Emissão',
  '110111': 'Cancelamento',
  '110300': 'Vinculação de pagamento',
  '110301': 'Cancelamento da vinculação',
};
