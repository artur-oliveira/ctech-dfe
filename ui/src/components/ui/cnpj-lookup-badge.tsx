import type {CnpjLookupState} from '@/lib/hooks/useCnpjLookup'

interface CnpjLookupBadgeProps {
  state: CnpjLookupState
}

export function CnpjLookupBadge({state}: CnpjLookupBadgeProps) {
  if (state.status === 'idle') return null

  if (state.status === 'searching') {
    return (
      <p className="mt-1 flex items-center gap-1.5 text-xs text-gray-500">
        <span className="inline-block w-3 h-3 rounded-full border-2 border-gray-400 border-t-transparent animate-spin"/>
        Consultando SEFAZ{state.currentUf ? ` (${state.currentUf})` : ''}…
      </p>
    )
  }

  if (state.status === 'found') {
    return (
      <p className="mt-1 flex items-center gap-1 text-xs text-green-700">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"
             strokeLinecap="round" strokeLinejoin="round">
          <polyline points="20 6 9 17 4 12"/>
        </svg>
        Dados preenchidos via SEFAZ ({state.result?.uf})
      </p>
    )
  }

  if (state.status === 'not_found') {
    return (
      <p className="mt-1 text-xs text-amber-600">
        CNPJ não localizado em nenhuma UF da SEFAZ.
      </p>
    )
  }

  if (state.status === 'no_certificate') {
    return (
      <p className="mt-1 text-xs text-red-600">
        Organização sem certificado digital - consulta SEFAZ indisponível.
      </p>
    )
  }

  if (state.status === 'sefaz_rejection') {
    return (
      <p className="mt-1 text-xs text-red-600">
        {state.errorMessage ?? 'Rejeição no serviço da sefaz'}
      </p>
    )
  }

  if (state.status === 'error') {
    return (
      <p className="mt-1 text-xs text-red-600">
        {state.errorMessage ?? 'Erro ao consultar SEFAZ.'}
      </p>
    )
  }

  return null
}
