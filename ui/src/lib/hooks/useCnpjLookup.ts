'use client'

import {useCallback, useRef, useState} from 'react'
import {apiClient, ApiError} from '@/lib/api/client'
import type {LookupOrganizationOut} from '@/lib/types/api'
import {bfsUfOrder} from '@/lib/utils/uf-graph'

export type CnpjLookupStatus =
  | 'idle'
  | 'searching'       // actively querying
  | 'found'           // CNPJ found, result available
  | 'not_found'       // exhausted all UFs
  | 'no_certificate'  // org has no cert — caller should surface this prominently
  | 'sefaz_rejection'  // org has no cert — caller should surface this prominently
  | 'error'           // unexpected error

export interface CnpjLookupState {
  status: CnpjLookupStatus
  result: LookupOrganizationOut | null
  currentUf: string | null
  errorMessage: string | null
}

const NO_CERTIFICATE_TYPE = '/problems/no-certificate'
const SEFAZ_REJECTION_TYPE = '/problems/sefaz-rejection'

/**
 * Hook that performs BFS ConsultaCadastro across Brazilian UFs.
 *
 * Usage:
 *   const {state, lookup, cancel} = useCnpjLookup()
 *   // When CNPJ is fully entered:
 *   lookup(cnpj14digits, orgStateFederation)
 */
export function useCnpjLookup() {
  const [state, setState] = useState<CnpjLookupState>({
    status: 'idle',
    result: null,
    currentUf: null,
    errorMessage: null,
  })

  // Ref used to cancel an in-progress search
  const abortRef = useRef(false)

  const cancel = useCallback(() => {
    abortRef.current = true
    setState((s) => s.status === 'searching' ? {...s, status: 'idle', currentUf: null} : s)
  }, [])

  const lookup = useCallback(async (cpfCnpj: string, orgUf: string) => {
    const clean = cpfCnpj.replace(/\D/g, '')
    if (clean.length !== 11 && clean.length !== 14) return

    abortRef.current = false
    setState({status: 'searching', result: null, currentUf: orgUf, errorMessage: null})

    const ufsToTry = bfsUfOrder(orgUf || 'SP')

    for (const uf of ufsToTry) {
      if (abortRef.current) return

      setState((s) => ({...s, currentUf: uf}))

      try {
        const result = await apiClient.lookupOrganization(clean, uf)
        if (abortRef.current) return
        setState({status: 'found', result, currentUf: null, errorMessage: null})
        setTimeout(() => {
          setState({status: 'idle', result: null, currentUf: null, errorMessage: null})
        }, 5000);
        return
      } catch (err) {
        if (abortRef.current) return

        if (err instanceof ApiError) {
          // No certificate — stop immediately, surface to user
          if (err.raw && typeof err.raw === 'object' && (err.raw as { type?: string }).type === NO_CERTIFICATE_TYPE) {
            setState({status: 'no_certificate', result: null, currentUf: null, errorMessage: err.detail})
            return
          }
          if (err.raw && typeof err.raw === 'object' && (err.raw as { type?: string }).type === SEFAZ_REJECTION_TYPE) {
            setState({status: 'sefaz_rejection', result: null, currentUf: null, errorMessage: err.detail})
            return
          }
          // Not found in this UF — continue BFS
          if (err.status === 404) continue
          // Other API error — stop
          setState({status: 'error', result: null, currentUf: null, errorMessage: err.detail})
          return
        }

        // Network or unexpected error — stop
        setState({status: 'error', result: null, currentUf: null, errorMessage: 'Erro de conexão'})
        return
      }
    }

    // All UFs tried, not found
    setState({status: 'not_found', result: null, currentUf: null, errorMessage: null})
  }, [])

  const reset = useCallback(() => {
    abortRef.current = true
    setState({status: 'idle', result: null, currentUf: null, errorMessage: null})
  }, [])

  return {state, lookup, cancel, reset}
}
