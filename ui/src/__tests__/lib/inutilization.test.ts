import {describe, expect, it} from 'vitest';
import {isInutilizationDisabled} from '@/lib/constants/inutilization';

// NT 2023.002 v1.02: só AC, BA e SP, a partir de 11/11/2026.
describe('isInutilizationDisabled', () => {
  const before = new Date('2026-11-10T23:59:59Z');
  const from = new Date('2026-11-11T00:00:00Z');

  it('liga na data certa nas UFs afetadas', () => {
    expect(isInutilizationDisabled('SP', before)).toBe(false);
    expect(isInutilizationDisabled('SP', from)).toBe(true);
    expect(isInutilizationDisabled(' ba ', new Date('2027-01-01T00:00:00Z'))).toBe(true);
    expect(isInutilizationDisabled('AC', from)).toBe(true);
  });

  it('não afeta as demais UFs nem UF ausente', () => {
    expect(isInutilizationDisabled('PI', new Date('2030-01-01T00:00:00Z'))).toBe(false);
    expect(isInutilizationDisabled(null, from)).toBe(false);
    expect(isInutilizationDisabled(undefined, from)).toBe(false);
  });
});
