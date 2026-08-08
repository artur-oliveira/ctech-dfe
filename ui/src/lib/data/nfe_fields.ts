/**
 * Domínios do cabeçalho da NF-e, compartilhados entre o formulário de emissão e
 * o cadastro de naturezas de operação (que os define como default).
 */

export const MOD_FRETE_OPTIONS = [
  {value: '9', label: '9 – Sem frete'},
  {value: '0', label: '0 – Contratação (remetente)'},
  {value: '1', label: '1 – Contratação (destinatário)'},
  {value: '2', label: '2 – Contratação por conta de terceiros'},
  {value: '3', label: '3 – Transporte próprio do remetente'},
  {value: '4', label: '4 – Transporte próprio do destinatário'},
]

export const TP_NF_OPTIONS = [
  {value: '1', label: '1 – Saída'},
  {value: '0', label: '0 – Entrada'},
]

export const FIN_NFE_OPTIONS = [
  {value: '1', label: '1 – Normal'},
  {value: '2', label: '2 – Complementar'},
  {value: '3', label: '3 – Ajuste'},
  {value: '4', label: '4 – Devolução'},
]

export const IND_FINAL_OPTIONS = [
  {value: '1', label: '1 – Consumidor final'},
  {value: '0', label: '0 – Normal (não consumidor final)'},
]

export const IND_PRES_OPTIONS = [
  {value: '0', label: '0 – Não se aplica'},
  {value: '1', label: '1 – Presencial'},
  {value: '2', label: '2 – Internet'},
  {value: '3', label: '3 – Teleatendimento'},
  {value: '4', label: '4 – NFC-e em entrega a domicílio'},
  {value: '5', label: '5 – Presencial fora do estabelecimento'},
  {value: '9', label: '9 – Outros'},
]
