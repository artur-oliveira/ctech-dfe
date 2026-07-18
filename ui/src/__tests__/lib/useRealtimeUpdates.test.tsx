import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest'
import {act, renderHook} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import type {ReactNode} from 'react'
import {CLIENT_PING_INTERVAL_MS, CLIENT_PONG_TIMEOUT_MS} from '@aoctech/ws-client'
import {useRealtimeUpdates} from '@/lib/hooks/useRealtimeUpdates'

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_00000000000191'}}),
}))
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
})
