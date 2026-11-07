import { describe, it, expect } from 'vitest';
import { ACCESS_KEY_MOD_CTE, ACCESS_KEY_MOD_MDFE, ACCESS_KEY_MOD_NFE, validateAccessKey } from '@/lib/utils/access-key';

const VALID = '35250512345678000195550010000000011000000015';

describe('validateAccessKey', () => {
  it('aceita uma chave numérica válida', () => {
    expect(validateAccessKey(VALID)).toEqual({ valid: true });
  });

  it('rejeita tamanho incorreto', () => {
    expect(validateAccessKey(VALID.slice(0, 43))).toEqual({ valid: false, error: 'length' });
  });

  it('rejeita cUF inexistente', () => {
    expect(validateAccessKey('99' + VALID.slice(2))).toEqual({ valid: false, error: 'cUF' });
  });

  it('rejeita mês 13 em AAMM', () => {
    expect(validateAccessKey(VALID.slice(0, 2) + '2513' + VALID.slice(6))).toEqual({ valid: false, error: 'AAMM' });
  });

  it('rejeita mod diferente de 55 (NFC-e fora de escopo)', () => {
    expect(validateAccessKey(VALID.slice(0, 20) + '65' + VALID.slice(22))).toEqual({ valid: false, error: 'mod' });
  });

  it('rejeita tpEmis=9 (exclusivo de NFC-e)', () => {
    expect(validateAccessKey(VALID.slice(0, 34) + '9' + VALID.slice(35))).toEqual({ valid: false, error: 'tpEmis' });
  });

  it('rejeita cDV incorreto', () => {
    const lastDigit = VALID[43];
    const bad = lastDigit === '0' ? '1' : '0';
    expect(validateAccessKey(VALID.slice(0, 43) + bad)).toEqual({ valid: false, error: 'cDV' });
  });

  it('aceita CPF com prefixo 000 e rejeita DV de CPF inválido', () => {
    const base = VALID.slice(0, 6) + '00052998224725' + VALID.slice(20, 43);
    const ok = base + computeExpectedDV(base);
    expect(validateAccessKey(ok)).toEqual({ valid: true });

    const badCpfBase = VALID.slice(0, 6) + '00052998224724' + VALID.slice(20, 43);
    const badCpf = badCpfBase + computeExpectedDV(badCpfBase);
    expect(validateAccessKey(badCpf)).toEqual({ valid: false, error: 'doc' });
  });
});

/** Chave com o documento e o modelo dados, DV calculado: `35 2507 <doc> <mod> 001 000000001 <tpEmis> 00000001 <cDV>`. */
function keyWith(doc: string, mod: string, tpEmis = '1'): string {
  const k43 = `352507${doc}${mod}001000000001${tpEmis}00000001`;
  return k43 + computeExpectedDV(k43);
}

const ALNUM_CNPJ = '12ABC34501DE35'; // exemplo oficial da Receita (IN RFB 2229/2024)

describe('validateAccessKey: modelos e CNPJ alfanumérico', () => {
  it('aceita MDF-e (58) quando o campo pede 58', () => {
    expect(validateAccessKey(keyWith('12345678000195', '58'), [ACCESS_KEY_MOD_MDFE])).toEqual({ valid: true });
  });

  it('aceita CT-e (57) e NF-e (55) num campo que aceita os dois', () => {
    const models = [ACCESS_KEY_MOD_NFE, ACCESS_KEY_MOD_CTE];
    expect(validateAccessKey(keyWith('12345678000195', '57'), models)).toEqual({ valid: true });
    expect(validateAccessKey(keyWith('12345678000195', '55'), models)).toEqual({ valid: true });
  });

  it('rejeita MDF-e num campo de NF-e (padrão)', () => {
    expect(validateAccessKey(keyWith('12345678000195', '58'))).toEqual({ valid: false, error: 'mod' });
  });

  it('aceita CNPJ alfanumérico nas posições 7 a 20, com DV ASCII menos 48', () => {
    expect(validateAccessKey(keyWith(ALNUM_CNPJ, '58'), [ACCESS_KEY_MOD_MDFE])).toEqual({ valid: true });
    expect(validateAccessKey(keyWith(ALNUM_CNPJ, '55'))).toEqual({ valid: true });
  });

  it('rejeita CNPJ alfanumérico com DV da regra antiga (A=10) e letra minúscula', () => {
    expect(validateAccessKey(keyWith('12ABC34501DE45', '58'), [ACCESS_KEY_MOD_MDFE])).toEqual({ valid: false, error: 'doc' });
    expect(validateAccessKey(keyWith('12abc34501de35', '58'), [ACCESS_KEY_MOD_MDFE])).toEqual({ valid: false, error: 'length' });
  });

  it('confere o tpEmis do modelo: MDF-e aceita 1 a 3, CT-e não aceita 2', () => {
    expect(validateAccessKey(keyWith('12345678000195', '58', '3'), [ACCESS_KEY_MOD_MDFE])).toEqual({ valid: true });
    expect(validateAccessKey(keyWith('12345678000195', '58', '4'), [ACCESS_KEY_MOD_MDFE])).toEqual({ valid: false, error: 'tpEmis' });
    expect(validateAccessKey(keyWith('12345678000195', '57', '2'), [ACCESS_KEY_MOD_CTE])).toEqual({ valid: false, error: 'tpEmis' });
  });

  it('tenta CNPJ antes do CPF prefixado: 00.000.000/0001-91 vale', () => {
    expect(validateAccessKey(keyWith('00000000000191', '55'))).toEqual({ valid: true });
  });
});

// Test-local reimplementation of the cDV algorithm, kept intentionally
// separate from the production calcAccessKeyDV so this test doesn't silently
// pass if both were wrong in the same way.
function computeExpectedDV(key43: string): string {
  const weights = [2, 3, 4, 5, 6, 7, 8, 9];
  let sum = 0;
  for (let i = key43.length - 1, wi = 0; i >= 0; i--, wi++) {
    sum += (key43.charCodeAt(i) - 48) * weights[wi % 8];
  }
  const rem = sum % 11;
  return String(rem < 2 ? 0 : 11 - rem);
}
