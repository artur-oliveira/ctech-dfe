import {existsSync, readdirSync, readFileSync} from 'node:fs'
import {join} from 'node:path'
import {describe, expect, it} from 'vitest'
import {GUIDE_TOPICS} from '@/lib/constants/guide'
import {DOC_CONTEXTS, NAV_GROUPS, SHARED_REGISTRIES} from '@/lib/navigation/nav'

/**
 * O guia publica o que estiver em `public/guide/`. Se uma captura for renomeada
 * ou removida sem atualizar a página, o leitor vê imagem quebrada — e nada mais
 * falha. Estes testes são essa rede.
 */

const ROOT = process.cwd()
const GUIDE_DIR = join(ROOT, 'src/app/guide')
const SHOTS_DIR = join(ROOT, 'public/guide')

function pagesWithShots(): string[] {
  const pages = [join(ROOT, 'src/app/(public)/page.tsx')]
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, {withFileTypes: true})) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) walk(full)
      else if (entry.name.endsWith('.tsx')) pages.push(full)
    }
  }
  walk(GUIDE_DIR)
  return pages
}

/**
 * Telas de configuração e de conta: mesma regra dos cadastros, mas as rotas não
 * saem de um `DocContext`, então a lista é explícita.
 */
const SETTINGS_ROUTES = ['organizations', 'members', 'fiscal-config', 'certificates', 'audit-logs', 'profile']

describe('guia', () => {
  /**
   * Toda tela de cadastro e de configuração precisa de captura publicada no
   * guia, com o slug igual ao da rota — ver `CLAUDE.md`. Rota nova sem seção no
   * guia reprova aqui.
   */
  it('documenta toda tela de cadastro e de configuração com captura', () => {
    const sources = pagesWithShots().map((file) => readFileSync(file, 'utf8')).join('\n')
    const routes = [
      ...SHARED_REGISTRIES.map((item) => item.href.slice(1)),
      ...DOC_CONTEXTS.flatMap((ctx) => ctx.items.map((item) => item.href.slice(1))),
      ...SETTINGS_ROUTES,
    ]
    const undocumented = routes.filter((slug) => !sources.includes(`/guide/${slug}.webp`))
    expect(undocumented, `telas sem captura no guia: ${undocumented.join(', ')}`).toEqual([])
  })

  it('mantém todo tipo de documento com um tópico dedicado', () => {
    const docLabels = NAV_GROUPS.find((g) => g.label === 'Documentos Fiscais')!.items.map((i) => i.label)
    const topics = GUIDE_TOPICS.map((t) => `${t.title} ${t.label} ${t.description}`).join(' ')
    const missing = docLabels.filter((label) => !topics.includes(label))
    expect(missing).toEqual([])
  })

  it('tem uma rota para cada tópico do índice', () => {
    for (const topic of GUIDE_TOPICS) {
      const slug = topic.href.replace('/guide/', '')
      expect(existsSync(join(GUIDE_DIR, slug, 'page.tsx')), `rota ausente: ${topic.href}`).toBe(true)
    }
  })

  it('referencia apenas capturas que existem em public/guide', () => {
    const referenced = new Set<string>()
    for (const page of pagesWithShots()) {
      for (const match of readFileSync(page, 'utf8').matchAll(/\/guide\/([a-z0-9-]+)\.webp/g)) {
        referenced.add(match[1])
      }
    }
    expect(referenced.size).toBeGreaterThan(0)
    for (const slug of referenced) {
      expect(existsSync(join(SHOTS_DIR, `${slug}.webp`)), `captura ausente: ${slug}.webp`).toBe(true)
    }
  })

  it('não deixa captura órfã — toda imagem gerada aparece em alguma página', () => {
    const referenced = new Set<string>()
    for (const page of pagesWithShots()) {
      for (const match of readFileSync(page, 'utf8').matchAll(/\/guide\/([a-z0-9-]+)\.webp/g)) {
        referenced.add(match[1])
      }
    }
    const orphans = readdirSync(SHOTS_DIR)
      .filter((file) => file.endsWith('.webp'))
      .map((file) => file.replace(/\.webp$/, ''))
      .filter((slug) => !referenced.has(slug))
    expect(orphans, `capturas sem uso: ${orphans.join(', ')}`).toEqual([])
  })
})
