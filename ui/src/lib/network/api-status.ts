import type {NetworkStatus} from '@aoctech/ui';

/**
 * The API status as the HTTP client sees it. The probe lives in @aoctech/ui's
 * NetworkProvider; this mirror exists because axios interceptors run outside
 * React and must fail fast while the API is known to be down (otherwise every
 * mounted query becomes its own probe against a server that cannot answer).
 */
let current: NetworkStatus = 'online';

export const setApiStatus = (status: NetworkStatus): void => {
  current = status;
};

export const getApiStatus = (): NetworkStatus => current;

export class ApiUnavailableError extends Error {
  constructor(public readonly reason: 'offline' | 'server') {
    super(reason === 'offline' ? 'Sem conexão com a internet' : 'Servidor temporariamente indisponível');
    this.name = 'ApiUnavailableError';
  }
}

/** Throws while the probe says the API is down or the device is offline. */
export function requireApiAvailable(): void {
  if (current === 'offline') throw new ApiUnavailableError('offline');
  if (current === 'unavailable') throw new ApiUnavailableError('server');
}
