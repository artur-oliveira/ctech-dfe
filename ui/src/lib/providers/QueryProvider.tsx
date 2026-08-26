'use client'

import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {ReactNode, useState} from 'react'
import {NetworkProvider} from '@/lib/network/NetworkProvider'

export function QueryProvider({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60_000,
            // ApiClient already gives safe reads one bounded, jittered retry
            // budget and fails fast while the API is known to be down. A second
            // retry layer here multiplies the same outage by three.
            retry: false,
          },
          mutations: {retry: false},
        },
      }),
  )

  return (
    <QueryClientProvider client={client}>
      <NetworkProvider>{children}</NetworkProvider>
    </QueryClientProvider>
  )
}
