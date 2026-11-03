import {describe, expect, it} from 'vitest';
import {
  getMunicipalTaxCodes,
  hasMunicipalTaxCatalog,
  TERESINA_IBGE_CODE,
  TERESINA_MUNICIPAL_TAX_CODES,
} from '@/lib/data/municipal_tax_codes';

describe('catálogo municipal de tributação', () => {
  it('expõe as 1.148 linhas da planilha oficial de Teresina sem par repetido', () => {
    expect(TERESINA_MUNICIPAL_TAX_CODES).toHaveLength(1148);
    const pairs = TERESINA_MUNICIPAL_TAX_CODES.map(({nationalCode, municipalCode}) => `${nationalCode}/${municipalCode}`);
    expect(new Set(pairs).size).toBe(1148);
  });

  it('usa o 4º segmento do código completo como cTribMun de 3 dígitos (01.06.01.001 = 010601 + 001)', () => {
    expect(getMunicipalTaxCodes(TERESINA_IBGE_CODE, '010601')).toEqual([
      expect.objectContaining({
        municipalityCode: TERESINA_IBGE_CODE,
        nationalCode: '010601',
        municipalCode: '001',
        description: 'CONSULTORIA EM TECNOLOGIA DA INFORMACAO',
        taxRate: 5,
      }),
    ]);
    expect(TERESINA_MUNICIPAL_TAX_CODES.every(({municipalCode}) => /^\d{3}$/.test(municipalCode))).toBe(true);
    expect(TERESINA_MUNICIPAL_TAX_CODES.every(({nationalCode}) => /^\d{6}$/.test(nationalCode ?? ''))).toBe(true);
  });

  it('lista só os cTribMun do cTribNac escolhido e nada sem cTribNac', () => {
    const codes = getMunicipalTaxCodes(TERESINA_IBGE_CODE, '170101').map(({municipalCode}) => municipalCode);
    expect(codes).toEqual(['001', '002', '003', '004', '005', '006', '007']);
    expect(getMunicipalTaxCodes(TERESINA_IBGE_CODE)).toEqual([]);
    expect(hasMunicipalTaxCatalog(TERESINA_IBGE_CODE)).toBe(true);
  });

  it('descrição vazia na planilha cai para a descrição nacional do subitem', () => {
    const [entry] = getMunicipalTaxCodes(TERESINA_IBGE_CODE, '141402');
    expect(entry.description).not.toBe('');
  });

  it('cobre outros municípios pelo código IBGE da organização', () => {
    const codes = getMunicipalTaxCodes('3550308');
    expect(codes.length).toBeGreaterThan(0);
    expect(new Set(codes.map(({municipalCode}) => municipalCode)).size).toBe(codes.length);
    expect(codes.every(({municipalityCode}) => municipalityCode === '3550308')).toBe(true);
  });

  it('mantém fallback vazio para municípios sem catálogo local', () => {
    expect(getMunicipalTaxCodes('9999999')).toEqual([]);
    expect(getMunicipalTaxCodes(undefined)).toEqual([]);
  });
});
