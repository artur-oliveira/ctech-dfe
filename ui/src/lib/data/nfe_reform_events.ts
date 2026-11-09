import {TPAG_TABLE} from '@/lib/data/payment-tables';
import {cleanDocument, validateCNPJ} from '@/lib/utils/validators';

/**
 * Eventos da reforma tributária da NF-e (NT 2026.006) e a vinculação de pagamento (110300): uma tabela de
 * definições que alimenta o menu, o modal, a validação e o corpo de `POST /nfes/{k}/ibscbs-events`.
 * Limites espelham api/internal/services/nfes/reform_events.go.
 */

/** Eventos sem XSD oficial no repositório: o layout vem das tabelas do MOC. */
export const MOC_LAYOUT_CODES: readonly string[] = ['110130', '110131', '110150', '110300'];
export const MOC_LAYOUT_NOTICE =
  'O leiaute deste evento segue as tabelas do MOC; ainda não foi conferido contra o XSD oficial, então uma rejeição de layout pode acontecer.';

/** A SEFAZ pode não aceitar estes eventos antes desta data. */
export const REFORM_EVENTS_NOTICE =
  'Os eventos da reforma tributária podem não ser aceitos pela SEFAZ antes de 16/11/2026; até lá a rejeição é esperada.';

export const REFORM_MAX_ITEMS = 990;
export const REFORM_ITEM_MAX = 990;
export const REFORM_SEQUENCE_MAX = 20;

export type ReformAuthor = '1' | '2';
export const REFORM_AUTHOR_OPTIONS = [
  {value: '1', label: 'Emitente da NF-e'},
  {value: '2', label: 'Destinatário da NF-e'},
];

/** Shape do corpo de cada evento. */
export type ReformShape = 'indicator' | 'forecast' | 'items' | 'payment';

/** Colunas por item; cada evento usa um subconjunto. */
export type ReformItemField =
  | 'ibs' | 'cbs' | 'stock' | 'credit' | 'presumed';

export interface ReformEventDef {
  code: string
  label: string
  /** Quem emite, em uma frase. */
  who: string
  /** Em que NF-e aparece: emitida pela organização (`out`) ou recebida (`in`). */
  sides: readonly ('out' | 'in')[]
  /** O que o evento informa, em uma frase. */
  summary: string
  shape: ReformShape
  itemFields?: ReformItemField[]
  /** 211110 deixa escolher o autor; nos demais ele é fixo e não vai no corpo. */
  chooseAuthor?: boolean
  /** 112110 só aceita indicator 1. */
  indicatorOnlyOn?: boolean
  /** Rótulos do indicador (sim/não). */
  indicatorLabels?: { on: string; off?: string }
}

export const REFORM_EVENTS: readonly ReformEventDef[] = [
  {code: '112110', label: 'Pagamento integral', who: 'Emitente', sides: ['out'], shape: 'indicator', indicatorOnlyOn: true,
    summary: 'Informa o efetivo pagamento integral, que libera o crédito presumido do adquirente.',
    indicatorLabels: {on: 'Confirmo o pagamento integral'}},
  {code: '112140', label: 'Fornecimento não realizado', who: 'Emitente', sides: ['out'], shape: 'items', itemFields: ['ibs', 'cbs', 'stock'],
    summary: 'Itens com pagamento antecipado que não foram fornecidos.'},
  {code: '112150', label: 'Data de previsão de entrega', who: 'Emitente', sides: ['out'], shape: 'forecast',
    summary: 'Atualiza a data prevista de entrega.'},
  {code: '211110', label: 'Crédito presumido', who: 'Emitente ou destinatário', sides: ['out', 'in'], shape: 'items', itemFields: ['presumed'], chooseAuthor: true,
    summary: 'Solicita a apropriação de crédito presumido por item.'},
  {code: '211124', label: 'Perecimento, perda, roubo ou furto', who: 'Destinatário', sides: ['in'], shape: 'items', itemFields: ['ibs', 'cbs', 'stock'],
    summary: 'Perda durante o transporte contratado pelo adquirente.'},
  {code: '211128', label: 'Aceite de débito', who: 'Destinatário', sides: ['in'], shape: 'indicator',
    summary: 'Aceita ou recusa o débito da nota de crédito na apuração.', indicatorLabels: {on: 'Aceitar o débito', off: 'Recusar o débito'}},
  {code: '211130', label: 'Imobilização de item', who: 'Destinatário', sides: ['in'], shape: 'items', itemFields: ['ibs', 'cbs', 'stock'],
    summary: 'Itens que passam a compor o ativo imobilizado.'},
  {code: '211140', label: 'Crédito de combustível', who: 'Destinatário', sides: ['in'], shape: 'items', itemFields: ['ibs', 'cbs', 'stock'],
    summary: 'Solicita a apropriação de crédito de combustível consumido.'},
  {code: '211150', label: 'Crédito dependente do adquirente', who: 'Destinatário', sides: ['in'], shape: 'items', itemFields: ['credit'],
    summary: 'Crédito de bens e serviços que dependem de atividade do adquirente.'},
  {code: '212110', label: 'Sucessão: crédito de IBS', who: 'Sucessora', sides: ['in'], shape: 'indicator',
    summary: 'Manifestação sobre o pedido de transferência de crédito de IBS em sucessão.', indicatorLabels: {on: 'Aceitar a transferência', off: 'Recusar a transferência'}},
  {code: '212120', label: 'Sucessão: crédito de CBS', who: 'Sucessora', sides: ['in'], shape: 'indicator',
    summary: 'Manifestação sobre o pedido de transferência de crédito de CBS em sucessão.', indicatorLabels: {on: 'Aceitar a transferência', off: 'Recusar a transferência'}},
  {code: '110300', label: 'Vinculação de pagamento', who: 'Emitente', sides: ['out'], shape: 'payment',
    summary: 'Vincula à nota a transação de pagamento (meio, recebedor e PSP).'},
] as const;

