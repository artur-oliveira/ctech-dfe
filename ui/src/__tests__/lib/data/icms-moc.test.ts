import {describe, expect, it} from 'vitest';
import {ICMS_CST_OPTIONS} from '@/lib/data/icms';
import {ICMS_MOT_DESONE_OPTIONS, icmsMotDesOptions, icmsMotDesStOptions} from '@/lib/data/is';
import {PIS_COFINS_OPTIONS} from '@/lib/data/pis_cofins';
import {icmsConditionalFields} from '@/components/tax/TaxFieldsEditor';

// Auditoria MOC x XSD (F-08, F-09, F-15): as listas seguem as enumerações do leiaute.
describe('ICMS CST e motivos pelo XSD', () => {
  it('oferece os 15 CSTs do regime normal, incluindo 02, 10, 15, 30, 53 e 61', () => {
    expect(ICMS_CST_OPTIONS.map((o) => o.value)).toEqual(
      ['00', '02', '10', '15', '20', '30', '40', '41', '50', '51', '53', '60', '61', '70', '90']);
  });

  it('motDesICMS por CST', () => {
    const v = (cst: string) => icmsMotDesOptions(cst).map((o) => o.value);
    expect(v('20')).toEqual(['3', '9', '10', '11', '12']);
    expect(v('30')).toEqual(['6', '7', '9']);
    expect(v('40')).toEqual(['1', '3', '4', '5', '6', '7', '8', '9', '10', '11', '16', '90']);
    expect(v('70')).toEqual(['3', '9', '12']);
    expect(v('90')).toEqual(['3', '9', '12']);
    expect(v('51')).toEqual([]);
    expect(v('00')).toEqual([]);
  });

  it('motivo 2 não existe e 12 existe; motDesICMSST só em 10, 70 e 90', () => {
    const all = ICMS_MOT_DESONE_OPTIONS.map((o) => o.value);
    expect(all).not.toContain('2');
    expect(all).toContain('12');
    expect(icmsMotDesStOptions('10').map((o) => o.value)).toEqual(['3', '9', '12']);
    expect(icmsMotDesStOptions('30')).toEqual([]);
  });

  it('visibilidade por CST: pRedBC em 20, 51, 70, 90 (não em 40); motivo em 20, 30, 40, 70, 90 (não em 51)', () => {
    const f = icmsConditionalFields;
    expect(['20', '40', '51', '70', '90'].map((c) => f(c).showPRedBC)).toEqual([true, false, true, true, true]);
    expect(['20', '30', '40', '41', '50', '51', '70', '90'].map((c) => f(c).showMotDeSon))
      .toEqual([true, true, true, true, true, false, true, true]);
    expect(f('40').motDesRequired).toBe(true);
    expect(f('20').motDesRequired).toBe(false);
  });

  it('PIS/COFINS oferece 03 e 05 com os rótulos do XSD', () => {
    const vals = PIS_COFINS_OPTIONS.map((o) => o.value);
    expect(vals).toContain('03');
    expect(vals).toContain('05');
    expect(PIS_COFINS_OPTIONS.find((o) => o.value === '02')?.label).not.toMatch(/qtd/);
  });
});
