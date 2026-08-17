'use client'

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
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

function saveConfig(variant: DocVariant, pk: string, data: Record<string, unknown>): Promise<unknown> {
  switch (variant) {
    case 'nfe': return apiClient.upsertNFeConfig(pk, data)
    case 'nfce': return apiClient.upsertNFCeConfig(pk, data)
    case 'cte': return apiClient.upsertCTeConfig(pk, data)
    case 'mdfe': return apiClient.upsertMDFeConfig(pk, data)
    case 'nfse': return apiClient.upsertNfseConfig(pk, data)
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

/**
 * Saves a document type's fiscal config.
 *
 * The five upsert endpoints differ only by document type, so the settings page
 * and the onboarding flow share this one mutation rather than each carrying its
 * own switch — the second copy is where the invalidation gets forgotten.
 */
export function useFiscalConfigMutation(variant: DocVariant, pk: string | undefined) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Record<string, unknown>) => saveConfig(variant, pk!, data),
    onSuccess: () => qc.invalidateQueries({queryKey: configQueryKey(variant, pk ?? '')}),
  })
}
