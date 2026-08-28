import {useEffect, useMemo, useState} from 'react'
import {apiClient} from '@/lib/api/client'
import {useDebounce} from '@/lib/hooks/useDebounce'

export interface IcmsAliqPreview {
  icms_aliq: string
  fcp_aliq: string
}

/**
 * Consulta debounced (300ms) de GET /v1.0/tax-tables/icms-aliq — a alíquota
 * que o backend resolveria para emit_uf/dest_uf/ncm sem nenhum override.
 * Devolve null enquanto emitUf/destUf não estão preenchidos, ou se a consulta
 * falhar — usado só para referência/warning, nunca bloqueia o formulário.
 */
export function useIcmsAliqPreview(emitUf?: string, destUf?: string, ncm?: string): IcmsAliqPreview | null {
  const [preview, setPreview] = useState<IcmsAliqPreview | null>(null)
  // useDebounce compara por identidade: um objeto literal novo a cada render
  // rearmaria o timer para sempre, num loop de fetch a cada 300ms.
  const query = useMemo(
    () => (emitUf && destUf ? {emitUf, destUf, ncm} : null),
    [emitUf, destUf, ncm],
  )
  const debouncedQuery = useDebounce(query, 300)

  useEffect(() => {
    let cancelled = false
    if (!debouncedQuery) {
      Promise.resolve().then(() => { if (!cancelled) setPreview(null) })
      return () => { cancelled = true }
    }
    apiClient.getIcmsAliqPreview(debouncedQuery).then((res) => {
      if (!cancelled) setPreview(res)
    }).catch(() => {
      if (!cancelled) setPreview(null)
    })
    return () => {
      cancelled = true
    }
  }, [debouncedQuery])

  return preview
}
