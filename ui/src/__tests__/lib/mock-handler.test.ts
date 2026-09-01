import {describe, expect, it} from 'vitest'
import {mockAdapter} from '@/lib/mock/handler'
import {
  nfeDetailFixture,
  nfeEventsFixture,
  organizationsFixture,
  servicesFixture,
  taxProfilesFixture,
} from '@/lib/mock/fixtures'
import {DOC_CONTEXTS, SHARED_REGISTRIES} from '@/lib/navigation/nav'
import type {InternalAxiosRequestConfig} from 'axios'

/**
 * O mock é o que alimenta as capturas do guia. Uma rota não modelada cai no
 * fallback (lista vazia) e a captura sai com a tela vazia — sem erro nenhum.
 * Estes testes cobrem as rotas cuja ausência já produziu imagem errada.
 */
async function get(url: string) {
  const response = await mockAdapter({method: 'get', url, headers: {}} as InternalAxiosRequestConfig)
  return response.data as Record<string, unknown>
}

describe('mock handler', () => {
  it('serve o detalhe da organização com `person`', async () => {
    // Sem isto, `org?.person?.crt` fica indefinido e o passo de produtos do
    // onboarding renderiza sem CRT nem UF.
    const data = await get(`/v1.0/organizations/${organizationsFixture[0].pk}`)
    expect(data.person).toBeDefined()
  })

  it('serve os cadastros reutilizáveis com itens', async () => {
    const services = await get('/v1.0/services')
    const profiles = await get('/v1.0/tax-profiles')
    expect(services.items).toHaveLength(servicesFixture.length)
    expect(profiles.items).toHaveLength(taxProfilesFixture.length)
  })

  /**
   * Todo cadastro que aparece na navegação vira captura do guia. Sem fixture, a
   * rota cai no fallback de lista vazia e a imagem sai em branco — sem erro.
   */
  it('serve itens para todo cadastro da navegação', async () => {
    const hrefs = [
      ...SHARED_REGISTRIES.map(i => i.href),
      ...DOC_CONTEXTS.flatMap(ctx => ctx.items.map(i => i.href)),
    ]
    const empty: string[] = []
    for (const href of hrefs) {
      const data = await get(`/v1.0${href}`)
      if (!Array.isArray(data.items) || data.items.length === 0) empty.push(href)
    }
    expect(empty).toEqual([])
  })

  it('distingue a rota de eventos da rota de detalhe', async () => {
    const key = nfeDetailFixture.sk
    const events = await get(`/v1.0/nfes/${key}/events`)
    const detail = await get(`/v1.0/nfes/${key}`)
    expect(events.items).toHaveLength(nfeEventsFixture.length)
    expect(detail.products).toBeDefined()
  })
})
