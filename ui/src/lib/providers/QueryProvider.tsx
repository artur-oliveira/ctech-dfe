'use client';

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {ReactNode, useState} from 'react';
import {ApiError} from '@/lib/api/client';
import {NetworkBoundary} from '@/lib/network/NetworkBoundary';

/** 403 é resposta definitiva (falta permissão ou escopo): refazer a mesma requisição a cada montagem ou foco só repete o erro. */
const isForbidden = (error: unknown) => error instanceof ApiError && error.status === 403;

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
            retryOnMount: (query) => !isForbidden(query.state.error),
            refetchOnWindowFocus: (query) => !isForbidden(query.state.error),
            refetchOnReconnect: (query) => !isForbidden(query.state.error),
          },
          mutations: {retry: false},
        },
      }),
  );

  return (
    <QueryClientProvider client={client}>
      <NetworkBoundary onReconnect={() => void client.refetchQueries({type: 'active'})}>{children}</NetworkBoundary>
    </QueryClientProvider>
  );
}
