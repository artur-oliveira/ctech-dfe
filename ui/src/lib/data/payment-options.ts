import {NF_PAYMENT_TYPES} from '@/lib/types/api';

/**
 * Payment-method options for every DF-e emit form (NF-e, NFC-e).
 *
 * The raw SEFAZ tPag table is ~20 flat codes; grouping it into the families an
 * operator actually thinks in keeps the decision inside working memory. One
 * list, one order, every document.
 */

export const PAYMENT_GROUP_CASH = 'Dinheiro';
export const PAYMENT_GROUP_CARD = 'Cartão';
export const PAYMENT_GROUP_PIX = 'PIX';
export const PAYMENT_GROUP_OTHER = 'Outros';

const CASH_CODES = new Set(['01']);
const CARD_CODES = new Set(['03', '04', '05', '21']);
const PIX_CODES = new Set(['17', '20']);

/** tPag code for "sem pagamento" — the note carries no financial settlement. */
export const NO_PAYMENT_TYPE = '90';

export function paymentGroup(code: string): string {
  if (CASH_CODES.has(code)) return PAYMENT_GROUP_CASH;
  if (CARD_CODES.has(code)) return PAYMENT_GROUP_CARD;
  if (PIX_CODES.has(code)) return PAYMENT_GROUP_PIX;
  return PAYMENT_GROUP_OTHER;
}

const GROUP_ORDER = [PAYMENT_GROUP_CASH, PAYMENT_GROUP_CARD, PAYMENT_GROUP_PIX, PAYMENT_GROUP_OTHER];

export interface PaymentOption {
  value: string
  label: string
  group: string
}

/** Grouped, ordered tPag options — the full SEFAZ table, shared by all documents. */
export const PAYMENT_OPTIONS: PaymentOption[] = Object.entries(NF_PAYMENT_TYPES)
  .map(([value, label]) => ({
    value,
    label: `${value} · ${label}`,
    group: paymentGroup(value),
  }))
  .sort((a, b) =>
    GROUP_ORDER.indexOf(a.group) - GROUP_ORDER.indexOf(b.group) || parseInt(a.value) - parseInt(b.value));

/** tPag vedados na NFC-e: 14 Duplicata Mercantil (YA02-10, rejeição 857) e 90 Sem Pagamento (YA02-40, rejeição 899). */
export const NFCE_FORBIDDEN_PAYMENT_TYPES: readonly string[] = ['14', '90'];

/** tPag oferecidos na NFC-e: a tabela completa sem os códigos que a SEFAZ rejeita nesse modelo. */
export const NFCE_PAYMENT_OPTIONS: PaymentOption[] = PAYMENT_OPTIONS.filter(
  (o) => !NFCE_FORBIDDEN_PAYMENT_TYPES.includes(o.value));

/** Quick-pick codes for a counter sale — the three an operator reaches for. */
export const QUICK_PAYMENT_TYPES = ['01', '03', '17'] as const;
