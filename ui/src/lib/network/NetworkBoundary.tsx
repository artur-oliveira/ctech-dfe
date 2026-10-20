'use client';

import {useEffect, type ReactNode} from 'react';
import {NetworkProvider, useNetwork} from '@aoctech/ui';
import {MOCK_ENABLED} from '@/lib/mock/env';
import {setApiStatus} from './api-status';
import {redirectOnMaintenance} from './maintenance';

const HEALTH_URL = `${(process.env.NEXT_PUBLIC_API_URL ?? '').replace(/\/$/, '')}/v1.0/health`;

// The mock API has no backend to probe.
const mockFetch: typeof fetch = async () => new Response('{}', {status: 200});

/** Mirrors the probe status for the axios client (see api-status.ts). */
function StatusMirror() {
  const {status} = useNetwork();
  useEffect(() => {
    setApiStatus(status);
  }, [status]);
  return null;
}

/**
 * One health probe for the app, the offline banner and the refetch on
 * reconnect. Mounted inside the QueryClientProvider; `onReconnect` brings what
 * is on screen back to life with no reload.
 */
export function NetworkBoundary({children, onReconnect}: {children: ReactNode; onReconnect: () => void}) {
  return (
    <NetworkProvider
      url={HEALTH_URL}
      fetch={MOCK_ENABLED ? mockFetch : undefined}
      locale="pt-BR"
      labels={{
        offline: 'Você está sem internet.',
        offlineHint: 'Nada foi perdido; a tela volta a atualizar assim que a conexão retornar.',
        unavailable: 'Servidor temporariamente indisponível.',
        unavailableHint: 'Seus dados estão seguros. Estamos verificando a conexão sem sobrecarregar o servidor.',
        retry: 'Verificar agora',
      }}
      onReconnect={onReconnect}
      onUnavailable={() => void redirectOnMaintenance(503)}
    >
      <StatusMirror/>
      {children}
    </NetworkProvider>
  );
}
