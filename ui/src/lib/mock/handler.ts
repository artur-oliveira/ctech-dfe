/**
 * Axios adapter that serves fixtures for the dev mock API. Attached to the
 * singleton `apiClient` instance when `NEXT_PUBLIC_MOCK_API=true` (see index.ts).
 * Routes by method + path; returns an AxiosResponse on success and rejects an
 * AxiosError on a forced error so `client.ts`'s response interceptor turns it
 * into a real `ApiError` exactly as the backend would.
 */

import {AxiosError, type AxiosAdapter, type AxiosResponse, type InternalAxiosRequestConfig} from 'axios'
import {
  auditLogsFixture,
  certificatesFixture,
  cteConfigFixture,
  distributionsFixture,
  mdfeConfigFixture,
  mdfesFixture,
  meFixture,
  membersFixture,
  nfceConfigFixture,
  nfcesFixture,
  nfeConfigFixture,
  nfesFixture,
  organizationsFixture,
  personsFixture,
  productsFixture,
  rolesFixture,
  vehiclesFixture,
} from './fixtures'
import {getMockState, shouldError} from './state'

const MOCK_LATENCY_MS = 250

function paginated<T>(items: T[]): { items: T[]; next_cursor: string | null; has_next: boolean; previous_cursor: string | null; has_previous: boolean } {
  return {
    items,
    next_cursor: null,
    has_next: false,
    previous_cursor: null,
    has_previous: false,
  }
}

/** Echoes a created/updated entity with a synthetic key so forms resolve. */
function echo(body: unknown, suffix: string) {
  const base = (body && typeof body === 'object' ? body : {}) as Record<string, unknown>
  return {pk: `mock-${suffix}-${Date.now()}`, sk: `MOCK#${suffix}`, ...base}
}

type RouteResult = {data: unknown; status?: number} | {error: {status: number; data: unknown}}

