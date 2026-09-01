import {readdirSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'
import robots from '@/app/robots'
import sitemap from '@/app/sitemap'
import {GUIDE_TOPICS} from '@/lib/constants/guide'
import {PUBLIC_ORIGIN} from '@/lib/seo/site'
import {documentTitleForPath, pageNameForPath, TITLE_SUFFIX} from '@/lib/navigation/page-title'

describe('robots.txt', () => {
  const rules = robots().rules as {allow?: string[]; disallow?: string}

  it('blocks everything and opens only the landing and the guide', () => {
    expect(rules.disallow).toBe('/')
    expect(rules.allow).toEqual(['/$', '/guide'])
  })

  it('points at the sitemap', () => {
    expect(robots().sitemap).toBe(`${PUBLIC_ORIGIN}/sitemap.xml`)
  })
})

describe('sitemap.xml', () => {
  const urls = sitemap().map(entry => entry.url)

  it('lists the landing, the guide index and every topic', () => {
    expect(urls).toContain(`${PUBLIC_ORIGIN}/`)
    expect(urls).toContain(`${PUBLIC_ORIGIN}/guide`)
    for (const topic of GUIDE_TOPICS) {
      expect(urls).toContain(`${PUBLIC_ORIGIN}${topic.href}`)
    }
  })

  it('never leaks an authenticated route', () => {
    const paths = urls.map(url => url.slice(PUBLIC_ORIGIN.length))
    const leaked = paths.filter(path => path !== '/' && !path.startsWith('/guide'))
    expect(leaked).toEqual([])
  })
})

describe('page titles', () => {
  it('names a navigation route by its label', () => {
    expect(documentTitleForPath('/nfe')).toBe(`NF-e | ${TITLE_SUFFIX}`)
    expect(documentTitleForPath('/services')).toBe(`Serviços | ${TITLE_SUFFIX}`)
    expect(documentTitleForPath('/nfse/emit')).toBe(`Emitir NFS-e | ${TITLE_SUFFIX}`)
  })

  it('prefixes the sub-screen of a listing', () => {
    expect(pageNameForPath('/persons/new')).toBe('Novo · Pessoas')
    expect(pageNameForPath('/vehicles/edit')).toBe('Editar · Veículos')
    expect(pageNameForPath('/nfe/detail')).toBe('Detalhes · NF-e')
  })

  it('names the routes that live outside the navigation', () => {
    expect(pageNameForPath('/login')).toBe('Entrar')
    expect(pageNameForPath('/onboarding/empresa')).toBe('Primeiros passos')
  })

  it('falls back to the product name on an unknown route', () => {
    expect(documentTitleForPath('/nao-existe')).toBe(TITLE_SUFFIX)
  })

  /** Toda tela do app precisa de nome próprio na aba — ver `CLAUDE.md`. */
  it('names every first-level app route', () => {
    const appDir = resolve(__dirname, '../../app')
    const unnamed = readdirSync(appDir, {withFileTypes: true})
      .filter(entry => entry.isDirectory())
      .map(entry => entry.name)
      .filter(name => !name.startsWith('_') && !name.startsWith('('))
      .filter(name => pageNameForPath(`/${name}`) === null)
    expect(unnamed).toEqual([])
  })
})