export const reformEventsFor = (incoming: boolean) => REFORM_EVENTS.filter((e) => e.sides.includes(incoming ? 'in' : 'out'));
export const reformEventDef = (code: string) => REFORM_EVENTS.find((e) => e.code === code);

export const PAYMENT_METHOD_OPTIONS = TPAG_TABLE.map((e) => ({value: e.code, label: `${e.code} · ${e.description}`}));

/** O código do crédito presumido é o do item; dentro do grupo a SEFAZ não o repete. */
export interface PresumedDraft { percent: string; value: string }
export interface ReformItemDraft {
  id: number
  item_number: string
  ibs: string; cbs: string
  quantity: string; unit: string
  credit_ibs: string; credit_cbs: string
  base_value: string; presumed_credit_code: string
  ibs_presumed: PresumedDraft; cbs_presumed: PresumedDraft
}
export interface PaymentDraft {
  transaction_id: string; payment_method: string
  receiver: { doc: string; name: string } | null
  psp: { doc: string; name: string } | null
}
export interface ReformDraft {
  sequence: string
  author: string
  indicator: string
  forecast: string
  items: ReformItemDraft[]
  payment: PaymentDraft
}

const emptyPresumed = (): PresumedDraft => ({percent: '', value: ''});
export const emptyReformItem = (id: number): ReformItemDraft => ({
  id, item_number: '', ibs: '', cbs: '', quantity: '', unit: '', credit_ibs: '', credit_cbs: '',
  base_value: '', presumed_credit_code: '', ibs_presumed: emptyPresumed(), cbs_presumed: emptyPresumed(),
});
export const emptyReformDraft = (): ReformDraft => ({
  sequence: '', author: '', indicator: '', forecast: '', items: [emptyReformItem(1)],
  payment: {transaction_id: '', payment_method: '', receiver: null, psp: null},
});

/** Ajusta casas decimais sem passar por float: '12.5' vira '12.50'. Devolve '' se não for número. */
export function fixedDecimal(v: string, places: number, minPlaces = places): string {
  const s = v.trim().replace(',', '.');
  if (!/^\d+(\.\d+)?$/.test(s)) return '';
  const [int, frac = ''] = s.split('.');
  const f = frac.slice(0, places).padEnd(minPlaces, '0');
  return f ? `${int}.${f}` : int;
}

const money = (v: string) => fixedDecimal(v, 2);
const reMoney = /^\d{1,13}\.\d{2}$/;
const reQty = /^\d{1,11}(\.\d{1,4})?$/;
const rePercent = /^\d{1,3}\.\d{2,4}$/;
const reTwo = /^\d{2}$/;
const reTransaction = /^.{2,35}$/;

const presumedFilled = (p: PresumedDraft) => !!(p.percent || p.value);

function itemIssues(def: ReformEventDef, it: ReformItemDraft, n: number): string[] {
  const out: string[] = [];
  const at = `Item ${n}`;
  const num = Number(it.item_number);
  if (!Number.isInteger(num) || num < 1 || num > REFORM_ITEM_MAX) out.push(`${at}: número do item de 1 a ${REFORM_ITEM_MAX}.`);
  const f = def.itemFields ?? [];
  if (f.includes('ibs') && !reMoney.test(money(it.ibs))) out.push(`${at}: informe o IBS.`);
  if (f.includes('cbs') && !reMoney.test(money(it.cbs))) out.push(`${at}: informe a CBS.`);
  if (f.includes('stock')) {
    if (!reQty.test(fixedDecimal(it.quantity, 4, 0))) out.push(`${at}: informe a quantidade (até 4 casas).`);
    if (!it.unit) out.push(`${at}: escolha a unidade.`);
  }
  if (f.includes('credit')) {
    if (!reMoney.test(money(it.credit_ibs))) out.push(`${at}: informe o crédito de IBS.`);
    if (!reMoney.test(money(it.credit_cbs))) out.push(`${at}: informe o crédito de CBS.`);
  }
  if (f.includes('presumed')) {
    if (!reMoney.test(money(it.base_value))) out.push(`${at}: informe a base de cálculo.`);
    if (!reTwo.test(it.presumed_credit_code)) out.push(`${at}: código do crédito presumido com 2 dígitos.`);
    for (const [name, p] of [['IBS', it.ibs_presumed], ['CBS', it.cbs_presumed]] as const) {
      if (presumedFilled(p) && (!rePercent.test(fixedDecimal(p.percent, 4, 2)) || !reMoney.test(money(p.value)))) {
        out.push(`${at}: crédito presumido de ${name} precisa de percentual e valor.`);
      }
    }
  }
  return out;
}

