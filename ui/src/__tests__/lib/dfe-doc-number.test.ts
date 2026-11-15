import {describe, expect, it} from 'vitest';
import {docNumberFromKey} from '@/lib/utils/dfe';

describe('docNumberFromKey', () => {
  it('NF-e, NFC-e, CT-e e MDF-e: 9 dígitos depois da série', () => {
    expect(docNumberFromKey('35261162787449000107550010000001231000000010')).toBe(123);
  });

  it('NFS-e nacional: 13 dígitos depois da inscrição', () => {
    expect(docNumberFromKey('22110012262787449000107000000000000126111234614220')).toBe(1);
  });

  it('chave ausente ou de outro tamanho não tem número', () => {
    expect(docNumberFromKey(null)).toBeNull();
    expect(docNumberFromKey('DPS221100126278744900010700001000000000000005')).toBeNull();
  });
});
