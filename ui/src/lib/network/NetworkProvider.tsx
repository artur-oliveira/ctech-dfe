'use client'

import React, {useEffect, useRef, useSyncExternalStore} from 'react'
import {useQueryClient} from '@tanstack/react-query'
import {CloudOff, RefreshCw, WifiOff} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {MOCK_ENABLED} from '@/lib/mock/env'
import {
  checkApiLiveness,
  getApiLivenessSnapshot,
  getServerApiLivenessSnapshot,
  livenessPollDelay,
  markApiOffline,
  subscribeApiLiveness,
  type ApiLivenessSnapshot,
} from './liveness'

export function useApiLiveness(): ApiLivenessSnapshot {
  return useSyncExternalStore(subscribeApiLiveness, getApiLivenessSnapshot, getServerApiLivenessSnapshot)
}

/**
 * Owns the health poll and the outage notice.
 *
 * Mounted once, inside the QueryClientProvider: when the API comes back it
 * refetches what is on screen, so recovery costs the user nothing — no reload,
 * no "tente novamente" on every card.
 */
export function NetworkProvider({children}: { children: React.ReactNode }) {
  const state = useApiLiveness()
  const queryClient = useQueryClient()
  const checkNowRef = useRef<() => void>(() => undefined)

  useEffect(() => {
    if (MOCK_ENABLED) return
    let timer: ReturnType<typeof setTimeout> | null = null
    let failures = 0
    let cancelled = false

    async function runCheck() {
      if (timer) clearTimeout(timer)
      const wasUnavailable = getApiLivenessSnapshot().status === 'unavailable'
      const available = await checkApiLiveness()
      if (cancelled) return
      failures = available ? 0 : failures + 1
      if (available && wasUnavailable) void queryClient.refetchQueries({type: 'active'})
      timer = setTimeout(() => void runCheck(), livenessPollDelay(failures))
    }

    checkNowRef.current = () => void runCheck()
    void runCheck()

    const onOnline = () => void runCheck()
    const onOffline = () => {
      if (timer) clearTimeout(timer)
      markApiOffline()
      failures += 1
      timer = setTimeout(() => void runCheck(), livenessPollDelay(failures))
    }
    const onVisibility = () => {
      if (document.visibilityState === 'visible' && getApiLivenessSnapshot().status !== 'available') {
        void runCheck()
      }
    }

    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      cancelled = true
      checkNowRef.current = () => undefined
      if (timer) clearTimeout(timer)
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [queryClient])

  return (
    <>
      {children}
      {state.status === 'unavailable' && (
        <NetworkStatusBanner
          offline={state.reason === 'offline'}
          onRetry={() => checkNowRef.current()}
        />
      )}
    </>
  )
}

/**
 * The notice sits at the bottom edge, not under the topbar: navigation is how
 * someone gets to a screen that still works from cache, and covering it is the
 * one thing an outage banner must not do.
 */
function NetworkStatusBanner({offline, onRetry}: { offline: boolean; onRetry: () => void }) {
  const Icon = offline ? WifiOff : CloudOff
  return (
    <aside
      role="status"
      aria-live="polite"
      className="fixed inset-x-0 bottom-0 z-50 border-t border-amber-200 bg-amber-50 px-4 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-3 md:px-8"
    >
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3 min-w-0">
          <Icon aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-amber-700"/>
          <p className="text-sm text-amber-900 text-pretty">
            <strong className="font-semibold">
              {offline ? 'Você está sem internet.' : 'Servidor temporariamente indisponível.'}
            </strong>{' '}
            {offline
              ? 'Nada foi perdido — a tela volta a atualizar assim que a conexão retornar.'
              : 'Seus dados estão seguros. Estamos verificando a conexão sem sobrecarregar o servidor.'}
          </p>
        </div>
        <Button type="button" size="sm" variant="outline" onClick={onRetry} className="shrink-0 h-11 sm:h-9">
          <RefreshCw aria-hidden="true"/> Verificar agora
        </Button>
      </div>
    </aside>
  )
}
