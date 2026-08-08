import {readFileSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'
import {resolveCfopScope} from '@/lib/data/cfop'

/**
 * Paridade com services.ResolveCFOPScope (Go), que é a fonte da verdade.
 * Os dois testes leem exatamente o mesmo arquivo: se as implementações
 * divergirem, um dos dois quebra.
 */
interface CfopScopeCase {
  name: string
  suffix: string
  emit_uf: string
  dest_uf: string
  cfop?: string
  error?: boolean
}

const CASES_PATH = resolve(
  __dirname, '../../../../..', 'api/internal/services/testdata/cfop_scope_cases.json',
)

const cases: CfopScopeCase[] = JSON.parse(readFileSync(CASES_PATH, 'utf8')).cases

describe('resolveCfopScope — paridade com o Go', () => {
  it('a tabela de casos compartilhada foi encontrada e não está vazia', () => {
    expect(cases.length).toBeGreaterThan(0)
  })

  it.each(cases.map((c) => [c.name, c] as const))('%s', (_name, c) => {
    const got = resolveCfopScope(c.suffix, c.emit_uf, c.dest_uf)
    if (c.error) {
      expect(got).toBeNull()
    } else {
      expect(got).toBe(c.cfop)
    }
  })
})
