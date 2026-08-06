'use client'

import {useCallback} from 'react'
import {useQueryClient} from '@tanstack/react-query'
import {toast} from 'sonner'
import {useAuth} from './useAuth'
import {useWebSocket, type WSStatus} from '@aoctech/ws-client'
import {queryKeys} from '@/lib/api/query-keys'
import {getAccessToken, subscribeAccessToken} from '@/lib/api/client'
import {resolveDfeResultToast} from '@/lib/utils/dfe-result-toast'

// `next dev` rewrites do not proxy the WebSocket upgrade, so local development
// points NEXT_PUBLIC_WS_URL straight at the API. Deployed environments leave it
// unset and fall back to the app origin, which CloudFront forwards to the ALB.
const WS_BASE_URL = process.env.NEXT_PUBLIC_WS_URL || process.env.NEXT_PUBLIC_API_URL || ''

function buildWsUrl(orgPk: string): string {
  const origin = WS_BASE_URL || window.location.origin
  const base = origin.replace(/^http/, 'ws')
  return `${base}/v1.0/ws?org_pk=${encodeURIComponent(orgPk)}`
}

interface RealtimeMessage {
  type: string
  // result_kind distinguishes a document result from a SEFAZ event result
  // (cancellation, encerramento). Sent by the worker on every dfe_result.
  result_kind?: string
  access_key?: string
  doc_pk?: string
  table_name?: string
  status?: string
  sefaz_motive?: string
  event_type?: string
  event_sk?: string
  emit_name?: string
  total?: string
  nsu?: number
}

// Query-key namespaces that own list/detail/events caches, keyed by the
// document's DynamoDB table name. CT-e has no list/detail queries yet.
const DOC_QUERY_KEYS = {
  nfes: queryKeys.nfes,
  nfces: queryKeys.nfces,
  mdfes: queryKeys.mdfes,
  nfses: queryKeys.nfses,
} as const

export function useRealtimeUpdates(): { wsStatus: WSStatus } {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()

  const token = getAccessToken()

  const wsUrl = token && selectedOrg?.pk
    ? buildWsUrl(selectedOrg.pk)
    : null

  const handleMessage = useCallback((data: unknown) => {
    const msg = data as RealtimeMessage
    if (!msg?.type || msg.type === 'ping' || msg.type === 'connected') return

    if (msg.type === 'dfe_result' && msg.access_key) {
      // Route invalidation by document type so each document's updates reach
      // its own queries (detail, list, and event history).
      const doc = DOC_QUERY_KEYS[msg.table_name as keyof typeof DOC_QUERY_KEYS]
      if (doc) {
        void qc.invalidateQueries({queryKey: doc.detail(msg.access_key)})
        void qc.invalidateQueries({queryKey: doc.lists(selectedOrg?.pk)})
        void qc.invalidateQueries({queryKey: doc.events(msg.access_key)})
      }
      // Resolve the toast from the result — event results report the event
      // outcome, not the (possibly reverted) document status.
      const {variant, message} = resolveDfeResultToast(msg)
      toast[variant](message)
    }

    if (msg.type === 'new_distribution_nfe') {
      void qc.invalidateQueries({queryKey: queryKeys.distributions.history('nfe', selectedOrg?.pk)})
      const label = msg.emit_name ? ` de ${msg.emit_name}` : ''
      const value = msg.total ? ` — R$ ${parseFloat(msg.total).toLocaleString('pt-BR', {minimumFractionDigits: 2})}` : ''
      toast.info(`Nova NF-e recebida${label}${value}`)
    }

    if (msg.type === 'new_distribution_cte') {
      void qc.invalidateQueries({queryKey: queryKeys.distributions.history('cte', selectedOrg?.pk)})
      const label = msg.emit_name ? ` de ${msg.emit_name}` : ''
      toast.info(`Novo CT-e recebido${label}`)
    }

    if (msg.type === 'new_distribution_mdfe') {
      void qc.invalidateQueries({queryKey: queryKeys.distributions.history('mdfe', selectedOrg?.pk)})
      toast.info('Novo MDF-e recebido')
    }
  }, [qc, selectedOrg?.pk])

  const {status: wsStatus} = useWebSocket({
    url: wsUrl,
    onMessage: handleMessage,
    enabled: !!wsUrl,
    authToken: token ?? undefined,
    subscribeToken: subscribeAccessToken,
  })

  return {wsStatus}
}