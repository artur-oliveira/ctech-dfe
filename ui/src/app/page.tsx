'use client'

import {useState} from 'react'
import Link from 'next/link'
import Image from 'next/image'
import {ACCOUNTS_LEGAL_URL, DFE_TERMS_URL, PRIVACY_POLICY_URL} from '@/lib/legal'
import {ArrowRight, Check, CheckCircle2, Puzzle, ShieldCheck, Zap} from 'lucide-react'
import {useAuth} from '@/lib/hooks/useAuth'
import {Button} from '@/components/ui/button'
import {AuthorizationCard} from '@/components/landing/authorization-card'
import {DFE_DOCUMENTS} from '@/lib/constants/dfe-documents'
import type {DfeThemeKey} from '@/lib/theme/dfe-theme'

const FLOW_STEPS = [
  {
    label: 'Emitir',
    body: 'Escolha o cliente e os produtos para NF-e, NFC-e, CT-e ou MDF-e. O sistema já sabe o resto.',
  },
  {
    label: 'Processar',
    body: 'O documento é assinado digitalmente e enviado direto para a SEFAZ.',
  },
  {
    label: 'Autorizar',
    body: 'Em segundos, o documento volta autorizado — pronto para imprimir, baixar ou cancelar se precisar.',
  },
]

// Capturas reais do sistema, geradas por `npm run screens:capture` a partir do
// mock — nenhum dado de cliente entra aqui. Alternam de lado na composição.
const SCREENS = [
  {
    slug: 'nfe-emit-review',
    kicker: 'Emissão',
    title: 'Quatro passos e a nota é autorizada',
    body: 'Destinatário, produtos, pagamento e revisão. O CFOP vem do perfil fiscal, a condição de pagamento monta as parcelas e a revisão mostra a nota inteira antes do envio.',
    alt: 'Revisão de uma NF-e antes da emissão, com destinatário, produtos, pagamento e total',
    href: '/guide/nfe',
    linkLabel: 'Ver a emissão passo a passo',
  },
  {
    slug: 'nfe-distribution',
    kicker: 'Distribuição',
    title: 'Notas emitidas para você',
    body: 'A SEFAZ entrega tudo que foi emitido para a sua empresa. O sistema consulta sozinho, e você importa por chave ou XML o que faltar.',
    alt: 'Documentos recebidos da SEFAZ por distribuição, com NSU, chave de acesso e data',
    href: '/guide/distributions',
    linkLabel: 'Como funciona a distribuição',
  },
  {
    slug: 'nfe-events',
    kicker: 'Eventos',
    title: 'Cancelamento e correção com histórico',
    body: 'Documento autorizado não se edita — se corrige por evento. Cada cancelamento, carta de correção ou encerramento fica no histórico.',
    alt: 'Linha do tempo de eventos de uma NF-e, com emissão e carta de correção registradas',
    href: '/guide/events',
    linkLabel: 'Entender os eventos',
  },
]

const BENEFITS = [
  {
    icon: Puzzle,
    title: 'Flexível',
    body: 'Uma conta, várias empresas — cada uma com seus próprios produtos, clientes e configuração fiscal, do seu jeito.',
  },
  {
    icon: Zap,
    title: 'Emissão resiliente',
    body: 'Uptime de 99,9%. Sua nota sai autorizada em segundos, até nos horários de mais movimento.',
  },
  {
    icon: CheckCircle2,
    title: 'Simples',
    body: 'Emita em poucos cliques e acompanhe o status em tempo real, da fila até a autorização.',
  },
  {
    icon: ShieldCheck,
    title: 'Segurança',
    body: 'Seu certificado fica guardado com criptografia, cada documento é assinado digitalmente e enviado direto para a SEFAZ.',
  },
]

