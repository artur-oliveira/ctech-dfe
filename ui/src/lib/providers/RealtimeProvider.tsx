'use client'

import {createContext, useContext} from 'react'
import {useRealtimeUpdates} from '@/lib/hooks/useRealtimeUpdates'
import type {WSStatus} from '@aoctech/ws-client'
import React from 'react'

const RealtimeStatusContext = createContext<WSStatus>('disconnected')

/** Current WebSocket connection status — for a future connection indicator. */
export function useRealtimeStatus(): WSStatus {
  return useContext(RealtimeStatusContext)
}

export function RealtimeProvider({children}: {children: React.ReactNode}) {
  const {wsStatus} = useRealtimeUpdates()
  return <RealtimeStatusContext.Provider value={wsStatus}>{children}</RealtimeStatusContext.Provider>
}