/** Validação do grupo de pagamento, comum ao 110300 da NF-e e ao do CT-e. */
export function paymentIssues(p: PaymentDraft): string[] {
  const out: string[] = [];
  if (!reTransaction.test(p.transaction_id)) out.push('Identificador da transação de 2 a 35 caracteres.');
  if (!p.payment_method) out.push('Escolha o meio de pagamento.');
  if (!p.receiver || !validateCNPJ(p.receiver.doc)) out.push('Escolha o recebedor (CNPJ).');
  if (!p.psp || !validateCNPJ(p.psp.doc)) out.push('Escolha o PSP (CNPJ).');
  return out;
}

/** Campos do pagamento na requisição (api `PaymentLink`); chamar com `paymentIssues` vazio. */
export function paymentBody(p: PaymentDraft) {
  return {
    transaction_id: p.transaction_id.trim(), payment_method: p.payment_method,
    receiver_cnpj: cleanDocument(p.receiver!.doc), psp_base_cnpj: cleanDocument(p.psp!.doc).slice(0, 8),
  };
}

/** Pendências que a api recusaria; vazio quando o evento pode ser enviado. */
export function reformIssues(def: ReformEventDef, d: ReformDraft): string[] {
  const out: string[] = [];
  if (def.chooseAuthor && !d.author) out.push('Escolha o autor do evento.');
  if (def.shape === 'indicator') {
    if (def.indicatorOnlyOn ? d.indicator !== '1' : d.indicator !== '0' && d.indicator !== '1') out.push(def.indicatorOnlyOn ? 'Confirme o pagamento integral.' : 'Escolha a resposta.');
  }
  if (def.shape === 'forecast' && !/^\d{4}-\d{2}-\d{2}$/.test(d.forecast)) out.push('Informe a data prevista de entrega.');
  if (def.shape === 'items') {
    if (d.items.length === 0 || d.items.length > REFORM_MAX_ITEMS) out.push(`Informe de 1 a ${REFORM_MAX_ITEMS} itens.`);
    const nums = d.items.map((i) => i.item_number);
    if (new Set(nums).size !== nums.length) out.push('Há item repetido.');
    d.items.forEach((it, i) => out.push(...itemIssues(def, it, i + 1)));
  }
  if (def.shape === 'payment') out.push(...paymentIssues(d.payment));
  return out;
}

export interface ReformEventBody {
  event_type: string
  sequence_number?: number
  author_type?: ReformAuthor
  indicator?: 0 | 1
  delivery_forecast?: string
  items?: Record<string, unknown>[]
  payment?: { transaction_id: string; payment_method: string; receiver_cnpj: string; psp_base_cnpj: string }
}

const presumedBody = (p: PresumedDraft, code: string) =>
  // `code` segue na requisição porque a api ainda o valida; o layout da SEFAZ usa o do item.
  presumedFilled(p) ? {code, percent: fixedDecimal(p.percent, 4, 2), value: money(p.value)} : undefined;

function itemBody(def: ReformEventDef, it: ReformItemDraft): Record<string, unknown> {
  const f = def.itemFields ?? [];
  const o: Record<string, unknown> = {item_number: Number(it.item_number)};
  if (f.includes('ibs')) o.ibs = money(it.ibs);
  if (f.includes('cbs')) o.cbs = money(it.cbs);
  if (f.includes('stock')) { o.quantity = fixedDecimal(it.quantity, 4, 0); o.unit = it.unit; }
  if (f.includes('credit')) { o.credit_ibs = money(it.credit_ibs); o.credit_cbs = money(it.credit_cbs); }
  if (f.includes('presumed')) {
    o.base_value = money(it.base_value);
    o.presumed_credit_code = it.presumed_credit_code;
    const ibs = presumedBody(it.ibs_presumed, it.presumed_credit_code);
    const cbs = presumedBody(it.cbs_presumed, it.presumed_credit_code);
    if (ibs) o.ibs_presumed_credit = ibs;
    if (cbs) o.cbs_presumed_credit = cbs;
  }
  return o;
}

/** Corpo de `POST /nfes/{k}/ibscbs-events`; só os campos do evento. Chamar depois de `reformIssues` vazio. */
export function buildReformBody(def: ReformEventDef, d: ReformDraft): ReformEventBody {
  const body: ReformEventBody = {event_type: def.code};
  if (d.sequence) body.sequence_number = Number(d.sequence);
  if (def.chooseAuthor) body.author_type = d.author as ReformAuthor;
  if (def.shape === 'indicator') body.indicator = d.indicator === '1' ? 1 : 0;
  if (def.shape === 'forecast') body.delivery_forecast = d.forecast;
  if (def.shape === 'items') body.items = d.items.map((it) => itemBody(def, it));
  if (def.shape === 'payment') body.payment = paymentBody(d.payment);
  return body;
}
