import {describe, expect, it} from 'vitest';
import {cleanDocument, validateCNPJ} from '@/lib/utils/validators';
import {hasDuplicateViewer} from '@/lib/schemas/authorized-viewers';
import {authorizedViewerSchema} from '@/lib/schemas/authorized-viewers';

describe('CNPJ alfanumérico preservado', () => {
  it('cleanDocument mantém as letras e tira a máscara', () => {
    expect(cleanDocument('12.abc.345/01de-35')).toBe('12ABC34501DE35');
  });

  it('o exemplo oficial passa pela limpeza e pela validação', () => {
    expect(validateCNPJ(cleanDocument('12.ABC.345/01DE-35'))).toBe(true);
  });

  it('autorizados a ver o XML aceitam CNPJ alfanumérico e detectam repetido', () => {
    expect(authorizedViewerSchema.safeParse({cpf_or_cnpj: '12ABC34501DE35', name: 'Contador'}).success).toBe(true);
    expect(hasDuplicateViewer([{cpf_cnpj: '12ABC34501DE35'}], '12.ABC.345/01DE-35')).toBe(true);
  });
});
