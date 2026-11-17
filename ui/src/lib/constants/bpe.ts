// Opções fechadas do BP-e (modelo 63, comum, TM e TA), espelho do leiaute e dos DTOs da API (bpes/dto*.go).
// A UI mostra seletores; texto livre só onde o campo é livre de verdade.

export interface BpeOption {
  value: string
  label: string
}

export type BpeVariant = 'normal' | 'tm' | 'ta'

export const BPE_VARIANTS: readonly BpeOption[] = [
  {value: 'normal', label: 'Comum (rodoviário, aquaviário, ferroviário)'},
  {value: 'ta', label: 'TA (transporte aéreo)'},
  {value: 'tm', label: 'TM (transporte metropolitano)'},
];

/** Rótulo do modal do documento (o TA guarda `aereo`). */
export const BPE_MODAL_LABELS: Record<string, string> = {rodoviario: 'Rodoviário', aquaviario: 'Aquaviário', ferroviario: 'Ferroviário', aereo: 'Aéreo'};

export const BPE_MODALS: readonly BpeOption[] = [
  {value: 'rodoviario', label: 'Rodoviário'},
  {value: 'aquaviario', label: 'Aquaviário'},
  {value: 'ferroviario', label: 'Ferroviário'},
];

/** indPres. */
export const BPE_PRESENCE: readonly BpeOption[] = [
  {value: '1', label: 'Presencial'},
  {value: '2', label: 'Internet'},
  {value: '3', label: 'Teleatendimento'},
  {value: '4', label: 'Entrega em domicílio'},
  {value: '5', label: 'Presencial fora do estabelecimento'},
  {value: '9', label: 'Outros'},
];

/** tpDoc do passageiro. */
export const BPE_DOC_TYPES: readonly BpeOption[] = [
  {value: '1', label: 'RG'},
  {value: '2', label: 'Título de eleitor'},
  {value: '3', label: 'Passaporte'},
  {value: '4', label: 'CNH'},
  {value: '5', label: 'Outros'},
];

/** Documento do comprador: um entre CNPJ, CPF e identificador estrangeiro (a API exige exatamente um). */
export const BPE_BUYER_DOC_KINDS: readonly BpeOption[] = [
  {value: 'cpf', label: 'CPF'},
  {value: 'cnpj', label: 'CNPJ'},
  {value: 'foreign', label: 'Identificador estrangeiro'},
];

/** tpViagem do comum. */
export const BPE_TRIP_TYPES: readonly BpeOption[] = [
  {value: '00', label: 'Convencional'},
  {value: '01', label: 'Fretamento'},
];

/** tpViagem do TA. */
export const BPE_TA_TRIP_TYPES: readonly BpeOption[] = [
  {value: '00', label: 'Regular doméstico'},
  {value: '01', label: 'Regular internacional'},
  {value: '02', label: 'Não regular doméstico'},
  {value: '03', label: 'Não regular internacional'},
];

/** tpServ do comum. */
export const BPE_SERVICE_TYPES: readonly BpeOption[] = [
  {value: '1', label: 'Convencional com sanitário'},
  {value: '2', label: 'Convencional sem sanitário'},
  {value: '3', label: 'Semileito'},
  {value: '4', label: 'Leito com ar condicionado'},
  {value: '5', label: 'Leito sem ar condicionado'},
  {value: '6', label: 'Executivo'},
  {value: '7', label: 'Semiurbano'},
  {value: '8', label: 'Longitudinal'},
  {value: '9', label: 'Travessia'},
  {value: '10', label: 'Semileito executivo'},
  {value: '11', label: 'Leito executivo'},
];

/** tpServ do TA. */
export const BPE_TA_SERVICE_TYPES: readonly BpeOption[] = [
  {value: '12', label: 'Comercial'},
  {value: '13', label: 'Executivo'},
];

/** tpAcomodacao do comum. */
export const BPE_ACCOMMODATIONS: readonly BpeOption[] = [
  {value: '1', label: 'Assento ou poltrona'},
  {value: '2', label: 'Rede'},
  {value: '3', label: 'Cabine'},
  {value: '4', label: 'Outros'},
  {value: '5', label: 'Ferroviário'},
];

/** tpAcomodacao do TA. */
export const BPE_TA_ACCOMMODATIONS: readonly BpeOption[] = [
  {value: '6', label: 'Comum'},
  {value: '7', label: 'Conforto'},
];

