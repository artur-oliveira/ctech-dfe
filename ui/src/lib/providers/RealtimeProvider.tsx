'use client'

import {useRealtimeUpdates} from '@/lib/hooks/useRealtimeUpdates'
import React from 'react'

export function RealtimeProvider({children}: {children: React.ReactNode}) {
  useRealtimeUpdates()
  return <>{children}</>
}
