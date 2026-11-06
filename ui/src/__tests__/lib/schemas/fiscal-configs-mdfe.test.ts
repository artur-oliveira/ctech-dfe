import {describe, expect, it} from 'vitest';
import {mdfeConfigSchema} from '@/lib/schemas/fiscal-configs';

/** Só o campo novo: o resto do schema é o `baseFields` compartilhado. */
const accepts = (rntrc: string | undefined) =>
  mdfeConfigSchema.shape.rntrc.safeParse(rntrc).success;

describe('mdfeConfigSchema: RNTRC', () => {
  it('aceita vazio (opcional no MDF-e) e 8 dígitos', () => {
    expect(accepts(undefined)).toBe(true);
    expect(accepts('')).toBe(true);
    expect(accepts('12345678')).toBe(true);
  });

  it('recusa tamanho diferente de 8 e não dígitos', () => {
    expect(accepts('1234567')).toBe(false);
    expect(accepts('123456789')).toBe(false);
    expect(accepts('1234567a')).toBe(false);
  });
});