const PLANS = [
  {
    name: 'Free',
    tagline: 'Para testar sem compromisso',
    price: 'R$ 0',
    period: '/mês',
    features: [
      '1 empresa e 1 usuário',
      'Até 3 NF-e por mês',
      'Até 15 NFC-e por mês',
      'Até 3 MDF-e por mês (frota própria)',
      'Até 3 NFS-e por mês (em breve)',
      'Sem CT-e',
    ],
    highlight: false,
  },
  {
    name: 'Pro',
    tagline: 'Para operações em crescimento',
    price: 'R$ 350',
    period: '/mês',
    features: [
      'Até 10 empresas e 25 usuários',
      'Até 1.200 NF-e por mês',
      'Até 6.000 NFC-e por mês',
      'Até 1.200 CT-e por mês',
      'Até 1.200 MDF-e por mês',
      'Até 1.200 NFS-e por mês (em breve)',
    ],
    highlight: true,
  },
  {
    name: 'Sob demanda',
    tagline: 'Pague só pelo que emitir',
    price: 'Variável',
    period: '',
    features: [
      'R$ 0,05 por NF-e',
      'R$ 0,01 por NFC-e',
      'R$ 0,50 por CT-e',
      'R$ 0,10 por MDF-e',
      'R$ 0,05 por NFS-e (em breve)',
    ],
    highlight: false,
  },
]

const ROADMAP = [
  {
    title: 'Inventário / Estoque fiscal',
    body: 'Controle de estoque integrado à emissão fiscal.',
  },
  {
    title: 'Arquivos SPED (EFD/ICMS-IPI)',
    body: 'Geração automática dos arquivos fiscais obrigatórios.',
  },
  {
    title: 'Outros documentos fiscais',
    body: 'BP-e, NF Gás, NFAg, NF3e e NFCom',
  },
]

function LoadingScreen() {
  return (
    <div className="flex items-center justify-center min-h-screen bg-gradient-login">
      <div className="text-center">
        <Image src="/app.svg" alt="" aria-hidden="true" width={64} height={64} className="mx-auto mb-4" priority unoptimized/>
        <h1 className="text-3xl font-bold text-gray-900 mb-2">CTech DF-e</h1>
        <p className="text-gray-600 text-sm">Carregando...</p>
      </div>
    </div>
  )
}