/** tpTrecho. */
export const BPE_LEG_TYPES: readonly BpeOption[] = [
  {value: '1', label: 'Normal'},
  {value: '2', label: 'Trecho inicial'},
  {value: '3', label: 'Conexão'},
];

/** tpComp do comum. */
export const BPE_COMPONENT_TYPES: readonly BpeOption[] = [
  {value: '01', label: 'Tarifa'},
  {value: '02', label: 'Pedágio'},
  {value: '03', label: 'Taxa de embarque'},
  {value: '04', label: 'Seguro'},
  {value: '05', label: 'TMR'},
  {value: '06', label: 'SCT'},
  {value: '99', label: 'Outros'},
];

/** tpComp do TA. */
export const BPE_TA_COMPONENT_TYPES: readonly BpeOption[] = [
  {value: '01', label: 'Tarifa'},
  {value: '03', label: 'Taxa de embarque'},
  {value: '06', label: 'SCT'},
  {value: '07', label: 'Taxa de serviço'},
  {value: '08', label: 'Taxa de conveniência'},
  {value: '99', label: 'Outros'},
];

/** tPag. */
export const BPE_PAYMENT_TYPES: readonly BpeOption[] = [
  {value: '01', label: 'Dinheiro'},
  {value: '02', label: 'Cheque'},
  {value: '03', label: 'Cartão de crédito'},
  {value: '04', label: 'Cartão de débito'},
  {value: '05', label: 'Vale'},
  {value: '06', label: 'PIX'},
  {value: '99', label: 'Outros'},
];

/** tPag que exige descrição. */
export const BPE_PAYMENT_OTHER = '99';

/** tpDesconto do comum. */
export const BPE_DISCOUNT_TYPES: readonly BpeOption[] = [
  {value: '01', label: 'Tarifa promocional'},
  {value: '02', label: 'Idoso'},
  {value: '03', label: 'Criança'},
  {value: '04', label: 'Deficiente'},
  {value: '05', label: 'Estudante'},
  {value: '06', label: 'Animal doméstico'},
  {value: '07', label: 'Acordo coletivo'},
  {value: '08', label: 'Profissional em deslocamento'},
  {value: '09', label: 'Profissional da empresa'},
  {value: '10', label: 'Jovem'},
  {value: '99', label: 'Outros'},
];

/** tpDesconto do TA. */
export const BPE_TA_DISCOUNT_TYPES: readonly BpeOption[] = [
  {value: '03', label: 'Criança'},
  {value: '09', label: 'Profissional da empresa (empregados ou tripulantes)'},
  {value: '10', label: 'Programa de fidelidade (milhagem)'},
  {value: '11', label: 'Acordo corporativo'},
  {value: '12', label: 'Acordo com grupo de passageiros'},
  {value: '13', label: 'Acompanhante de passageiro PNE'},
  {value: '99', label: 'Outros'},
];

/** Situação do ICMS: CST do leiaute e `sn` (Simples Nacional). */
export const BPE_ICMS_OPTIONS: readonly BpeOption[] = [
  {value: '00', label: '00 - Tributada integralmente'},
  {value: '40', label: '40 - Isenta'},
  {value: '41', label: '41 - Não tributada'},
  {value: '51', label: '51 - Diferimento'},
  {value: '90', label: '90 - Outras'},
  {value: 'sn', label: 'Simples Nacional'},
];

/** ICMS que leva alíquota. */
export const BPE_ICMS_WITH_RATE: readonly string[] = ['00', '90'];

export const BPE_TRIPS_MAX = 20;
export const BPE_COMPONENTS_MAX = 20;
export const BPE_PAYMENTS_MAX = 10;
export const BPE_TM_GROUPS_MAX = 99;
export const BPE_TM_ITEMS_MAX = 990;
export const BPE_TM_COMPONENTS_MAX = 20;
export const BPE_TM_COMPONENT_NAME_MAX = 15;

/** Eventos que o emitente envia (tpEvento) e o rótulo na lista de eventos. */
export const BPE_EVENT_LABELS: Record<string, string> = {
  emission: 'Emissão',
  '110111': 'Cancelamento',
  '110115': 'Não embarque',
  '110116': 'Alteração de poltrona',
  '110117': 'Excesso de bagagem',
  '112140': 'Fornecimento não realizado',
  '110300': 'Vinculação de pagamento',
  '110301': 'Cancelamento da vinculação',
};

/** tpCompra do TA (purchase_type da API). */
export const BPE_PURCHASE_TYPES: readonly BpeOption[] = [
  {value: 'normal', label: 'Compra normal'},
  {value: 'multiple', label: 'BP-e múltiplo (fretamento)'},
  {value: 'linked', label: 'Vinculado a um BP-e múltiplo'},
];

