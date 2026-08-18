'use client'

import Image from 'next/image'
import Link from 'next/link'
import {ArrowRight} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {GuideChrome} from '@/components/guide/GuidePage'
import {GUIDE_TOPICS} from '@/lib/constants/guide'

const QUICKSTART = [
  {step: 'Configure', detail: 'empresa, certificado A1 e numeração'},
  {step: 'Cadastre', detail: 'o cliente e os produtos da nota'},
  {step: 'Emita', detail: 'e acompanhe até a autorização'},
]

export default function GuideHome() {
  return (
    <GuideChrome>
      <div className="mx-auto max-w-6xl px-4 py-10 md:px-6 md:py-16">
        <header className="max-w-2xl">
          <h1 className="text-3xl font-bold tracking-tight text-gray-900 md:text-4xl">
            Como usar o CTech DF-e
          </h1>
          <p className="mt-4 text-base leading-relaxed text-gray-600">
            Cada tópico mostra a tela real do sistema, explica o que acontece depois do clique e o
            que a SEFAZ devolve. Leia na ordem para configurar do zero, ou vá direto ao assunto.
          </p>
        </header>

        <section
          aria-labelledby="quickstart-title"
          className="mt-10 grid gap-8 rounded-2xl bg-gradient-login p-6 md:grid-cols-[minmax(0,1fr)_360px] md:items-center md:p-8"
        >
          <div>
            <h2 id="quickstart-title" className="text-xl font-semibold tracking-tight text-gray-900">
              Da conta vazia à primeira nota autorizada
            </h2>
            <p className="mt-2 max-w-md text-sm leading-relaxed text-gray-600">
              O onboarding pergunta só o necessário e já deixa a empresa apta a emitir em
              homologação — o ambiente de teste, sem validade fiscal.
            </p>
            <ol className="mt-5 space-y-2.5">
              {QUICKSTART.map(({step, detail}, i) => (
                <li key={step} className="flex items-baseline gap-3 text-sm text-gray-700">
                  <span
                    className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary-600 text-xs font-semibold text-white">
                    {i + 1}
                  </span>
                  <span><b className="font-semibold text-gray-900">{step}</b> {detail}</span>
                </li>
              ))}
            </ol>
            <Button variant="brand" className="mt-6" render={<Link href="/guide/getting-started"/>}>
              Começar pelos primeiros passos <ArrowRight size={16} aria-hidden="true"/>
            </Button>
          </div>
          <figure className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-card">
            <Image
              src="/guide/onboarding.webp"
              alt="Última etapa do onboarding do CTech DF-e, com NF-e, NFC-e, CT-e, MDF-e e NFS-e já habilitados para emissão"
              width={1280}
              height={800}
              sizes="(max-width: 768px) 100vw, 360px"
              className="w-full"
              priority
            />
          </figure>
        </section>

        <h2 className="mt-14 text-xl font-semibold tracking-tight text-gray-900">Tópicos</h2>
        <div className="mt-5 grid gap-3 sm:grid-cols-2">
          {GUIDE_TOPICS.map((topic) => (
            <Link
              key={topic.href}
              href={topic.href}
              className="group flex gap-4 rounded-xl border border-gray-200 p-5 transition-all hover:border-gray-300 hover:shadow-card-hover"
            >
              <span
                className="flex size-10 shrink-0 items-center justify-center rounded-lg text-white"
                style={{backgroundColor: topic.accent ?? 'var(--color-primary-600)'}}
              >
                {topic.icon}
              </span>
              <span className="min-w-0">
                <span className="flex items-baseline gap-2">
                  <b className="font-semibold text-gray-900 group-hover:text-gray-700">{topic.label}</b>
                  <small className="text-xs text-gray-400">{topic.time}</small>
                </span>
                <span className="mt-1 block text-sm leading-relaxed text-gray-600">{topic.description}</span>
                <span className="mt-2.5 flex flex-wrap gap-1.5">
                  {topic.tags.map((tag) => (
                    <em
                      key={tag}
                      className="rounded-md bg-gray-100 px-1.5 py-0.5 text-xs font-medium not-italic text-gray-600"
                    >
                      {tag}
                    </em>
                  ))}
                </span>
              </span>
            </Link>
          ))}
        </div>
      </div>
    </GuideChrome>
  )
}
