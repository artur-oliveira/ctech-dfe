'use client'

import Image from 'next/image'
import Link from 'next/link'
import type {ReactNode} from 'react'
import {ArrowLeft, ArrowRight, CircleAlert, Info, Lightbulb, ShieldCheck} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {GUIDE_TOPICS} from '@/lib/constants/guide'
import {useAuth} from '@/lib/hooks/useAuth'

/** Cabeçalho e rodapé públicos do guia — não depende de sessão. */
export function GuideChrome({children}: { children: ReactNode }) {
  const {user} = useAuth()
  return (
    <div className="min-h-screen bg-white">
      <header className="sticky top-0 z-30 border-b border-gray-200 bg-white/90 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3 md:px-6">
          <Link href="/" className="flex items-center gap-2.5">
            <Image src="/app.svg" alt="" aria-hidden="true" width={28} height={28} unoptimized/>
            <span className="font-semibold text-gray-900">CTech DF-e</span>
            <span className="hidden text-sm text-gray-400 sm:inline">Guia</span>
          </Link>
          <Button variant="brand" size="sm" render={<Link href={user ? '/dashboard' : '/login'}/>}>
            {user ? 'Ir para o painel' : 'Entrar'}
          </Button>
        </div>
      </header>
      {children}
      <footer className="border-t border-gray-200">
        <div
          className="mx-auto flex max-w-6xl flex-col gap-2 px-4 py-8 text-sm text-gray-500 md:flex-row md:items-center md:justify-between md:px-6">
          <p>Guia do CTech DF-e — telas reais do sistema, com dados de demonstração.</p>
          <Link href="/" className="font-medium text-gray-700 hover:text-gray-900">Voltar ao site</Link>
        </div>
      </footer>
    </div>
  )
}

export interface GuideSection {
  id: string
  title: string
  summary?: string
  body: ReactNode
  image?: { src: string; alt: string }
}

/**
 * Renderiza um tópico do guia: navegação entre tópicos, índice da página e as
 * seções com captura de tela. Toda seção com imagem aponta para um arquivo de
 * `public/guide/`, gerado por `npm run screens:capture` (ver `../../../DOCS.md §5`).
 */
