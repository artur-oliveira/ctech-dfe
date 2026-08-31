import {readdirSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'
import {GUIDE_TOPICS} from '@/lib/constants/guide'
import {
  CONTEXT_BY_HREF,
  contextForPath,
  DOC_CONTEXTS,
  isItemActive,
  NAV_GROUPS,
  SEARCH_ENTRIES,
  SHARED_REGISTRIES,
} from '@/lib/navigation/nav'

/**
 * Rotas que existem como página mas não pertencem à navegação autenticada:
 * fluxos de entrada, telas de erro, e as subpáginas (`new`/`edit`/`detail`)
 * alcançadas a partir da própria listagem.
 */
const NOT_NAVIGABLE = new Set([
  'callback',
  'invite',
  'login',
  'onboarding',
  'terms-addendum',
  'unavailable',
])

/** Rotas de primeiro nível do App Router, ignorando as subpáginas. */
function appRoutes(): string[] {
  const appDir = resolve(__dirname, '../../app')
  return readdirSync(appDir, {withFileTypes: true})
    .filter(entry => entry.isDirectory())
    .map(entry => entry.name)
    .filter(name => !name.startsWith('_') && !name.startsWith('('))
}

describe('configuração de navegação', () => {
  it('mantém só cadastros compartilhados na barra global', () => {
    expect(SHARED_REGISTRIES.map(i => i.href)).toEqual(['/persons', '/products'])
  })

  it('coloca os cadastros exclusivos dentro do contexto do documento', () => {
    const hrefsOf = (key: string) =>
      DOC_CONTEXTS.find(c => c.key === key)!.items.map(i => i.href)

    expect(hrefsOf('nfse')).toContain('/services')
    expect(hrefsOf('nfse')).toContain('/service-locations')
    expect(hrefsOf('mdfe')).toContain('/vehicles')
    expect(hrefsOf('nfce')).toContain('/fuel-pumps')
    expect(hrefsOf('nfe')).toContain('/operations')

    const globalHrefs = NAV_GROUPS.flatMap(g => g.items.map(i => i.href))
    for (const ctx of DOC_CONTEXTS) {
      for (const item of ctx.items) {
        expect(globalHrefs).not.toContain(item.href)
      }
    }
  })

  it('resolve o contexto do documento pela rota mais específica', () => {
    expect(contextForPath('/services')?.key).toBe('nfse')
    expect(contextForPath('/services/new')?.key).toBe('nfse')
    expect(contextForPath('/vehicles')?.key).toBe('mdfe')
    expect(contextForPath('/nfe/emit')?.key).toBe('nfe')
    expect(contextForPath('/dashboard')).toBeNull()
  })

  it('não marca o pai como ativo quando um filho responde pela rota', () => {
    expect(isItemActive('/nfe', '/nfe/emit')).toBe(false)
    expect(isItemActive('/nfe/emit', '/nfe/emit')).toBe(true)
    expect(isItemActive('/nfe', '/nfe/detail')).toBe(true)
  })

  it('não repete rotas no índice da busca', () => {
    const hrefs = SEARCH_ENTRIES.map(e => e.href)
    expect(new Set(hrefs).size).toBe(hrefs.length)
  })

  it('mapeia cada rota de contexto para um contexto existente', () => {
    for (const key of Object.values(CONTEXT_BY_HREF)) {
      expect(DOC_CONTEXTS.some(c => c.key === key)).toBe(true)
    }
  })

  it('indexa todo tópico do guia', () => {
    const indexed = new Set(SEARCH_ENTRIES.map(e => e.href))
    for (const topic of GUIDE_TOPICS) {
      expect(indexed.has(topic.href)).toBe(true)
    }
  })

  /**
   * A regra "toda página nova entra na busca global" — ver `CLAUDE.md`.
   * Se este teste quebrar, adicione a rota em `lib/navigation/nav`.
   */
  it('deixa toda página navegável pesquisável', () => {
    const indexed = new Set(SEARCH_ENTRIES.map(e => e.href))
    const missing = appRoutes()
      .filter(route => !NOT_NAVIGABLE.has(route))
      .filter(route => !indexed.has(`/${route}`))
    expect(missing).toEqual([])
  })
})