export default function Home() {
  const {user, loading} = useAuth()
  const [carouselTheme, setCarouselTheme] = useState<DfeThemeKey>('nfe')

  if (loading) {
    return <LoadingScreen/>
  }

  const primaryHref = user ? '/dashboard' : '/login'
  const primaryLabel = user ? 'Ir para o painel' : 'Entrar'

  return (
    <div className="min-h-screen bg-white" data-dfe-theme={carouselTheme}>
      <header className="mx-auto flex max-w-6xl items-center justify-between px-6 py-6">
        <div className="flex items-center gap-2.5">
          <Image src="/app.svg" alt="" aria-hidden="true" width={32} height={32} priority unoptimized/>
          <span className="font-semibold text-gray-900">CTech DFe</span>
        </div>
        <div className="flex items-center gap-2">
          <Link
            href="/guide"
            className="inline-flex min-h-11 items-center rounded-lg px-3 text-sm font-medium text-gray-600 hover:text-gray-900 sm:min-h-0 sm:py-2"
          >
            Guia
          </Link>
          <Button variant="brand" render={<Link href={primaryHref}/>}>
            {primaryLabel}
          </Button>
        </div>
      </header>

      <section className="bg-gradient-login">
        <div className="mx-auto grid max-w-6xl gap-12 px-6 py-16 md:grid-cols-2 md:items-center md:py-24">
          <div className="space-y-6">
            <p className="font-mono text-xs tracking-widest text-primary-700 uppercase">
              NF-e · NFC-e · CT-e · MDF-e
            </p>
            <h1 className="text-4xl font-bold leading-tight tracking-tight text-gray-900 md:text-5xl">
              Emita seus documentos fiscais de forma simples e resiliente.
            </h1>
            <p className="max-w-md text-base leading-relaxed text-gray-600">
              Da emissão à autorização em segundos — com fila resiliente e novas tentativas automáticas nos raros
              momentos de instabilidade da SEFAZ.
            </p>
            <div className="flex items-center gap-3">
              <Button variant="brand" size="lg" render={<Link href={primaryHref}/>}>
                {primaryLabel}
              </Button>
            </div>
          </div>

          <div className="flex justify-center md:justify-end">
            <AuthorizationCard onDocChange={setCarouselTheme}/>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-6 py-16">
        <h2 className="mb-6 text-2xl font-bold text-gray-900 md:text-3xl">Documentos suportados</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-4">
          {DFE_DOCUMENTS.map((doc) => (
            <Link
              key={doc.code}
              href={doc.href}
              className="group flex flex-col gap-3 rounded-xl border border-gray-200 p-5 transition-all hover:border-gray-300 hover:shadow-card-hover"
            >
              <div
                className="flex size-10 items-center justify-center rounded-lg text-white"
                style={{backgroundColor: doc.accent}}
              >
                {doc.icon}
              </div>
              <div>
                <p className="font-mono text-xs font-semibold text-gray-500 uppercase">{doc.code}</p>
                <p className="mt-0.5 text-sm font-semibold text-gray-900 group-hover:text-gray-700">{doc.title}</p>
                <p className="mt-1 text-xs leading-relaxed text-gray-500">{doc.description}</p>
              </div>
            </Link>
          ))}
        </div>
      </section>

      <section className="border-t border-gray-100 bg-gray-50/60">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <h2 className="mb-8 text-2xl font-bold text-gray-900 md:text-3xl">Como funciona</h2>
          <div className="grid gap-8 md:grid-cols-3">
            {FLOW_STEPS.map((step, i) => (
              <div key={step.label} className="relative pl-6">
                <span className="absolute left-0 top-1 font-mono text-xs text-primary-500">
                  {String(i + 1).padStart(2, '0')}
                </span>
                <p className="font-semibold text-gray-900">{step.label}</p>
                <p className="mt-1.5 text-sm leading-relaxed text-gray-600">{step.body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl space-y-20 px-6 py-20" aria-labelledby="screens-title">
        <div className="max-w-xl">
          <h2 id="screens-title" className="text-2xl font-bold text-gray-900 md:text-3xl">
            As telas que você vai usar
          </h2>
          <p className="mt-2 text-sm leading-relaxed text-gray-600">
            Sem tour guiado nem simulação: é o sistema rodando, com dados de demonstração.
          </p>
        </div>

        {SCREENS.map((screen, i) => (
          <div
            key={screen.slug}
            // A troca de lado é por `order`, então a proporção das colunas também
            // inverte — sem isso a captura cai na coluna estreita nas linhas ímpares.
            className={`grid items-center gap-8 md:gap-12 ${
              i % 2 === 1
                ? 'md:grid-cols-[minmax(0,1fr)_minmax(0,380px)]'
                : 'md:grid-cols-[minmax(0,380px)_minmax(0,1fr)]'
            }`}
          >
            <div className={i % 2 === 1 ? 'md:order-2' : undefined}>
              <p className="text-sm font-semibold text-primary-700">{screen.kicker}</p>
              <h3 className="mt-1.5 text-xl font-bold leading-snug text-gray-900 md:text-2xl">
                {screen.title}
              </h3>
              <p className="mt-3 text-sm leading-relaxed text-gray-600">{screen.body}</p>
              <Link
                href={screen.href}
                className="mt-4 inline-flex min-h-11 items-center gap-1.5 text-sm font-medium text-primary-700 hover:text-primary-800 sm:min-h-0"
              >
                {screen.linkLabel} <ArrowRight size={16} aria-hidden="true"/>
              </Link>
            </div>
            <figure
              className={`overflow-hidden rounded-xl border border-gray-200 bg-gray-50 shadow-card ${
                i % 2 === 1 ? 'md:order-1' : ''
              }`}
            >
              <Image
                src={`/guide/${screen.slug}.webp`}
                alt={screen.alt}
                width={1280}
                height={800}
                sizes="(max-width: 768px) 100vw, 700px"
                className="w-full"
                unoptimized
              />
            </figure>
          </div>
        ))}
      </section>

      <section className="mx-auto max-w-6xl px-6 py-16">
        <h2 className="mb-8 text-2xl font-bold text-gray-900 md:text-3xl">Por que escolher a CTech DFe</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {BENEFITS.map((benefit) => (
            <div key={benefit.title} className="rounded-xl border border-gray-200 bg-white p-5 shadow-card">
              <div className="flex size-10 items-center justify-center rounded-lg bg-primary-50 text-primary-700">
                <benefit.icon size={20}/>
              </div>
              <p className="mt-3 font-semibold text-gray-900">{benefit.title}</p>
              <p className="mt-1.5 text-sm leading-relaxed text-gray-600">{benefit.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="border-t border-gray-100 bg-gray-50/60">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <div className="mb-8">
            <h2 className="text-2xl font-bold text-gray-900 md:text-3xl">Planos</h2>
            <p className="mt-2 text-sm text-gray-600">
              Cobrança ainda não habilitada — valores de referência. Planos Basic e Max disponíveis sob
              consulta.
            </p>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {PLANS.map((plan) => (
              <div
                key={plan.name}
                className={`flex flex-col rounded-xl border p-5 ${
                  plan.highlight
                    ? 'border-primary-300 bg-white shadow-modal ring-1 ring-primary-200'
                    : 'border-gray-200 bg-white'
                }`}
              >
                {plan.highlight && (
                  <span
                    className="mb-2 inline-flex w-fit items-center rounded-full bg-primary-100 px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wide text-primary-700">
                    Mais popular
                  </span>
                )}
                <p className="font-semibold text-gray-900">{plan.name}</p>
                <p className="mt-0.5 text-xs text-gray-600">{plan.tagline}</p>
                <p className="mt-3 text-2xl font-bold text-gray-900">
                  {plan.price}
                  {plan.period && <span className="text-sm font-normal text-gray-500">{plan.period}</span>}
                </p>
                <ul className="mt-4 flex-1 space-y-2">
                  {plan.features.map((feature) => (
                    <li key={feature} className="flex items-start gap-2 text-sm text-gray-600">
                      <Check size={16} className="mt-0.5 shrink-0 text-primary-600"/>
                      {feature}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-6 py-16">
        <div className="mb-8">
          <h2 className="text-2xl font-bold text-gray-900 md:text-3xl">Em desenvolvimento</h2>
          <p className="mt-2 text-sm text-gray-600">Funcionalidades a caminho.</p>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {ROADMAP.map((item) => (
            <div key={item.title} className="rounded-xl border border-dashed border-gray-300 p-5">
              <span
                className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wide text-gray-500">
                Planejado
              </span>
              <p className="mt-2 font-semibold text-gray-900">{item.title}</p>
              <p className="mt-1 text-sm leading-relaxed text-gray-600">{item.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="bg-gradient-login">
        <div className="mx-auto max-w-6xl px-6 py-16 text-center">
          <h2 className="text-2xl font-bold text-gray-900 md:text-3xl">Comece a emitir grátis</h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-gray-600">
            Emita seus primeiros documentos fiscais em minutos, sem burocracia.
          </p>
          <div className="mt-6 flex flex-col items-center justify-center gap-3 sm:flex-row">
            <Button variant="brand" size="lg" render={<Link href={primaryHref}/>}>
              {primaryLabel}
            </Button>
            <Button variant="outline" size="lg" render={<Link href="/guide"/>}>
              Ler o guia
            </Button>
          </div>
        </div>
      </section>

      <footer className="border-t border-gray-100">
        <div
          className="mx-auto flex max-w-6xl flex-col items-center gap-3 px-6 py-8 text-sm text-gray-500 md:flex-row md:justify-between">
          <p>© {new Date().getFullYear()} A O CARVALHO TECH</p>
          <div className="flex items-center gap-4">
            <a href={DFE_TERMS_URL} className="hover:text-gray-800" target="_blank"
               rel="noreferrer">
              Termos de uso
            </a>
            <a href={PRIVACY_POLICY_URL} className="hover:text-gray-800" target="_blank"
               rel="noreferrer">
              Privacidade
            </a>
            <a href={ACCOUNTS_LEGAL_URL} className="hover:text-gray-800" target="_blank" rel="noreferrer">
              Central Jurídica
            </a>
          </div>
        </div>
      </footer>
    </div>
  )
}
