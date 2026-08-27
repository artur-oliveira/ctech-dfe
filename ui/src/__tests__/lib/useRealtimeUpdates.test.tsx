import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest'
import {act, renderHook} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import type {ReactNode} from 'react'
import {CLIENT_PING_INTERVAL_MS, CLIENT_PONG_TIMEOUT_MS} from '@aoctech/ws-client'
import {useRealtimeUpdates} from '@/lib/hooks/useRealtimeUpdates'
import {queryKeys} from '@/lib/api/query-keys'

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_00000000000191'}}),
}))
vi.mock('sonner', () => ({toast: {success: vi.fn(), error: vi.fn(), info: vi.fn()}}))
vi.mock('@/lib/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api/client')>()
  return {...actual, getAccessToken: () => 'test-token'}
})

class FakeWebSocket {
  static OPEN = 1
  static instances: FakeWebSocket[] = []
  readyState = FakeWebSocket.OPEN
  onopen: (() => void) | null = null
  onmessage: ((evt: {data: string}) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  sent: string[] = []

  constructor(public url: string) {
    FakeWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.onclose?.()
  }
}

function wrapper({children}: {children: ReactNode}) {
  const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

const ORG_PK = 'CNPJ_00000000000191'

/** Wrapper over a caller-owned QueryClient, so invalidations can be asserted. */
function wrapperWith(qc: QueryClient) {
  return function Wrapper({children}: {children: ReactNode}) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  }
}

describe('useRealtimeUpdates', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('closes the socket when a client heartbeat pong never arrives', async () => {
    renderHook(() => useRealtimeUpdates(), {wrapper})
    const sock = FakeWebSocket.instances[0]
    act(() => sock.onopen?.())

    act(() => vi.advanceTimersByTime(CLIENT_PING_INTERVAL_MS))
    expect(sock.sent).toContain(JSON.stringify({type: 'ping'}))

    let closed = false
    sock.close = () => {
      closed = true
    }
    act(() => vi.advanceTimersByTime(CLIENT_PONG_TIMEOUT_MS))
    expect(closed).toBe(true)
  })

  it('reconnects immediately (no backoff) when the access token changes', async () => {
    const {apiClient} = await import('@/lib/api/client')
    renderHook(() => useRealtimeUpdates(), {wrapper})
    const first = FakeWebSocket.instances[0]
    act(() => first.onopen?.())

    let firstClosed = false
    first.close = () => {
      firstClosed = true
    }

    // Real production path: apiClient.setToken -> notifyTokenListeners ->
    // subscribeAccessToken's subscriber (wired in useRealtimeUpdates). The
    // reconnect is synchronous (no backoff delay), so no timer advance needed.
    act(() => apiClient.setToken('new-token'))

    expect(FakeWebSocket.instances.length).toBe(2)
    expect(firstClosed).toBe(true)
  })

  // Regressão: um resultado de inutilização só invalidava as queries de
  // documento, então a lacuna fechada e a faixa recém-enviada continuavam na
  // tela depois da notificação.
  it('invalidates the inutilização list and gaps on an INUT event result', () => {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
    const invalidate = vi.spyOn(qc, 'invalidateQueries').mockResolvedValue(undefined)
    renderHook(() => useRealtimeUpdates(), {wrapper: wrapperWith(qc)})
    const sock = FakeWebSocket.instances[0]
    act(() => sock.onopen?.())

    act(() => sock.onmessage?.({
      data: JSON.stringify({
        type: 'dfe_result',
        result_kind: 'event',
        access_key: 'INUT#prod#' + ORG_PK,
        doc_pk: 'prod#' + ORG_PK,
        table_name: 'nfces',
        event_type: 'INUT',
        status: 'success',
      }),
    }))

    const keys = invalidate.mock.calls.map(([arg]) => arg?.queryKey)
    expect(keys).toContainEqual(queryKeys.inutilizations.list('nfce', ORG_PK))
    expect(keys).toContainEqual(queryKeys.inutilizations.gaps('nfce', ORG_PK))
    // Nenhuma query de documento: a chave é sintética, não existe NFC-e alguma.
    expect(keys).not.toContainEqual(queryKeys.nfces.lists(ORG_PK))
  })

  it('still invalidates document queries on a document result', () => {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
    const invalidate = vi.spyOn(qc, 'invalidateQueries').mockResolvedValue(undefined)
    renderHook(() => useRealtimeUpdates(), {wrapper: wrapperWith(qc)})
    const sock = FakeWebSocket.instances[0]
    act(() => sock.onopen?.())

    act(() => sock.onmessage?.({
      data: JSON.stringify({
        type: 'dfe_result',
        result_kind: 'document',
        access_key: 'AK1',
        doc_pk: 'prod#' + ORG_PK,
        table_name: 'nfces',
        status: 'authorized',
      }),
    }))

    const keys = invalidate.mock.calls.map(([arg]) => arg?.queryKey)
    expect(keys).toContainEqual(queryKeys.nfces.lists(ORG_PK))
    expect(keys).toContainEqual(queryKeys.nfces.detail('AK1'))
  })
})
