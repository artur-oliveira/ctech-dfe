import {describe, expect, it} from 'vitest';
import {readFileSync} from 'node:fs';
import {join} from 'node:path';

// Regressão: EmissionSection e FiscalConfigForm são irmãos; com a mesma `key` o React deixava a seção da aba
// anterior no DOM e acumulava uma nova a cada troca de aba. Cada irmão precisa de uma chave própria.
describe('Configuração Fiscal: chaves dos irmãos do formulário', () => {
  it('EmissionSection e FiscalConfigForm não usam a mesma key', () => {
    const src = readFileSync(join(process.cwd(), 'src/app/fiscal-config/page.tsx'), 'utf8');
    const keys = [...src.matchAll(/<(EmissionSection|FiscalConfigForm)\s[\s\S]*?key=\{([^}]+)\}/g)].map((m) => m[2]);
    expect(keys.length).toBe(2);
    expect(new Set(keys).size).toBe(2);
  });
});
