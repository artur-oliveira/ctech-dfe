'use client'

import {useEffect, useLayoutEffect, useRef, useState} from 'react'

export type WSStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

export interface UseWebSocketOptions {
  url: string | null
  onMessage: (data: unknown) => void
  enabled?: boolean
}

const BASE_DELAY_MS = 1_000
const MAX_DELAY_MS = 30_000
const MAX_RECONNECT_ATTEMPTS = 10

export function useWebSocket({url, onMessage, enabled = true}: UseWebSocketOptions): {status: WSStatus} {
  const [status, setStatus] = useState<WSStatus>('disconnected')
  const attemptsRef = useRef(0)
  const onMessageRef = useRef(onMessage)

  useLayoutEffect(() => {
    onMessageRef.current = onMessage
  })

  useEffect(() => {
    if (!url || !enabled) return

    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let ws: WebSocket | null = null

    function connect() {
      if (cancelled) return

      setStatus('connecting')
      ws = new WebSocket(url!)

      ws.onopen = () => {
        attemptsRef.current = 0
        setStatus('connected')
      }

      ws.onmessage = (evt) => {
        try {
          const data = JSON.parse(evt.data as string)
          if (data?.type === 'ping') {
            const sock = evt.target as WebSocket
            if (sock.readyState === WebSocket.OPEN) {
              sock.send(JSON.stringify({type: 'pong'}))
            }
          }
          onMessageRef.current(data)
        } catch {
          // malformed frame — ignore
        }
      }

      ws.onerror = () => {
        setStatus('error')
      }

      ws.onclose = () => {
        ws = null
        setStatus('disconnected')
        if (cancelled) return

        attemptsRef.current++
        if (attemptsRef.current > MAX_RECONNECT_ATTEMPTS) return

        const delay = Math.min(BASE_DELAY_MS * 2 ** (attemptsRef.current - 1), MAX_DELAY_MS)
        timer = setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
      ws?.close(1000)
      ws = null
    }
  }, [url, enabled])

  return {status}
}