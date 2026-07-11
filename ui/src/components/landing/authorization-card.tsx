'use client'

import { useEffect, useState } from 'react'
import { DFE_DOCUMENTS } from '@/lib/constants/dfe-documents'
import type { DfeThemeKey } from '@/lib/theme/dfe-theme'

type Stage = 'draft' | 'transmitting' | 'authorized'

interface CarouselDoc {
  code: string
  modelo: string
  fullName: string
  theme: DfeThemeKey
}

const CAROUSEL_DOCS: CarouselDoc[] = [
  { code: 'NF-e', modelo: 'Modelo 55', fullName: 'Nota Fiscal Eletrônica', theme: 'nfe' },
  { code: 'NFC-e', modelo: 'Modelo 65', fullName: 'Nota Fiscal de Consumidor Eletrônica', theme: 'nfce' },
  { code: 'CT-e', modelo: 'Modelo 57', fullName: 'Conhecimento de Transporte Eletrônico', theme: 'cte' },
  { code: 'MDF-e', modelo: 'Modelo 58', fullName: 'Manifesto Eletrônico de Documentos Fiscais', theme: 'mdfe' },
]

function accentFor(code: string): string {
  return DFE_DOCUMENTS.find((d) => d.code === code)?.accent ?? '#2ea87f'
}

const ACCESS_KEY_GROUPS = Array.from({ length: 11 }, () => '9999')

// Timings lean slower than the real thing on the fast end, but the SEFAZ
// round trip is genuinely unpredictable in production — this is a floor, not
// a promise.
const STAGE_TIMING_MS: Record<Stage, number> = {
  draft: 1000,
  transmitting: 2800,
  authorized: 0,
}
const HOLD_AUTHORIZED_MS = 2600
const KEY_REVEAL_INTERVAL_MS = 70

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

interface AuthorizationCardProps {
  onDocChange?: (theme: DfeThemeKey) => void
}

// A self-playing simulation of what actually happens when this product
// issues a document: the XML is assembled, sent to SEFAZ, and comes back
// with an access key and an authorization protocol. Cycles through NF-e,
// NFC-e, CT-e and MDF-e; rests briefly on each before moving to the next.
export function AuthorizationCard({ onDocChange }: AuthorizationCardProps) {
  const [reducedMotion] = useState(prefersReducedMotion)
  const [docIndex, setDocIndex] = useState(0)
  const [stage, setStage] = useState<Stage>(() => (reducedMotion ? 'authorized' : 'draft'))
  const [visibleGroups, setVisibleGroups] = useState(() => (reducedMotion ? ACCESS_KEY_GROUPS.length : 0))

  const doc = CAROUSEL_DOCS[docIndex]

  useEffect(() => {
    onDocChange?.(doc.theme)
  }, [doc.theme, onDocChange])

  useEffect(() => {
    if (reducedMotion) return

    const timers: ReturnType<typeof setTimeout>[] = []
    timers.push(setTimeout(() => setStage('transmitting'), STAGE_TIMING_MS.draft))
    timers.push(
      setTimeout(() => setStage('authorized'), STAGE_TIMING_MS.draft + STAGE_TIMING_MS.transmitting),
    )
    return () => timers.forEach(clearTimeout)
  }, [reducedMotion, docIndex])

  useEffect(() => {
    if (reducedMotion || stage !== 'authorized') return
    const groupInterval = setInterval(() => {
      setVisibleGroups((n) => (n >= ACCESS_KEY_GROUPS.length ? n : n + 1))
    }, KEY_REVEAL_INTERVAL_MS)
    return () => clearInterval(groupInterval)
  }, [reducedMotion, stage])

  useEffect(() => {
    if (reducedMotion || stage !== 'authorized') return
    const advance = setTimeout(() => {
      setDocIndex((i) => (i + 1) % CAROUSEL_DOCS.length)
      setStage('draft')
      setVisibleGroups(0)
    }, HOLD_AUTHORIZED_MS)
    return () => clearTimeout(advance)
  }, [reducedMotion, stage])

  const accent = accentFor(doc.code)

  return (
    <div className="w-full max-w-sm rounded-2xl border border-primary-200 bg-white shadow-modal">
      <div className="flex items-center justify-between border-b border-dashed border-primary-200 px-5 py-4">
        <div>
          <p className="font-mono text-[0.65rem] tracking-widest uppercase" style={{ color: accent }}>
            {doc.code} · {doc.modelo}
          </p>
          <p className="text-sm font-semibold text-gray-900">{doc.fullName}</p>
        </div>
        <StatusPill stage={stage} accent={accent} />
      </div>

      <div className="space-y-3 px-5 py-4">
        <div className="space-y-1">
          <p className="font-mono text-[0.65rem] tracking-widest text-gray-400 uppercase">Chave de acesso</p>
          <p className="font-mono text-[0.8rem] leading-relaxed text-gray-800 tabular-nums">
            {ACCESS_KEY_GROUPS.map((group, i) => (
              <span
                key={i}
                className={i < visibleGroups ? 'opacity-100' : 'opacity-0'}
                style={{ transitionProperty: 'opacity', transitionDuration: '150ms' }}
              >
                {group}
                {i < ACCESS_KEY_GROUPS.length - 1 ? ' ' : ''}
              </span>
            ))}
          </p>
        </div>

        <div className="flex items-center gap-4 border-t border-gray-100 pt-3">
          <Field label="UF" value="PI" />
          <Field label="Ambiente" value="Produção" />
          <Field label="Protocolo" value={stage === 'authorized' ? '999999999999999' : '—'} />
        </div>
      </div>
    </div>
  )
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-0.5">
      <p className="font-mono text-[0.6rem] tracking-widest text-gray-400 uppercase">{label}</p>
      <p className="font-mono text-xs text-gray-700 tabular-nums">{value}</p>
    </div>
  )
}

function StatusPill({ stage, accent }: { stage: Stage; accent: string }) {
  if (stage === 'authorized') {
    return (
      <span
        className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[0.7rem] font-medium text-white"
        style={{ backgroundColor: accent }}
      >
        <span className="size-1.5 rounded-full bg-white" />
        Autorizado
      </span>
    )
  }
  if (stage === 'transmitting') {
    return (
      <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-100 px-2.5 py-1 text-[0.7rem] font-medium text-amber-800">
        <span className="size-1.5 animate-pulse rounded-full bg-amber-500" />
        Transmitindo à SEFAZ
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-[0.7rem] font-medium text-gray-600">
      <span className="size-1.5 rounded-full bg-gray-400" />
      Gerando XML
    </span>
  )
}
