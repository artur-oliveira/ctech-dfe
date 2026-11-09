import {describe, it, expect} from 'vitest';
import {TP_VALE_PED_OPTIONS, tollProviderSchema} from '@/lib/schemas/toll-providers';
import {WHEELSET_OPTIONS} from '@/lib/schemas/vehicles';

// Auditoria MOC MDF-e M-03 e M-04: os domínios do cadastro seguem o XSD (tpValePed 01|04, tpRod 01..06).
describe('domínios do MDF-e no cadastro', () => {
  it('tpValePed oferece só 01 e 04', () => {
    expect(TP_VALE_PED_OPTIONS.map((o) => o.value)).toEqual(['01', '04']);
    const base = {name: 'Fornecedora', cnpj_forn: '11222333000181'};
    expect(tollProviderSchema.safeParse({...base, tp_vale_ped: '04'}).success).toBe(true);
    expect(tollProviderSchema.safeParse({...base, tp_vale_ped: '02'}).success).toBe(false);
  });

  it('tpRod oferece 01 a 06 e nunca 99', () => {
    expect(WHEELSET_OPTIONS.map((o) => o.value)).toEqual(['01', '02', '03', '04', '05', '06']);
  });
});
