import type { ReactNode } from 'react'

export function LegalPage({ title, updatedAt, children }: { title: string; updatedAt: string; children: ReactNode }) {
  return (
    <div className="min-h-screen bg-white">
      <div className="mx-auto max-w-3xl px-6 py-12">
        <div className="flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary-600 text-white">
            <span className="text-base">📄</span>
          </div>
          <span className="font-semibold text-gray-900">CTech DFe</span>
        </div>

        <h1 className="mt-6 text-2xl font-bold tracking-tight text-gray-900">{title}</h1>
        <p className="mt-1 text-sm text-gray-500">Última atualização: {updatedAt}</p>

        <article className="mt-8 space-y-8 text-sm leading-relaxed text-gray-700">{children}</article>
      </div>
    </div>
  )
}

export function LegalSection({ heading, children }: { heading: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-base font-semibold tracking-tight text-gray-900">{heading}</h2>
      <div className="space-y-3">{children}</div>
    </section>
  )
}
