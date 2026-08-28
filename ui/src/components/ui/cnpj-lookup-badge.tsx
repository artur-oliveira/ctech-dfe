import {
  CNPJ_LOOKUP_SOURCE,
  type CnpjLookupState,
  type CnpjLookupSource,
} from '@/lib/hooks/useCnpjLookup'

interface CnpjLookupBadgeProps {
  state: CnpjLookupState
}

const SOURCE_LABELS: Record<CnpjLookupSource, string> = {
  [CNPJ_LOOKUP_SOURCE.OPEN_CNPJ]: 'CNPJá',
  [CNPJ_LOOKUP_SOURCE.SEFAZ]: 'SEFAZ',
}

const DATE_FORMAT = new Intl.DateTimeFormat('pt-BR', {dateStyle: 'short'})

function formattedDate(value: string | null): string | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : DATE_FORMAT.format(date)
}

export function CnpjLookupBadge({state}: CnpjLookupBadgeProps) {
  if (state.status === 'idle') return null

  if (state.status === 'searching') {
    const source = state.phase ? SOURCE_LABELS[state.phase] : 'bases cadastrais'
    return (
      <p role="status" aria-live="polite" className="mt-2 flex items-center gap-1.5 text-xs text-gray-500">
        <span aria-hidden="true"
              className="inline-block size-3 rounded-full border-2 border-gray-400 border-t-transparent motion-safe:animate-spin"/>
        Consultando {source}{state.currentUf ? ` (${state.currentUf})` : ''}…
      </p>
    )
  }

  if (state.status === 'found' && state.result) {
    const updatedAt = formattedDate(state.result.updatedAt)
    return (
      <div role="status" aria-live="polite"
           className="mt-2 rounded-md border border-brand-200 bg-brand-50/60 px-3 py-2 text-xs text-gray-700">
        <div className="flex flex-wrap items-center gap-1.5">
          <svg aria-hidden="true" width="13" height="13" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
               className="text-brand-700">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          <span className="font-medium text-brand-800">Dados consultados</span>
          {state.result.sources.map((source) => (
            <span key={source} className="rounded-full border border-brand-200 bg-white px-1.5 py-0.5 text-xs text-brand-800">
              {SOURCE_LABELS[source]}
            </span>
          ))}
          {updatedAt && <span className="text-gray-500">CNPJá atualizado em {updatedAt}</span>}
        </div>
        {(state.result.conflicts.length > 0 || state.result.warnings.length > 0) && (
          <ul className="mt-1.5 space-y-1 text-amber-800">
            {state.result.conflicts.map((conflict) => <li key={conflict.field}>• {conflict.message} Revise antes de salvar.</li>)}
            {state.result.warnings.map((warning) => <li key={warning}>• {warning}</li>)}
          </ul>
        )}
      </div>
    )
  }

  if (state.status === 'not_found') {
    return (
      <p role="status" aria-live="polite" className="mt-2 text-xs text-amber-700">
        {state.errorMessage ?? 'Cadastro não localizado nas bases consultadas.'}
      </p>
    )
  }

  if (state.status === 'no_certificate') {
    return (
      <p role="alert" className="mt-2 text-xs text-red-700">
        Organização sem certificado digital — consulta SEFAZ indisponível.
      </p>
    )
  }

  if (state.status === 'sefaz_rejection') {
    return <p role="alert" className="mt-2 text-xs text-red-700">{state.errorMessage ?? 'Rejeição no serviço da SEFAZ.'}</p>
  }

  return <p role="alert" className="mt-2 text-xs text-red-700">{state.errorMessage ?? 'Erro ao consultar os dados.'}</p>
}
