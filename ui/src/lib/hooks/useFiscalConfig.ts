'use client'

import {useQuery} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'
import type {CTeConfigOut, MDFeConfigOut, NFCeConfigOut, NFeConfigOut, NfseConfigOut} from '@/lib/types/api'

type FiscalConfigOut<V extends DocVariant> =
  V extends 'nfe' ? NFeConfigOut :
  V extends 'nfce' ? NFCeConfigOut :
  V extends 'cte' ? CTeConfigOut :
  V extends 'mdfe' ? MDFeConfigOut :
  NfseConfigOut

function fetchConfig(variant: DocVariant, pk: string) {
  switch (variant) {
    case 'nfe': return apiClient.getNFeConfig(pk)
    case 'nfce': return apiClient.getNFCeConfig(pk)
    case 'cte': return apiClient.getCTeConfig(pk)
    case 'mdfe': return apiClient.getMDFeConfig(pk)
    case 'nfse': return apiClient.getNfseConfig(pk)
  }
}

function configQueryKey(variant: DocVariant, pk: string) {
  switch (variant) {
    case 'nfe': return queryKeys.nfeConfig(pk)
    case 'nfce': return queryKeys.nfceConfig(pk)
    case 'cte': return queryKeys.cteConfig(pk)
    case 'mdfe': return queryKeys.mdfeConfig(pk)
    case 'nfse': return queryKeys.nfseConfig(pk)
  }
}

/**
 * Fetches a document type's fiscal config, treating a 404 as "not configured
 * yet" (`config: null`) instead of a query error — every emit/list page needs
 * this same distinction to gate on missing config.
 */
export function useFiscalConfig<V extends DocVariant>(variant: V, pk: string | undefined) {
  const query = useQuery<FiscalConfigOut<V> | null>({
    queryKey: configQueryKey(variant, pk ?? ''),
    queryFn: async () => {
      try {
        return (await fetchConfig(variant, pk!)) as FiscalConfigOut<V>
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null
        throw e
      }
    },
    enabled: !!pk,
  })

  return {
    config: query.data,
    isPending: query.isPending,
    isMissing: query.data === null,
    error: query.error,
  }
}
