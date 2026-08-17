'use client'

import {DOCUMENT_METERS, METER_LABELS, QUOTA_UNLIMITED} from '@/lib/constants/billing'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

/** What each document type is for, in the words of someone who has to pick. */
const DOC_PURPOSE: Record<DocVariant, string> = {
  nfe: 'Venda de mercadoria para outra empresa ou para fora do estado.',
  nfce: 'Venda no balcão, direto ao consumidor.',
  cte: 'Frete: o conhecimento de transporte de uma carga.',
  mdfe: 'Manifesto da viagem, que agrupa as notas e os conhecimentos a bordo.',
  nfse: 'Prestação de serviço, emitida pela prefeitura.',
}

interface DocumentPickerProps {
  /** Limit per meter from the plan; -1 unlimited, absent means not granted. */
  quotas: Record<string, number>
  /** Types already configured — shown as done, never unselectable. */
  configured: Record<DocVariant, boolean>
  selected: DocVariant[]
  onToggle: (variant: DocVariant) => void
}

function allowance(limit: number | undefined): {allowed: boolean; text: string} {
  if (limit === undefined) return {allowed: false, text: 'Não incluído no seu plano'}
  if (limit === 0) return {allowed: false, text: 'Não incluído no seu plano'}
  if (limit === QUOTA_UNLIMITED) return {allowed: true, text: 'Sem limite mensal'}
  return {allowed: true, text: `${limit.toLocaleString('pt-BR')} por mês`}
}

/**
 * Which documents the company issues.
 *
 * The answer is not a preference the product stores — it is which fiscal
 * configurations get created. Asking it here, once, replaces five settings tabs
 * a new user has no way to know they need to visit.
 *
 * A type the plan does not grant stays visible and disabled: "not in your plan"
 * and "does not exist" are different, and hiding the first one makes an upgrade
 * impossible to discover.
 */
export function DocumentPicker({quotas, configured, selected, onToggle}: DocumentPickerProps) {
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="sr-only">Documentos que a empresa emite</legend>
      {DOCUMENT_METERS.map((meter) => {
        const variant = meter as DocVariant
        const {allowed, text} = allowance(quotas[meter])
        const isConfigured = configured[variant]
        const isChecked = isConfigured || selected.includes(variant)

        return (
          <label
            key={variant}
            className={`flex min-h-11 items-start gap-3 rounded-xl border p-4 transition-colors ${
              !allowed || isConfigured
                ? 'cursor-default border-gray-200 bg-gray-50/60'
                : isChecked
                  ? 'cursor-pointer border-brand-600 bg-white ring-1 ring-brand-600'
                  : 'cursor-pointer border-gray-200 bg-white hover:border-gray-300'
            }`}
          >
            <input
              type="checkbox"
              className="mt-0.5 size-4 shrink-0 accent-brand-600"
              checked={isChecked}
              disabled={!allowed || isConfigured}
              onChange={() => onToggle(variant)}
            />
            <span className="min-w-0 flex-1">
              <span className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-semibold text-gray-900">{METER_LABELS[meter]}</span>
                {isConfigured && (
                  <span className="rounded-md bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
                    Já configurado
                  </span>
                )}
              </span>
              <span className="mt-0.5 block text-sm leading-relaxed text-gray-600 text-pretty">
                {DOC_PURPOSE[variant]}
              </span>
              <span className={`mt-1 block text-xs ${allowed ? 'text-gray-500' : 'text-warning'}`}>{text}</span>
            </span>
          </label>
        )
      })}
    </fieldset>
  )
}