export function GuidePage({
                            title,
                            description,
                            sections,
                            currentHref,
                          }: {
  title: string
  description: string
  sections: GuideSection[]
  currentHref: string
}) {
  const index = GUIDE_TOPICS.findIndex((topic) => topic.href === currentHref)
  const next = GUIDE_TOPICS[index + 1]

  return (
    <GuideChrome>
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-12">
        {/*
          `overflow-x-auto` também torna o overflow vertical `auto`, e a altura do
          nav é exatamente a das pílulas — sem folga vertical o `ring` da pílula
          ativa é cortado em cima e embaixo. `py-1` dá a folga, `-my-1` devolve o
          espaçamento original.
        */}
        <nav
          aria-label="Tópicos do guia"
          className="-mx-4 -my-1 mb-7 overflow-x-auto px-4 py-1 md:mx-0 md:px-0"
        >
          <ul className="flex w-max gap-1.5">
            {GUIDE_TOPICS.map((item) => {
              const active = item.href === currentHref
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    aria-current={active ? 'page' : undefined}
                    className={`inline-flex min-h-11 items-center rounded-lg px-3 text-sm font-medium transition-colors sm:min-h-0 sm:py-1.5 ${
                      active
                        ? 'bg-primary-50 text-primary-700 ring-1 ring-primary-200'
                        : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
                    }`}
                  >
                    {item.label}
                  </Link>
                </li>
              )
            })}
          </ul>
        </nav>

        <header className="max-w-2xl">
          <h1 className="text-3xl font-bold tracking-tight text-gray-900">{title}</h1>
          <p className="mt-3 text-base leading-relaxed text-gray-600">{description}</p>
        </header>

        <div className="mt-10 grid gap-10 lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-12">
          <aside className="lg:sticky lg:top-24 lg:self-start">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Nesta página</p>
            <ol className="mt-3 space-y-1.5">
              {sections.map((section) => (
                <li key={section.id}>
                  <a
                    href={`#${section.id}`}
                    className="block py-1 text-sm text-gray-600 transition-colors hover:text-primary-700"
                  >
                    {section.title}
                  </a>
                </li>
              ))}
            </ol>
          </aside>

          <div className="min-w-0 space-y-14">
            {sections.map((section) => (
              <article key={section.id} id={section.id} className="scroll-mt-24">
                <h2 className="text-xl font-semibold tracking-tight text-gray-900">{section.title}</h2>
                {section.summary && (
                  <p className="mt-2 max-w-2xl text-sm leading-relaxed text-gray-600">{section.summary}</p>
                )}
                {section.image && (
                  <figure className="mt-5 overflow-hidden rounded-xl border border-gray-200 bg-gray-50 shadow-card">
                    <Image
                      src={section.image.src}
                      alt={section.image.alt}
                      width={1280}
                      height={800}
                      sizes="(max-width: 1024px) 100vw, 760px"
                      className="w-full"
                      unoptimized
                    />
                  </figure>
                )}
                <div className="mt-5 space-y-4 text-sm leading-relaxed text-gray-700">{section.body}</div>
              </article>
            ))}
          </div>
        </div>

        <div
          className="mt-16 flex flex-col gap-3 border-t border-gray-200 pt-6 sm:flex-row sm:items-center sm:justify-between">
          <Link
            href="/guide"
            className="inline-flex min-h-11 items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-900 sm:min-h-0"
          >
            <ArrowLeft size={16} aria-hidden="true"/> Todos os tópicos
          </Link>
          {next && (
            <Link
              href={next.href}
              className="inline-flex min-h-11 items-center gap-2 text-sm font-medium text-primary-700 hover:text-primary-800 sm:min-h-0"
            >
              Próximo: {next.label} <ArrowRight size={16} aria-hidden="true"/>
            </Link>
          )}
        </div>
      </div>
    </GuideChrome>
  )
}

export function GuideSteps({children}: { children: ReactNode }) {
  return (
    <ol className="ml-5 list-decimal space-y-2 marker:font-semibold marker:text-primary-600">{children}</ol>
  )
}

export function GuideBullets({children}: { children: ReactNode }) {
  return <ul className="ml-5 list-disc space-y-2 marker:text-gray-300">{children}</ul>
}

export function GuideTerms({children}: { children: ReactNode }) {
  return <dl className="divide-y divide-gray-100 rounded-xl border border-gray-200">{children}</dl>
}

export function GuideTerm({term, children}: { term: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 px-4 py-3 sm:grid-cols-[180px_minmax(0,1fr)] sm:gap-4">
      <dt className="font-semibold text-gray-900">{term}</dt>
      <dd className="text-gray-600">{children}</dd>
    </div>
  )
}

const CALLOUT_STYLES = {
  info: {icon: Info, box: 'border-gray-200 bg-gray-50', mark: 'text-gray-500'},
  tip: {icon: Lightbulb, box: 'border-primary-200 bg-primary-50/60', mark: 'text-primary-700'},
  safe: {icon: ShieldCheck, box: 'border-primary-200 bg-primary-50/60', mark: 'text-primary-700'},
  warning: {icon: CircleAlert, box: 'border-amber-200 bg-amber-50', mark: 'text-amber-700'},
} as const

export function GuideCallout({kind = 'info', title, children}: {
  kind?: keyof typeof CALLOUT_STYLES
  title: string
  children: ReactNode
}) {
  const {icon: Icon, box, mark} = CALLOUT_STYLES[kind]
  return (
    <aside className={`flex gap-3 rounded-xl border p-4 ${box}`}>
      <Icon size={18} className={`mt-0.5 shrink-0 ${mark}`} aria-hidden="true"/>
      <div className="min-w-0 space-y-1">
        <p className="font-semibold text-gray-900">{title}</p>
        <div className="text-gray-700">{children}</div>
      </div>
    </aside>
  )
}
