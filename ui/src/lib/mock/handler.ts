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
  billingInvoicesFixture,
  billingPlansFixture,
  billingSubscriptionFixtures,
  certificatesFixture,
  cteConfigFixture,
  distributionsFixture,
  inutilizationsFixture,
  numberGapsFixture,
  mdfeConfigFixture,
  mdfeDetailFixture,
  mdfeEventsFixture,
  mdfesFixture,
  meFixture,
  membersFixture,
  nfceConfigFixture,
  nfceDetailFixture,
  nfceEventsFixture,
  nfcesFixture,
  nfeConfigFixture,
  nfeDetailFixture,
  nfeEventsFixture,
  nfesFixture,
  nfseConfigFixture,
  nfseDistributionsFixture,
  nfseEventsFixture,
  nfsesFixture,
  operationsFixture,
  organizationsFixture,
  paymentTermsFixture,
  personsFixture,
  productsFixture,
  rolesFixture,
  servicesFixture,
  taxProfilesFixture,
  vehiclesFixture,
  vehicleSetsFixture,
} from './fixtures'
import {getMockState, shouldError} from './state'

const MOCK_LATENCY_MS = 250

/**
 * Cadastros reutilizáveis: mesmo CRUD, mesma paginação, só muda a coleção.
 * `[segmento de rota, fixture, sufixo da chave sintética]`.
 */
