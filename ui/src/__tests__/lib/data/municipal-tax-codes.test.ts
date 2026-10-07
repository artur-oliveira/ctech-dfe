import {describe, expect, it} from 'vitest';
import {
  getMunicipalTaxCodes,
  TERESINA_IBGE_CODE,
  TERESINA_MUNICIPAL_TAX_CODES,
} from '@/lib/data/municipal_tax_codes';

describe('catálogo municipal de tributação', () => {
  it('expõe os 197 códigos informados para Teresina sem duplicidade', () => {
    expect(TERESINA_MUNICIPAL_TAX_CODES).toHaveLength(197);
    expect(new Set(TERESINA_MUNICIPAL_TAX_CODES.map(({municipalCode}) => municipalCode)).size).toBe(197);
  });

  it('mapeia o subitem 04.15 para o código municipal exigido pelo autorizador', () => {
    expect(TERESINA_MUNICIPAL_TAX_CODES.find(({nationalItem}) => nationalItem === '04.15')).toMatchObject({
      municipalityCode: TERESINA_IBGE_CODE,
      municipalCode: '415',
      description: 'Psicanálise.',
      taxRate: 3,
    });
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
