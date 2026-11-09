import {emptyReformDraft, paymentBody, paymentIssues, type PaymentDraft} from '@/lib/data/nfe_reform_events';

/**
 * Vinculação de pagamento do CT-e e do CT-e OS (110300) e o cancelamento dela (110301): rascunho, pendências e corpo
 * de `POST /ctes/{k}/payment-link`. Limites espelham api/internal/services/ctes/payment_events.go (XSD evVincPgto).
 */

export const PAYMENT_NUMBER_MAX = 990;

export interface PaymentLinkDraft {
  /** nPag, número sequencial do pagamento. */
  number: string
  payment: PaymentDraft
}

export const emptyPaymentLinkDraft = (): PaymentLinkDraft => ({number: '1', payment: emptyReformDraft().payment});

export function paymentLinkIssues(d: PaymentLinkDraft): string[] {
  const n = Number(d.number);
  const out = Number.isInteger(n) && n >= 1 && n <= PAYMENT_NUMBER_MAX ? [] : [`Número do pagamento de 1 a ${PAYMENT_NUMBER_MAX}.`];
  return [...out, ...paymentIssues(d.payment)];
}

export const paymentLinkBody = (d: PaymentLinkDraft, sequence: string) => ({
  number: Number(d.number), ...paymentBody(d.payment), ...(sequence ? {sequence_number: Number(sequence)} : {}),
});

export const paymentLinkCancelBody = (eventSk: string, sequence: string) => ({
  event_sk: eventSk, ...(sequence ? {sequence_number: Number(sequence)} : {}),
});