const REUSABLE_REGISTRIES: [string, unknown[], string][] = [
  ['tax-profiles', taxProfilesFixture, 'tax-profile'],
  ['operations', operationsFixture, 'operation'],
  ['payment-terms', paymentTermsFixture, 'payment-term'],
  ['vehicle-sets', vehicleSetsFixture, 'vehicle-set'],
]

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
  // Detalhe da organização — traz `person`, que a listagem de /auth/me não tem.
  if (m === 'get' && /^\/v1\.0\/organizations\/[^/]+$/.test(path)) return {data: organizationsFixture[0]}

  // Configs (path-based org in URL)
  if (path.endsWith('/nfe-config')) return {data: nfeConfigFixture}
  if (path.endsWith('/nfce-config')) return {data: nfceConfigFixture}
  if (path.endsWith('/cte-config')) return {data: cteConfigFixture}
  if (path.endsWith('/mdfe-config')) return {data: mdfeConfigFixture}
  if (path.endsWith('/nfse-config')) return {data: nfseConfigFixture}

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
  // Busca por CPF/CNPJ — usada pelos atalhos "Recentes" da emissão.
  if (m === 'get' && path.startsWith('/v1.0/persons/')) return {data: personsFixture[0]}

  if (key === 'get /v1.0/services') return {data: paginated(servicesFixture)}
  if (m === 'post' && path === '/v1.0/services') return {data: echo(body, 'service')}
  if (m === 'put' && path.startsWith('/v1.0/services/')) return {data: echo(body, 'service')}
  if (m === 'delete' && path.startsWith('/v1.0/services/')) return {data: undefined}

  // Cadastros reutilizáveis — mesma forma para os quatro, só muda a fixture.
  for (const [segment, fixture, suffix] of REUSABLE_REGISTRIES) {
    if (key === `get /v1.0/${segment}`) return {data: paginated(fixture)}
    if (m === 'post' && path === `/v1.0/${segment}`) return {data: echo(body, suffix)}
    if (m === 'put' && path.startsWith(`/v1.0/${segment}/`)) return {data: echo(body, suffix)}
    if (m === 'delete' && path.startsWith(`/v1.0/${segment}/`)) return {data: undefined}
    if (m === 'get' && path.startsWith(`/v1.0/${segment}/`)) return {data: fixture[0]}
  }

  // Documents
  if (key === 'get /v1.0/nfes') return {data: paginated(nfesFixture)}
  if (m === 'post' && path === '/v1.0/nfes') return {data: echo(body, 'nfe')}
  if (key === 'get /v1.0/nfces') return {data: paginated(nfcesFixture)}
  if (m === 'post' && path === '/v1.0/nfces') return {data: echo(body, 'nfce')}
  if (key === 'get /v1.0/mdfes') return {data: paginated(mdfesFixture)}
  if (m === 'post' && path === '/v1.0/mdfes') return {data: echo(body, 'mdfe')}
  if (m === 'post' && path === '/v1.0/mdfes/cargo-preview') {
    return {data: {items: mdfeDetailFixture.documents, total_weight: '2450.000', total_value: '3589.80'}}
  }
  if (key === 'get /v1.0/nfses') return {data: paginated(nfsesFixture)}
  if (m === 'post' && path === '/v1.0/nfses') return {data: echo(body, 'nfse')}

  // Inutilização de numeração — antes das rotas de detalhe, que também casam
  // com `/nfes/{key}`.
  if (/^get \/v1\.0\/nfc?es\/inutilizations\/gaps$/.test(key)) return {data: {items: numberGapsFixture}}
  if (/^get \/v1\.0\/nfc?es\/inutilizations$/.test(key)) return {data: paginated(inutilizationsFixture)}
  if (/^post \/v1\.0\/nfc?es\/inutilizations$/.test(key)) return {data: inutilizationsFixture[1]}
  if (/^get \/v1\.0\/nfc?es\/inutilizations\/[^/]+\/xml$/.test(key)) return {data: '<ProcInutNFe/>'}

  // Event timelines. Antes das rotas de detalhe: `/nfes/{key}/events` também
  // casa com o padrão de detalhe.
  if (/^get \/v1\.0\/nfes\/[^/]+\/events$/.test(key)) return {data: paginated(nfeEventsFixture)}
  if (/^get \/v1\.0\/nfces\/[^/]+\/events$/.test(key)) return {data: paginated(nfceEventsFixture)}
  if (/^get \/v1\.0\/mdfes\/[^/]+\/events$/.test(key)) return {data: paginated(mdfeEventsFixture)}
  if (/^get \/v1\.0\/nfses\/[^/]+\/events$/.test(key)) return {data: paginated(nfseEventsFixture)}

  // Detail routes (any access key)
  if (/^get \/v1\.0\/nfes\/[^/]+$/.test(key)) return {data: nfeDetailFixture}
  if (/^get \/v1\.0\/nfces\/[^/]+$/.test(key)) return {data: nfceDetailFixture}
  if (/^get \/v1\.0\/mdfes\/[^/]+$/.test(key)) return {data: mdfeDetailFixture}
  if (/^get \/v1\.0\/nfses\/[^/]+$/.test(key)) return {data: nfsesFixture[0]}
  if (/^post \/v1\.0\/nfes\/.+\/(cancel|correction-letter|manifestation)$/.test(key)) return {data: nfeDetailFixture}
  if (/^post \/v1\.0\/nfces\/.+\/(cancel|substitute)$/.test(key)) return {data: nfceDetailFixture}
  if (/^post \/v1\.0\/mdfes\/.+\/(cancel|close|include-condutor|include-dfe)$/.test(key)) return {data: mdfeDetailFixture}
  if (/^post \/v1\.0\/nfses\/.+\/(cancel|substitute|events)$/.test(key)) return {data: nfsesFixture[0]}

  // Distributions
  if (key === 'get /v1.0/nfse/distributions') return {data: paginated(nfseDistributionsFixture)}
  if (/^get \/v1\.0\/distributions\/\w+\/history$/.test(key)) return {data: paginated(distributionsFixture)}
  if (m === 'post' && /^post \/v1\.0\/distributions\/\w+\/sync$/.test(key)) return {data: {enqueued: true, job_id: 'mock-job'}}
  if (/^get \/v1\.0\/distributions\/\w+\/(nsu|key)\//.test(key)) return {data: distributionsFixture[0]}

  // Billing — the scenario decides what the account's standing is.
  if (key === 'get /v1.0/billing/plans') return {data: billingPlansFixture}
  if (key === 'get /v1.0/billing/subscription') {
    return {data: billingSubscriptionFixtures[getMockState().billing]}
  }
  if (key === 'get /v1.0/billing/invoices') return {data: {data: billingInvoicesFixture}}
  if (m === 'post' && path === '/v1.0/billing/subscription') {
    // Choosing a paid plan lands on INCOMPLETE with an invoice, which is what
    // production does — the checkout is a redirect away, not a state jump.
    return {data: {...billingSubscriptionFixtures.checkout_pending, invoice: billingInvoicesFixture[1]}, status: 201}
  }
  if (m === 'post' && path === '/v1.0/billing/subscription/change') {
    return {data: {...billingSubscriptionFixtures.pro_active, invoice: billingInvoicesFixture[1]}}
  }
  if (m === 'post' && path === '/v1.0/billing/subscription/cancel') {
    return {data: {...billingSubscriptionFixtures.pro_active, cancel_at_period_end: true}}
  }
  if (/^get \/v1\.0\/organizations\/[^/]+\/plan$/.test(key)) {
    return {data: {...billingSubscriptionFixtures[getMockState().billing], manageable: false}}
  }

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
