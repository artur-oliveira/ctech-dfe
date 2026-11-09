import {describe, expect, it} from 'vitest';
import {NFCE_PAYMENT_OPTIONS, PAYMENT_OPTIONS} from '@/lib/data/payment-options';

describe('tPag da NFC-e', () => {
  it('nao oferece 14 nem 90 (YA02-10 e YA02-40)', () => {
    const codes = NFCE_PAYMENT_OPTIONS.map((o) => o.value);
    expect(codes).not.toContain('14');
    expect(codes).not.toContain('90');
    expect(codes).toEqual(expect.arrayContaining(['01', '23', '24', '91', '99']));
  });

  it('a NF-e continua com a tabela completa', () => {
    expect(PAYMENT_OPTIONS.map((o) => o.value)).toEqual(expect.arrayContaining(['14', '90']));
  });
});
