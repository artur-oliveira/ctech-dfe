/**
 * Grupos e campos da carta de correção do CT-e (`grupoAlterado` / `campoAlterado` do evento 110110), com os
 * nomes das tags do leiaute. É uma lista de escolha: o operador não digita nome de grupo. O que a SEFAZ aceita
 * corrigir não está catalogado no MOC para nós (rejeição 681): a lista só oferece campos de texto e cadastro, e
 * a SEFAZ continua sendo quem recusa. Valores que entram no cálculo do imposto, o tomador e as datas ficam fora.
 */

export interface CteCorrectionGroup {
  value: string
  label: string
  /** Grupo que se repete no leiaute: pede o número do item (`nroItemAlterado`, 1 a 99). */
  repeats?: boolean
  fields: { value: string; label: string }[]
}

// Campos do próprio grupo da pessoa (rem, dest, exped, receb); xFant só existe em rem no leiaute.
const partyFields = (withFant: boolean) => [
  {value: 'xNome', label: 'Razão social ou nome'},
  ...(withFant ? [{value: 'xFant', label: 'Nome fantasia'}] : []),
  {value: 'IE', label: 'Inscrição estadual'},
  {value: 'fone', label: 'Telefone'},
  {value: 'email', label: 'E-mail'},
];

// Endereço: grupo enderReme, enderDest, enderExped ou enderReceb, filho da pessoa.
const ADDRESS_FIELDS = [
  {value: 'xLgr', label: 'Logradouro'},
  {value: 'nro', label: 'Número'},
  {value: 'xCpl', label: 'Complemento'},
  {value: 'xBairro', label: 'Bairro'},
  {value: 'CEP', label: 'CEP'},
];

export const CTE_CORRECTION_GROUPS: CteCorrectionGroup[] = [
  {value: 'ide', label: 'Identificação (ide)', fields: [
    {value: 'natOp', label: 'Natureza da operação'},
    {value: 'xDetRetira', label: 'Detalhe da retirada'},
  ]},
  {value: 'compl', label: 'Complemento (compl)', fields: [
    {value: 'xCaracAd', label: 'Característica adicional do transporte'},
    {value: 'xCaracSer', label: 'Característica adicional do serviço'},
    {value: 'xEmi', label: 'Funcionário emissor'},
    {value: 'xObs', label: 'Observações gerais'},
  ]},
  {value: 'ObsCont', label: 'Observação do contribuinte (ObsCont)', repeats: true, fields: [
    {value: 'xCampo', label: 'Identificação do campo'},
    {value: 'xTexto', label: 'Texto'},
  ]},
  {value: 'ObsFisco', label: 'Observação do fisco (ObsFisco)', repeats: true, fields: [
    {value: 'xCampo', label: 'Identificação do campo'},
    {value: 'xTexto', label: 'Texto'},
  ]},
  {value: 'rem', label: 'Remetente (rem)', fields: partyFields(true)},
  {value: 'enderReme', label: 'Endereço do remetente (enderReme)', fields: ADDRESS_FIELDS},
  {value: 'dest', label: 'Destinatário (dest)', fields: partyFields(false)},
  {value: 'enderDest', label: 'Endereço do destinatário (enderDest)', fields: ADDRESS_FIELDS},
  {value: 'exped', label: 'Expedidor (exped)', fields: partyFields(false)},
  {value: 'enderExped', label: 'Endereço do expedidor (enderExped)', fields: ADDRESS_FIELDS},
  {value: 'receb', label: 'Recebedor (receb)', fields: partyFields(false)},
  {value: 'enderReceb', label: 'Endereço do recebedor (enderReceb)', fields: ADDRESS_FIELDS},
  {value: 'infCarga', label: 'Informações da carga (infCarga)', fields: [
    {value: 'proPred', label: 'Produto predominante'},
    {value: 'xOutCat', label: 'Outras características da carga'},
  ]},
];

export const correctionGroup = (value: string) => CTE_CORRECTION_GROUPS.find((g) => g.value === value);