function route(method: string, path: string, body: unknown): RouteResult {
  const m = method.toLowerCase()
  const key = `${m} ${path}`

  // Auth (always succeed — see SAFE_PATHS in state.ts)
  if (key === 'get /v1.0/auth/me') return {data: meFixture}
  if (key === 'get /v1.0/auth/roles') return {data: rolesFixture}

  // Organizations
  if (key === 'get /v1.0/organizations') return {data: organizationsFixture}
  if (m === 'post' && path === '/v1.0/organizations') return {data: organizationsFixture[0]}
  if (m === 'put' && path.startsWith('/v1.0/organizations/') && !path.includes('-config')) return {data: organizationsFixture[0]}
  if (m === 'delete' && path.startsWith('/v1.0/organizations/')) return {data: undefined}

  // Configs (path-based org in URL)
  if (path.endsWith('/nfe-config')) return {data: nfeConfigFixture}
  if (path.endsWith('/nfce-config')) return {data: nfceConfigFixture}
  if (path.endsWith('/cte-config')) return {data: cteConfigFixture}
  if (path.endsWith('/mdfe-config')) return {data: mdfeConfigFixture}

  // Certificates / members
  if (path.endsWith('/certificates')) return {data: certificatesFixture}
  if (m === 'post' && path.endsWith('/certificates')) return {data: echo(body, 'cert')}
  if (m === 'delete' && path.includes('/certificates/')) return {data: undefined}
  if (path.endsWith('/members')) return {data: membersFixture}
  if (path.endsWith('/invitations')) return {data: []}

  // Catalog
  if (key === 'get /v1.0/products') return {data: paginated(productsFixture)}
  if (m === 'post' && path === '/v1.0/products') return {data: echo(body, 'product')}
  if (m === 'put' && path.startsWith('/v1.0/products/')) return {data: echo(body, 'product')}
  if (m === 'delete' && path.startsWith('/v1.0/products/')) return {data: undefined}

  if (key === 'get /v1.0/vehicles') return {data: paginated(vehiclesFixture)}
  if (m === 'post' && path === '/v1.0/vehicles') return {data: echo(body, 'vehicle')}
  if (m === 'put' && path.startsWith('/v1.0/vehicles/')) return {data: echo(body, 'vehicle')}
  if (m === 'delete' && path.startsWith('/v1.0/vehicles/')) return {data: undefined}

  if (key === 'get /v1.0/persons') return {data: paginated(personsFixture)}
  if (m === 'post' && path === '/v1.0/persons') return {data: echo(body, 'person')}
  if (m === 'put' && path.startsWith('/v1.0/persons/')) return {data: echo(body, 'person')}
  if (m === 'delete' && path.startsWith('/v1.0/persons/')) return {data: undefined}

  // Documents
  if (key === 'get /v1.0/nfes') return {data: paginated(nfesFixture)}
  if (m === 'post' && path === '/v1.0/nfes') return {data: echo(body, 'nfe')}
  if (key === 'get /v1.0/nfces') return {data: paginated(nfcesFixture)}
  if (m === 'post' && path === '/v1.0/nfces') return {data: echo(body, 'nfce')}
  if (key === 'get /v1.0/mdfes') return {data: paginated(mdfesFixture)}
  if (m === 'post' && path === '/v1.0/mdfes') return {data: echo(body, 'mdfe')}
  if (m === 'post' && path === '/v1.0/mdfes/cargo-preview') return {data: {items: [], total_weight: '0.00', total_value: '0.00'}}

  // Detail routes (any access key)
  if (/^get \/v1\.0\/nfes\/.+/.test(key)) return {data: nfesFixture[0]}
  if (/^get \/v1\.0\/nfces\/.+/.test(key)) return {data: nfesFixture[0]}
  if (/^get \/v1\.0\/mdfes\/.+/.test(key)) return {data: mdfesFixture[0]}
  if (/^post \/v1\.0\/nfes\/.+\/(cancel|correction-letter|manifestation)$/.test(key)) return {data: nfesFixture[0]}
  if (/^post \/v1\.0\/nfces\/.+\/(cancel|substitute)$/.test(key)) return {data: nfesFixture[0]}
  if (/^post \/v1\.0\/mdfes\/.+\/(cancel|close|include-condutor|include-dfe)$/.test(key)) return {data: mdfesFixture[0]}
  if (/^get \/v1\.0\/nfes\/.+\/(events|xml|danfe)$/.test(key)) return {data: []}

  // Distributions
  if (/^get \/v1\.0\/distributions\/\w+\/history/.test(key)) return {data: paginated(distributionsFixture)}
  if (m === 'post' && /^post \/v1\.0\/distributions\/\w+\/sync$/.test(key)) return {data: {enqueued: true, job_id: 'mock-job'}}
  if (/^get \/v1\.0\/distributions\/\w+\/(nsu|key)\//.test(key)) return {data: distributionsFixture[0]}

  // Audit logs
  if (key === 'get /v1.0/audit-logs') return {data: paginated(auditLogsFixture)}

  // Fallback: never hard-crash an unmodeled endpoint.
  if (m === 'get') return {data: paginated([])}
  return {data: echo(body, 'generic')}
}

export const mockAdapter: AxiosAdapter = async (config) => {
  const req = config as InternalAxiosRequestConfig
  const method = (req.method ?? 'get').toLowerCase()
  const path = (req.url ?? '').split('?')[0]

  await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS))

  if (shouldError(path)) {
    const {status, message} = getMockState()
    const body = {detail: message}
    const error = new AxiosError(message, 'ERR_BAD_RESPONSE', req, undefined, {
      status,
      statusText: 'Mock Error',
      data: body,
      headers: {},
      config: req,
    } as never)
    throw error
  }

  const result = route(method, path, req.data)
  if ('error' in result) {
    const error = new AxiosError('Mock error', 'ERR_BAD_RESPONSE', req, undefined, {
      status: result.error.status,
      statusText: 'Mock Error',
      data: result.error.data,
      headers: {},
      config: req,
    } as never)
    throw error
  }

  const response: AxiosResponse = {
    data: result.data,
    status: result.status ?? 200,
    statusText: 'OK',
    headers: {},
    config: req,
    request: {},
  }
  return response
}