/** tpSub do bilhete comum. */
export const BPE_SUBSTITUTION_TYPES: readonly BpeOption[] = [
  {value: '1', label: 'Remarcação'},
  {value: '2', label: 'Transferência'},
  {value: '3', label: 'Transferência e remarcação'},
];

/** tpSub do TA: os do comum e mais os de correção. */
export const BPE_TA_SUBSTITUTION_TYPES: readonly BpeOption[] = [
  ...BPE_SUBSTITUTION_TYPES,
  {value: '4', label: 'Erro de tarifa'},
  {value: '5', label: 'Erro de tributação'},
  {value: '6', label: 'Decisão judicial'},
  {value: '7', label: 'Erro cadastral'},
  {value: '9', label: 'Outros'},
];

export const BPE_BOARD_PASS_MIN = 50;
export const BPE_BOARD_PASS_MAX = 1000;
export const BPE_JUSTIFICATION_MIN = 15;
export const BPE_JUSTIFICATION_MAX = 255;
/** Chave de acesso do BP-e: 44 caracteres alfanuméricos (CNPJ alfanumérico). */
export const BPE_KEY_PATTERN = /^[0-9A-Z]{44}$/;

/**
 * Classificações tributárias (cClassTrib) que o BP-e TA aceita, da tabela do IT 2025.002 RT v1.71 (`indBPeTA`), espelho de
 * `go-dfe/validate/tables/ibscbs.json`. `donation`: 0 veda indDoacao, 1 exige, 2 opcional; `reversal`: exige estorno de crédito;
 * `reduced`: o CST traz redução de alíquota (o servidor calcula a alíquota efetiva).
 */
export interface BpeTaClass extends BpeOption {
  cst: string
  donation: 0 | 1 | 2
  reversal: boolean
  reduced: boolean
}

export const BPE_TA_CLASSES: readonly BpeTaClass[] = [
  {value: '000001', cst: '000', donation: 2, reversal: false, reduced: false, label: '000001 · Tributação integral pelo IBS e CBS'},
  {value: '200050', cst: '200', donation: 2, reversal: false, reduced: true, label: '200050 · Transporte aéreo regional (redução de 40%)'},
  {value: '222001', cst: '222', donation: 2, reversal: false, reduced: false, label: '222001 · Transporte internacional com ida e volta vendidos em conjunto'},
  {value: '410001', cst: '410', donation: 0, reversal: false, reduced: false, label: '410001 · Bonificação constante do documento'},
  {value: '410003', cst: '410', donation: 1, reversal: false, reduced: false, label: '410003 · Doação ou amostra grátis'},
  {value: '410004', cst: '410', donation: 0, reversal: false, reduced: false, label: '410004 · Exportação de serviço'},
  {value: '410026', cst: '410', donation: 1, reversal: true, reduced: false, label: '410026 · Doação com anulação de crédito'},
  {value: '410999', cst: '410', donation: 0, reversal: false, reduced: false, label: '410999 · Operação não onerosa sem previsão de tributação'},
];

/** CST que não tem o grupo gIBSCBS (imunidade e não incidência). */
export const BPE_TA_NO_TAX_CST = '410';

/** Primeiro ano em que CBS e IBS deixam de ter alíquota fixada em lei (LC 214/25): CBS em 2027, IBS em 2029. */
export const BPE_CBS_RATE_FROM_YEAR = 2027;
export const BPE_IBS_RATE_FROM_YEAR = 2029;

/** tpEnteGov. */
export const BPE_GOV_ENTITIES: readonly BpeOption[] = [
  {value: '1', label: 'União'},
  {value: '2', label: 'Estados'},
  {value: '3', label: 'Distrito Federal'},
  {value: '4', label: 'Municípios'},
  {value: '5', label: 'Consórcio público'},
  {value: '6', label: 'Comitê Gestor do IBS'},
];

/** tpOperGov. Com 2 vai uma chave de DFe anterior; com 3 uma ou mais; com 1 e 4 nenhuma. */
export const BPE_GOV_OPERATIONS: readonly BpeOption[] = [
  {value: '1', label: 'Fornecimento com pagamento posterior'},
  {value: '2', label: 'Recebimento do pagamento com fornecimento já realizado'},
  {value: '3', label: 'Fornecimento com pagamento já realizado'},
  {value: '4', label: 'Recebimento do pagamento com fornecimento posterior'},
];
