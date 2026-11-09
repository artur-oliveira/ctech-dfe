import {describe, expect, it} from 'vitest';
import {emptyPaymentLinkDraft, paymentLinkBody, paymentLinkCancelBody, paymentLinkIssues} from '@/lib/utils/cte-payment-link';

const filled = () => ({
  number: '2',
  payment: {transaction_id: 'TX-1', payment_method: '17', receiver: {doc: '11222333000181', name: 'A'}, psp: {doc: '11222333000181', name: 'B'}},
});

describe('vinculação de pagamento do CT-e (110300)', () => {
  it('rascunho vazio pede transação, meio, recebedor e PSP; o número começa em 1', () => {
    expect(emptyPaymentLinkDraft().number).toBe('1');
    expect(paymentLinkIssues(emptyPaymentLinkDraft())).toHaveLength(4);
  });

  it('número do pagamento vai de 1 a 990', () => {
    for (const bad of ['', '0', '991', '1.5']) expect(paymentLinkIssues({...filled(), number: bad})).toHaveLength(1);
    expect(paymentLinkIssues(filled())).toEqual([]);
    expect(paymentLinkIssues({...filled(), number: '990'})).toEqual([]);
  });

  it('corpo da rota: raiz de 8 dígitos do PSP e sequência só quando escolhida', () => {
    expect(paymentLinkBody(filled(), '')).toEqual({
      number: 2, transaction_id: 'TX-1', payment_method: '17', receiver_cnpj: '11222333000181', psp_base_cnpj: '11222333',
    });
    expect(paymentLinkBody(filled(), '3')).toMatchObject({sequence_number: 3});
  });

  it('corpo do cancelamento (110301)', () => {
    expect(paymentLinkCancelBody('ev#1', '')).toEqual({event_sk: 'ev#1'});
    expect(paymentLinkCancelBody('ev#1', '2')).toEqual({event_sk: 'ev#1', sequence_number: 2});
  });
});
