import axios, {AxiosError, type AxiosAdapter, type AxiosInstance, type AxiosRequestConfig, type AxiosResponse} from 'axios'
import type {
  OperationCreate,
  CargoUnitCreate,
  ImportDeclarationCreate,
  InsurancePolicyCreate,
  InsurancePolicyItemOut,
  ProductLotCreate,
  ProductLotItemOut,
  ReferenceDocumentCreate,
  ReferenceDocumentItemOut,
  ServiceLocationCreate,
  ServiceLocationItemOut,
  FuelPumpCreate,
  FuelPumpItemOut,
  ImportDeclarationItemOut,
  CargoUnitItemOut,
  TollProviderCreate,
  TollProviderItemOut,
  PaymentTerminalCreate,
  PaymentTerminalItemOut,
  PaymentTermCreate,
  PaymentTermItemOut,
  VehicleSetCreate,
  VehicleSetItemOut,
  OperationItemOut,
  TaxProfileCreate,
  TaxProfileItemOut,
  AuditLogOut,
  AuxiliaryDocumentDownload,
  SignedFileDownload,
  CertificateOut,
  CTeConfigOut,
  DistributionLookupOut,
  InvitationOut,
  InvitationPreview,
  LookupOrganizationOut,
  OpenCnpjOffice,
  MemberOut,
  MdfeCargoPreview,
  MDFeConfigOut,
  MdfeDetailOut,
  MdfeDocRef,
  MdfeEmit,
  MdfeIncludeDFeDoc,
  MdfeListOut,
  MeResponse,
  MunicipalParamsOut,
  NFCeConfigOut,
  NfceEmit,
  NFeConfigOut,
  NfeDetailOut,
  NFeDistributionOut,
  NfeEmit,
  NfeEventOut,
  NfeListOut,
  NfseConfigOut,
  NfseDetailOut,
  NfseDistributionOut,
  NfseEmit,
  NfseEventBody,
  NfseEventOut,
  NfseListOut,
  OrganizationOut,
  PaginatedResponse,
  PersonCreate,
  PersonItemOut,
  PersonUpdate,
  ProductCreate,
  ProductOut,
  ProductUpdate,
  RoleOut,
  ServiceCreate,
  ServiceOut,
  ServiceUpdate,
  SyncEnqueuedOut,
  VehicleCreate,
  VehicleOut,
  VehicleRequirements,
  VehicleUpdate,
  InutilizationIn,
  InutilizationOut,
  NumberGapOut,
} from '@/lib/types/api'
import {unformatCpfCnpj} from "@/lib/utils/document";
import {STORAGE_KEY_ORG} from '@/lib/constants/storage'
import {isStrippableBody, stripNulls} from '@/lib/utils/strip-nulls'
import {MOCK_ENABLED} from '@/lib/mock/env'
import {
  HTTP_TIMEOUT_MS,
  checkApiLiveness,
  requireApiLiveness,
  type ApiUnavailableError,
} from '@/lib/network/liveness'
import {redirectOnMaintenance} from '@/lib/network/maintenance'
import type {PersonRole} from '@/lib/schemas/entity'
import type {
  AccountSubscription,
  AccountSubscriptionWithInvoice,
  BillingInvoice,
  BillingPlansResponse,
  PlanChoice,
} from '@/lib/types/billing'

// Empty means same-origin: CloudFront forwards /v1.0/* to the ALB in deployed
// environments, and `next dev` proxies it locally (next.config.ts). Either way
// the browser never makes a cross-origin request, so CORS never applies.
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? ''
export const ORG_HEADER = 'Dfe-Organization-Pk'

/**
 * The value of `Dfe-Organization-Pk`.
 *
 * Exported so the rule is testable: it was a line inside an interceptor, and it
 * was wrong. `unformatCpfCnpj` strips a company id's hyphens and uppercases its
 * hex, and the API's `IsCompanyKey` refuses that — so every request in the
 * product would have failed with the cause three layers from the symptom.
 *
 * The header's NAME does not change. Renaming it is a coordinated two-app
 * deploy for a word (see the constant's counterpart in middleware/rbac.go).
 */
export const orgHeaderValue = (org: {pk: string}): string => unformatCpfCnpj(org.pk)
const OPEN_CNPJ_API_URL = 'https://open.cnpja.com'
const OPEN_CNPJ_DOCUMENT_LENGTH = 14
const OPEN_CNPJ_CACHE_TTL_MS = 30 * 60 * 1_000
const AUXILIARY_DOCUMENT_TIMEOUT_MS = 12_000

// Retries live here rather than in TanStack: one bounded, jittered budget for
// the whole app. Retrying in both layers turns three transport attempts into
// nine requests against a server that is already struggling.
const MAX_HTTP_RETRIES = 2
const RETRYABLE_HTTP_STATUSES = new Set([408, 425, 429, 500, 502, 503, 504])
// Only requests that can be repeated without issuing something twice. A fiscal
// document is never worth an accidental duplicate.
const SAFE_HTTP_METHODS = new Set(['get', 'head', 'options'])
const RETRY_BASE_DELAY_MS = 250
const MAX_RETRY_DELAY_MS = 3_000

interface RetryConfig extends AxiosRequestConfig {
  _retry?: boolean
  _networkRetryCount?: number
}

/** Full jitter, and `Retry-After` wins when the server names a delay. */
export function httpRetryDelay(attempt: number, retryAfter?: string, random: () => number = Math.random): number {
  const retryAfterMs = retryAfter == null ? Number.NaN : Number(retryAfter) * 1_000
  if (Number.isFinite(retryAfterMs)) return Math.max(0, retryAfterMs) + Math.floor(random() * 250)
  const ceiling = Math.min(MAX_RETRY_DELAY_MS, RETRY_BASE_DELAY_MS * 2 ** Math.max(0, attempt - 1))
  return Math.floor(random() * ceiling)
}

function isRetryable(error: AxiosError): boolean {
  const config = error.config as RetryConfig | undefined
  if (!config || (config._networkRetryCount ?? 0) >= MAX_HTTP_RETRIES) return false
  if (!SAFE_HTTP_METHODS.has((config.method ?? 'get').toLowerCase())) return false
  const status = error.response?.status
  // No status means timeout or transport failure — ambiguous, so retryable.
  return status === undefined || RETRYABLE_HTTP_STATUSES.has(status)
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// Access token held in memory only — never written to localStorage.
let _accessToken: string | null = null

// AuthContext registers this to supply a fresh access token on 401.
let _refreshFn: (() => Promise<string | null>) | null = null

export function registerRefreshFn(fn: () => Promise<string | null>): void {
  _refreshFn = fn
}

export function getAccessToken(): string | null {
  return _accessToken
}

const tokenListeners = new Set<(token: string) => void>()

// Notified on every genuinely new access token (login, silent refresh) — lets
// a WebSocket consumer force an immediate reconnect instead of holding a
// stale token indefinitely (see @aoctech/ws-client's subscribeToken).
export function subscribeAccessToken(cb: (token: string) => void): () => void {
  tokenListeners.add(cb)
  return () => tokenListeners.delete(cb)
}

function notifyTokenListeners(token: string): void {
  tokenListeners.forEach((cb) => cb(token))
}

interface ErrorResponseBody {
  detail?: string
  title?: string

  [key: string]: unknown
}

async function parseResponseErrToJson(response: AxiosResponse): Promise<ErrorResponseBody> {
  if (response.data instanceof Blob) {
    const text = await response.data.text();
    return JSON.parse(text);
  }
  return response.data;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly detail: string,
    public readonly raw?: unknown,
  ) {
    super(detail)
    this.name = 'ApiError'
  }
}

function createAxiosInstance(): AxiosInstance {
  const instance = axios.create({
    baseURL: API_BASE_URL,
    // The API answers fast or not at all; a request still open after this is an
    // outage, and waiting longer only makes the screen feel broken.
    timeout: HTTP_TIMEOUT_MS,
    headers: {'Content-Type': 'application/json'},
  })

  instance.interceptors.request.use(async (config) => {
    // The public health probe owns recovery while the API is down. Without this
    // every mounted query becomes its own availability probe against a server
    // that cannot answer.
    if (!MOCK_ENABLED) await requireApiLiveness()

    if (_accessToken) config.headers.Authorization = `Bearer ${_accessToken}`

    if (typeof window !== 'undefined') {
      const orgRaw = localStorage.getItem(STORAGE_KEY_ORG)
      if (orgRaw) {
        try {
          const org = JSON.parse(orgRaw) as { pk: string }
          if (org?.pk) config.headers[ORG_HEADER] = orgHeaderValue(org)
        } catch {
          // ignore malformed storage
        }
      }
    }

    // Only strip plain JSON bodies — never FormData/Blob/etc. (would be flattened to {}).
    if (isStrippableBody(config.data)) {
      const method = (config.method ?? 'get').toLowerCase()
      const dropNull = method === 'post' // create: no clear semantics; updates keep null
      config.data = stripNulls(config.data, dropNull)
    }
    return config
  })

  instance.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
      const original = error.config as RetryConfig | undefined
      if (error.response?.status === 401 && original && !original._retry && _refreshFn) {
        original._retry = true
        const newToken = await _refreshFn()
        if (newToken) {
          _accessToken = newToken
          notifyTokenListeners(newToken)
          original.headers = {
            ...original.headers,
            Authorization: `Bearer ${newToken}`,
          }
          return instance(original)
        }
        // Refresh failed — start OAuth flow (imported lazily to avoid SSR issues).
        // Never use /callback as returnTo; that would loop back into the callback handler.
        if (typeof window !== 'undefined') {
          const {startOAuthFlow, currentReturnTo} = await import('@/lib/auth/oauth')
          await startOAuthFlow(currentReturnTo())
        }
        return
      }
      // A timeout or a transport failure is ambiguous in a browser: a dead load
      // balancer answers without CORS headers and surfaces as a TypeError, not
      // as a status. Confirm against the health probe before spending retries.
      let livenessAllowsRetry = error.name !== 'ApiUnavailableError'
      if (!MOCK_ENABLED && !error.response && livenessAllowsRetry) livenessAllowsRetry = await checkApiLiveness()
      if (livenessAllowsRetry && isRetryable(error) && original) {
        const attempt = (original._networkRetryCount ?? 0) + 1
        original._networkRetryCount = attempt
        const retryAfter = error.response?.headers?.['retry-after'] as string | undefined
        await wait(httpRetryDelay(attempt, retryAfter))
        return instance(original)
      }

      // A 503 that survived its retries is the server saying it is in
      // maintenance — an answer, not a hiccup — so the person gets a screen
      // instead of a failed query behind a banner that polls forever.
      redirectOnMaintenance(error.response?.status)

      if (error.name === 'ApiUnavailableError') {
        const unavailable = error as unknown as ApiUnavailableError
        throw new ApiError(0, unavailable.message, {reason: unavailable.reason})
      }

      const data = error.response ? await parseResponseErrToJson(error.response) : undefined
      const detail = data?.detail ?? data?.title ?? error.message ?? `HTTP ${error.response?.status}`
      throw new ApiError(error.response?.status ?? 0, detail, data)
    },
  )

  return instance
}

/** Isolado do cliente autenticado para nunca enviar token ou organização a um
 * serviço público de terceiros. Também não aplica retry automático: o CNPJá
 * limita cada IP e repetir 429 só consumiria ainda mais a janela. */
function createOpenCnpjInstance(): AxiosInstance {
  return axios.create({
    baseURL: OPEN_CNPJ_API_URL,
    timeout: HTTP_TIMEOUT_MS,
    headers: {Accept: 'application/json'},
  })
}

class ApiClient {
  private readonly http: AxiosInstance
  private readonly openCnpjHttp: AxiosInstance
  private readonly openCnpjCache = new Map<string, {expiresAt: number; value: OpenCnpjOffice}>()
  private readonly openCnpjPending = new Map<string, Promise<OpenCnpjOffice>>()

  constructor() {
    this.http = createAxiosInstance()
    this.openCnpjHttp = createOpenCnpjInstance()
  }

  setToken(token: string | null): void {
    _accessToken = token
    if (token) notifyTokenListeners(token)
  }

  /** Dev-only seam: lets the mock layer replace axios's adapter with an
   *  in-memory fixture handler. Never call in production paths. */
  setAdapter(adapter: AxiosAdapter): void {
    this.http.defaults.adapter = adapter
  }

  /** Test seam for the isolated public client. */
  setOpenCnpjAdapter(adapter: AxiosAdapter): void {
    this.openCnpjHttp.defaults.adapter = adapter
    this.openCnpjCache.clear()
    this.openCnpjPending.clear()
  }

  // Auth
  async me(): Promise<MeResponse> {
    return this.get<MeResponse>('/v1.0/auth/me')
  }

  async getRoles(): Promise<RoleOut[]> {
    return this.get<RoleOut[]>('/v1.0/auth/roles')
  }

  async acceptTermsAddendum(): Promise<void> {
    await this.post('/v1.0/auth/terms-addendum/accept')
  }

  // Organizations (path-based, no header needed)
  async getOrganizations(): Promise<OrganizationOut[]> {
    return this.get<OrganizationOut[]>('/v1.0/organizations')
  }

  async getOrganization(pk: string): Promise<OrganizationOut> {
    return this.get<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(pk)}`)
  }

  // linkCompany adopts a company created in the CTech account — the return leg
  // of the handoff.
  //
  // Idempotent by design on the server, which matters here: this is called from
  // a landing page, and a refresh replays the same ids. The caller does not have
  // to guard against it.
  async linkCompany(organizationId: string, companyId: string): Promise<OrganizationOut> {
    return this.post<OrganizationOut>('/v1.0/organizations/link', {
      organization_id: organizationId,
      company_id: companyId,
    })
  }

  // No createOrganization: a company is registered in ctech-account and adopted
  // here by linkCompany (ctech-billing ADR 0022). The route still exists on the
  // API for the migration's sake, and calling it from the browser is what the
  // handoff was built to stop — it produces a company the platform never heard
  // of, with no company id and no reach edge.

  // certificateRequirement reports whether this company needs an A1 upload of
  // its own — false when a matriz certificate for the same CNPJ root can be
  // inherited, which is how the onboarding certificate step closes for a filial
  // that must not upload one.
  async certificateRequirement(cpfOrCnpj: string): Promise<{ required: boolean }> {
    return this.get(`/v1.0/organizations/certificate-requirement?cpf_or_cnpj=${unformatCpfCnpj(cpfOrCnpj)}`)
  }

  async updateOrganization(pk: string, data: unknown): Promise<OrganizationOut> {
    return this.put<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(pk)}`, data)
  }

  async deleteOrganization(pk: string): Promise<void> {
    return this.del<void>(`/v1.0/organizations/${unformatCpfCnpj(pk)}`)
  }

  // The organization path segments below go through unformatCpfCnpj because a
  // legacy key travels as a bare document; a company id passes through it
  // untouched, so both eras address the right organization. The second argument
  // on removeAuthorizedViewer is a person's document and is unrelated.
  async addAuthorizedViewer(orgPk: string, data: { cpf_or_cnpj: string; name: string }): Promise<OrganizationOut> {
    return this.post<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(orgPk)}/authorized-viewers`, data)
  }

  async removeAuthorizedViewer(orgPk: string, cpfCnpj: string): Promise<OrganizationOut> {
    return this.del<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(orgPk)}/authorized-viewers/${unformatCpfCnpj(cpfCnpj)}`)
  }

  // Products — org context auto-injected via Dfe-Organization-Pk header
  async getProducts(params?: { limit?: number; cursor?: string }): Promise<PaginatedResponse<ProductOut>> {
    return this.get('/v1.0/products', {params})
  }

  async getProduct(id: string): Promise<ProductOut> {
    return this.get(`/v1.0/products/${id}`)
  }

  async createProduct(data: ProductCreate): Promise<ProductOut> {
    return this.post('/v1.0/products', data)
  }

  async updateProduct(id: string, data: ProductUpdate): Promise<ProductOut> {
    return this.put(`/v1.0/products/${id}`, data)
  }

  async deleteProduct(id: string): Promise<void> {
    return this.del(`/v1.0/products/${id}`)
  }

  // Services (catálogo NFS-e) — org context auto-injected via Dfe-Organization-Pk header
  async getServices(params?: { limit?: number; cursor?: string }): Promise<PaginatedResponse<ServiceOut>> {
    return this.get('/v1.0/services', {params})
  }

  async getService(id: string): Promise<ServiceOut> {
    return this.get(`/v1.0/services/${id}`)
  }

  async createService(data: ServiceCreate): Promise<ServiceOut> {
    return this.post('/v1.0/services', data)
  }

  async updateService(id: string, data: ServiceUpdate): Promise<ServiceOut> {
    return this.put(`/v1.0/services/${id}`, data)
  }

  async deleteService(id: string): Promise<void> {
    return this.del(`/v1.0/services/${id}`)
  }

  // Vehicles
  async getVehicles(params?: {
    role?: 'tractor' | 'trailer';
    limit?: number;
    cursor?: string
  }): Promise<PaginatedResponse<VehicleOut>> {
    return this.get('/v1.0/vehicles', {params})
  }

  async getVehicle(id: string): Promise<VehicleOut> {
    return this.get(`/v1.0/vehicles/${id}`)
  }

  async getVehicleRequirements(id: string, docType: string, role: string): Promise<VehicleRequirements> {
    return this.get(`/v1.0/vehicles/${id}/requirements`, {params: {doc_type: docType, role}})
  }

  async createVehicle(data: VehicleCreate): Promise<VehicleOut> {
    return this.post('/v1.0/vehicles', data)
  }

  async updateVehicle(id: string, data: VehicleUpdate): Promise<VehicleOut> {
    return this.put(`/v1.0/vehicles/${id}`, data)
  }

  async deleteVehicle(id: string): Promise<void> {
    return this.del(`/v1.0/vehicles/${id}`)
  }

  // Tax profiles (perfis fiscais reutilizáveis)
  async getTaxProfiles(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<TaxProfileItemOut>> {
    return this.get('/v1.0/tax-profiles', {params})
  }

  async getTaxProfile(id: string): Promise<TaxProfileItemOut> {
    return this.get(`/v1.0/tax-profiles/${id}`)
  }

  async createTaxProfile(data: TaxProfileCreate): Promise<TaxProfileItemOut> {
    return this.post('/v1.0/tax-profiles', data)
  }

  async updateTaxProfile(id: string, data: TaxProfileCreate): Promise<TaxProfileItemOut> {
    return this.put(`/v1.0/tax-profiles/${id}`, data)
  }

  async deleteTaxProfile(id: string): Promise<void> {
    return this.del(`/v1.0/tax-profiles/${id}`)
  }

  // Alíquota ICMS/FCP que o backend resolveria (sem override) — usada pelo
  // warning de alíquota customizada no TaxFieldsEditor.
  async getIcmsAliqPreview(params: {emitUf: string; destUf: string; ncm?: string}): Promise<{icms_aliq: string; fcp_aliq: string}> {
    return this.get('/v1.0/tax-tables/icms-aliq', {
      params: {emit_uf: params.emitUf, dest_uf: params.destUf, ncm: params.ncm},
    })
  }

  // Operations (naturezas de operação)
  async getOperations(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<OperationItemOut>> {
    return this.get('/v1.0/operations', {params})
  }

  async getOperation(id: string): Promise<OperationItemOut> {
    return this.get(`/v1.0/operations/${id}`)
  }

  async createOperation(data: OperationCreate): Promise<OperationItemOut> {
    return this.post('/v1.0/operations', data)
  }

  async updateOperation(id: string, data: OperationCreate): Promise<OperationItemOut> {
    return this.put(`/v1.0/operations/${id}`, data)
  }

  async deleteOperation(id: string): Promise<void> {
    return this.del(`/v1.0/operations/${id}`)
  }

  // Payment terms (condições de pagamento)
  async getPaymentTerms(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<PaymentTermItemOut>> {
    return this.get('/v1.0/payment-terms', {params})
  }

  async getPaymentTerm(id: string): Promise<PaymentTermItemOut> {
    return this.get(`/v1.0/payment-terms/${id}`)
  }

  async createPaymentTerm(data: PaymentTermCreate): Promise<PaymentTermItemOut> {
    return this.post('/v1.0/payment-terms', data)
  }

  async updatePaymentTerm(id: string, data: PaymentTermCreate): Promise<PaymentTermItemOut> {
    return this.put(`/v1.0/payment-terms/${id}`, data)
  }

  async deletePaymentTerm(id: string): Promise<void> {
    return this.del(`/v1.0/payment-terms/${id}`)
  }

  // Payment terminals (terminais de captura / POS)

  async getPaymentTerminals(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<PaymentTerminalItemOut>> {
    return this.get('/v1.0/payment-terminals', {params})
  }

  async getPaymentTerminal(id: string): Promise<PaymentTerminalItemOut> {
    return this.get(`/v1.0/payment-terminals/${id}`)
  }

  async createPaymentTerminal(data: PaymentTerminalCreate): Promise<PaymentTerminalItemOut> {
    return this.post('/v1.0/payment-terminals', data)
  }

  async updatePaymentTerminal(id: string, data: PaymentTerminalCreate): Promise<PaymentTerminalItemOut> {
    return this.put(`/v1.0/payment-terminals/${id}`, data)
  }

  async deletePaymentTerminal(id: string): Promise<void> {
    return this.del(`/v1.0/payment-terminals/${id}`)
  }

  // Toll providers (fornecedoras de vale-pedágio)

  async getTollProviders(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<TollProviderItemOut>> {
    return this.get('/v1.0/toll-providers', {params})
  }

  async getTollProvider(id: string): Promise<TollProviderItemOut> {
    return this.get(`/v1.0/toll-providers/${id}`)
  }

  async createTollProvider(data: TollProviderCreate): Promise<TollProviderItemOut> {
    return this.post('/v1.0/toll-providers', data)
  }

  async updateTollProvider(id: string, data: TollProviderCreate): Promise<TollProviderItemOut> {
    return this.put(`/v1.0/toll-providers/${id}`, data)
  }

  async deleteTollProvider(id: string): Promise<void> {
    return this.del(`/v1.0/toll-providers/${id}`)
  }

  // Cargo units (unidades de transporte e de carga do MDF-e)

  async getCargoUnits(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<CargoUnitItemOut>> {
    return this.get('/v1.0/cargo-units', {params})
  }

  async getCargoUnit(id: string): Promise<CargoUnitItemOut> {
    return this.get(`/v1.0/cargo-units/${id}`)
  }

  async createCargoUnit(data: CargoUnitCreate): Promise<CargoUnitItemOut> {
    return this.post('/v1.0/cargo-units', data)
  }

  async updateCargoUnit(id: string, data: CargoUnitCreate): Promise<CargoUnitItemOut> {
    return this.put(`/v1.0/cargo-units/${id}`, data)
  }

  async deleteCargoUnit(id: string): Promise<void> {
    return this.del(`/v1.0/cargo-units/${id}`)
  }

  // Import declarations (declarações de importação — NF-e prod/DI)

  async getImportDeclarations(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<ImportDeclarationItemOut>> {
    return this.get('/v1.0/import-declarations', {params})
  }

  async getImportDeclaration(id: string): Promise<ImportDeclarationItemOut> {
    return this.get(`/v1.0/import-declarations/${id}`)
  }

  async createImportDeclaration(data: ImportDeclarationCreate): Promise<ImportDeclarationItemOut> {
    return this.post('/v1.0/import-declarations', data)
  }

  async updateImportDeclaration(id: string, data: ImportDeclarationCreate): Promise<ImportDeclarationItemOut> {
    return this.put(`/v1.0/import-declarations/${id}`, data)
  }

  async deleteImportDeclaration(id: string): Promise<void> {
    return this.del(`/v1.0/import-declarations/${id}`)
  }

  // Insurance policies (apólices de seguro da carga — MDF-e infMDFe/seg)

  async getInsurancePolicies(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<InsurancePolicyItemOut>> {
    return this.get('/v1.0/insurance-policies', {params})
  }

  async getInsurancePolicy(id: string): Promise<InsurancePolicyItemOut> {
    return this.get(`/v1.0/insurance-policies/${id}`)
  }

  async createInsurancePolicy(data: InsurancePolicyCreate): Promise<InsurancePolicyItemOut> {
    return this.post('/v1.0/insurance-policies', data)
  }

  async updateInsurancePolicy(id: string, data: InsurancePolicyCreate): Promise<InsurancePolicyItemOut> {
    return this.put(`/v1.0/insurance-policies/${id}`, data)
  }

  async deleteInsurancePolicy(id: string): Promise<void> {
    return this.del(`/v1.0/insurance-policies/${id}`)
  }

  // Product lots (lotes de produção — NF-e prod/rastro)

  async getProductLots(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<ProductLotItemOut>> {
    return this.get('/v1.0/product-lots', {params})
  }

  async getProductLot(id: string): Promise<ProductLotItemOut> {
    return this.get(`/v1.0/product-lots/${id}`)
  }

  async createProductLot(data: ProductLotCreate): Promise<ProductLotItemOut> {
    return this.post('/v1.0/product-lots', data)
  }

  async updateProductLot(id: string, data: ProductLotCreate): Promise<ProductLotItemOut> {
    return this.put(`/v1.0/product-lots/${id}`, data)
  }

  async deleteProductLot(id: string): Promise<void> {
    return this.del(`/v1.0/product-lots/${id}`)
  }

  // Service locations (obra, imóvel e local de evento — NFS-e)

  async getServiceLocations(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<ServiceLocationItemOut>> {
    return this.get('/v1.0/service-locations', {params})
  }

  async getServiceLocation(id: string): Promise<ServiceLocationItemOut> {
    return this.get(`/v1.0/service-locations/${id}`)
  }

  async createServiceLocation(data: ServiceLocationCreate): Promise<ServiceLocationItemOut> {
    return this.post('/v1.0/service-locations', data)
  }

  async updateServiceLocation(id: string, data: ServiceLocationCreate): Promise<ServiceLocationItemOut> {
    return this.put(`/v1.0/service-locations/${id}`, data)
  }

  async deleteServiceLocation(id: string): Promise<void> {
    return this.del(`/v1.0/service-locations/${id}`)
  }

  // Reference documents (dedução/redução e reembolso/repasse — NFS-e)

  async getReferenceDocuments(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<ReferenceDocumentItemOut>> {
    return this.get('/v1.0/reference-documents', {params})
  }

  async getReferenceDocument(id: string): Promise<ReferenceDocumentItemOut> {
    return this.get(`/v1.0/reference-documents/${id}`)
  }

  async createReferenceDocument(data: ReferenceDocumentCreate): Promise<ReferenceDocumentItemOut> {
    return this.post('/v1.0/reference-documents', data)
  }

  async updateReferenceDocument(id: string, data: ReferenceDocumentCreate): Promise<ReferenceDocumentItemOut> {
    return this.put(`/v1.0/reference-documents/${id}`, data)
  }

  async deleteReferenceDocument(id: string): Promise<void> {
    return this.del(`/v1.0/reference-documents/${id}`)
  }

  // Fuel pumps (bicos, bombas e tanques — NF-e prod/comb/encerrante)

  async getFuelPumps(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<FuelPumpItemOut>> {
    return this.get('/v1.0/fuel-pumps', {params})
  }

  async getFuelPump(id: string): Promise<FuelPumpItemOut> {
    return this.get(`/v1.0/fuel-pumps/${id}`)
  }

  async createFuelPump(data: FuelPumpCreate): Promise<FuelPumpItemOut> {
    return this.post('/v1.0/fuel-pumps', data)
  }

  async updateFuelPump(id: string, data: FuelPumpCreate): Promise<FuelPumpItemOut> {
    return this.put(`/v1.0/fuel-pumps/${id}`, data)
  }

  async deleteFuelPump(id: string): Promise<void> {
    return this.del(`/v1.0/fuel-pumps/${id}`)
  }

  // Vehicle sets (composições veiculares)
  async getVehicleSets(params?: { limit?: number; cursor?: string; name?: string }): Promise<PaginatedResponse<VehicleSetItemOut>> {
    return this.get('/v1.0/vehicle-sets', {params})
  }

  async getVehicleSet(id: string): Promise<VehicleSetItemOut> {
    return this.get(`/v1.0/vehicle-sets/${id}`)
  }

  async createVehicleSet(data: VehicleSetCreate): Promise<VehicleSetItemOut> {
    return this.post('/v1.0/vehicle-sets', data)
  }

  async updateVehicleSet(id: string, data: VehicleSetCreate): Promise<VehicleSetItemOut> {
    return this.put(`/v1.0/vehicle-sets/${id}`, data)
  }

  async deleteVehicleSet(id: string): Promise<void> {
    return this.del(`/v1.0/vehicle-sets/${id}`)
  }

  // Persons (Clientes/Fornecedores)
  async getPersons(params?: {
    limit?: number
    cursor?: string
    /** Filtra por papel de cadastro. Uma pessoa multi-papel aparece em todas as suas listagens. */
    role?: PersonRole
    /** Termo de busca: dígitos casam o documento, qualquer outro texto casa o prefixo do nome. */
    q?: string
  }): Promise<PaginatedResponse<PersonItemOut>> {
    return this.get('/v1.0/persons', {params})
  }

  async getPerson(cpfCnpj: string): Promise<PersonItemOut> {
    return this.get(`/v1.0/persons/${cpfCnpj}`)
  }

  async createPerson(data: PersonCreate): Promise<PersonItemOut> {
    return this.post('/v1.0/persons', data)
  }

  async updatePerson(cpfCnpj: string, data: PersonUpdate): Promise<PersonItemOut> {
    return this.put(`/v1.0/persons/${cpfCnpj}`, data)
  }

  async deletePerson(cpfCnpj: string): Promise<void> {
    return this.del(`/v1.0/persons/${cpfCnpj}`)
  }

  // Fiscal configs (path-based, org in URL not header)
  async getNFeConfig(pk: string): Promise<NFeConfigOut> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/nfe-config`)
  }

  async upsertNFeConfig(pk: string, data: object): Promise<NFeConfigOut> {
    return this.put(`/v1.0/organizations/${unformatCpfCnpj(pk)}/nfe-config`, data)
  }

  async getNFCeConfig(pk: string): Promise<NFCeConfigOut> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/nfce-config`)
  }

  async upsertNFCeConfig(pk: string, data: object): Promise<NFCeConfigOut> {
    return this.put(`/v1.0/organizations/${unformatCpfCnpj(pk)}/nfce-config`, data)
  }

  async getCTeConfig(pk: string): Promise<CTeConfigOut> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/cte-config`)
  }

  async upsertCTeConfig(pk: string, data: object): Promise<CTeConfigOut> {
    return this.put(`/v1.0/organizations/${unformatCpfCnpj(pk)}/cte-config`, data)
  }

  async getMDFeConfig(pk: string): Promise<MDFeConfigOut> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/mdfe-config`)
  }

  async upsertMDFeConfig(pk: string, data: object): Promise<MDFeConfigOut> {
    return this.put(`/v1.0/organizations/${unformatCpfCnpj(pk)}/mdfe-config`, data)
  }

  async getNfseConfig(pk: string): Promise<NfseConfigOut> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/nfse-config`)
  }

  async upsertNfseConfig(pk: string, data: object): Promise<NfseConfigOut> {
    return this.put(`/v1.0/organizations/${unformatCpfCnpj(pk)}/nfse-config`, data)
  }

  // Certificates
  async getCertificates(pk: string): Promise<CertificateOut[]> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/certificates`)
  }

  async uploadCertificate(pk: string, file: File, password: string): Promise<CertificateOut> {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('password', password)
    return (await this.http.post<CertificateOut>(
      `/v1.0/organizations/${unformatCpfCnpj(pk)}/certificates`,
      formData,
      {headers: {'Content-Type': undefined}},
    )).data
  }

  async deleteCertificate(pk: string, md5: string): Promise<void> {
    return this.del(`/v1.0/organizations/${unformatCpfCnpj(pk)}/certificates/${md5}`)
  }

  // Members & invitations (org-scoped)
  async listMembers(pk: string): Promise<MemberOut[]> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/members`)
  }

  async removeMember(pk: string, userId: string): Promise<void> {
    return this.del(`/v1.0/organizations/${unformatCpfCnpj(pk)}/members/${userId}`)
  }

  async updateMemberRole(pk: string, userId: string, role: string): Promise<void> {
    await this.put(`/v1.0/organizations/${unformatCpfCnpj(pk)}/members/${userId}/role`, {role})
  }

  async listInvitations(pk: string): Promise<InvitationOut[]> {
    return this.get(`/v1.0/organizations/${unformatCpfCnpj(pk)}/invitations`)
  }

  async createInvitation(pk: string, role: string): Promise<InvitationOut> {
    return this.post(`/v1.0/organizations/${unformatCpfCnpj(pk)}/invitations`, {role})
  }

  async revokeInvitation(pk: string, id: string): Promise<void> {
    return this.del(`/v1.0/organizations/${unformatCpfCnpj(pk)}/invitations/${id}`)
  }

  // Invitations by token (invitee side)
  async getInvitation(token: string): Promise<InvitationPreview> {
    return this.get(`/v1.0/invitations/${encodeURIComponent(token)}`)
  }

  async acceptInvitation(token: string): Promise<{ org_pk: string; role: string }> {
    return this.post(`/v1.0/invitations/${encodeURIComponent(token)}/accept`)
  }

  async declineInvitation(token: string): Promise<void> {
    await this.post(`/v1.0/invitations/${encodeURIComponent(token)}/decline`)
  }

  // NF-es — uses Dfe-Organization-Pk header
  async getNfes(params?: {
    limit?: number
    cursor?: string
    year?: number
    month?: number
    day?: number
    number?: number
    incoming?: 0 | 1 | 2
    sort?: 'asc' | 'desc'
  }): Promise<PaginatedResponse<NfeListOut>> {
    return this.get('/v1.0/nfes', {params})
  }

  async getNfe(accessKey: string): Promise<NfeDetailOut> {
    return this.get(`/v1.0/nfes/${accessKey}`)
  }

  async emitNfe(data: NfeEmit): Promise<NfeDetailOut> {
    return this.post('/v1.0/nfes', data)
  }

  async cancelNfe(accessKey: string, justification: string, sequenceNumber = 1): Promise<NfeDetailOut> {
    return this.post(`/v1.0/nfes/${accessKey}/cancel`, {justification, sequence_number: sequenceNumber})
  }

  async sendCorrectionLetter(accessKey: string, correctionText: string, sequenceNumber = 1): Promise<NfeDetailOut> {
    return this.post(`/v1.0/nfes/${accessKey}/correction-letter`, {
      correction_text: correctionText,
      sequence_number: sequenceNumber
    })
  }

  async sendManifestation(accessKey: string, eventType: string, sequenceNumber = 1, justification?: string): Promise<NfeDetailOut> {
    return this.post(`/v1.0/nfes/${accessKey}/manifestation`, {
      event_type: eventType,
      sequence_number: sequenceNumber,
      justification
    })
  }

  async getNfeEvents(accessKey: string): Promise<PaginatedResponse<NfeEventOut>> {
    return this.get(`/v1.0/nfes/${accessKey}/events`, {params: {limit: 50}})
  }

  async downloadNfeEventXml(accessKey: string, eventSk: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfes/${accessKey}/events/${encodeURIComponent(eventSk)}/xml`)
  }

  async downloadNfeXml(accessKey: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfes/${accessKey}/xml`)
  }

  async downloadNfeDanfe(accessKey: string): Promise<AuxiliaryDocumentDownload> {
    return this.get(`/v1.0/nfes/${accessKey}/danfe`, {timeout: AUXILIARY_DOCUMENT_TIMEOUT_MS})
  }

  // NFC-es (modelo 65) — same record shape as NF-e, distinct routes
  async listNfces(params?: {
    limit?: number
    cursor?: string
    year?: number
    month?: number
    day?: number
    number?: number
    sort?: 'asc' | 'desc'
  }): Promise<PaginatedResponse<NfeListOut>> {
    return this.get('/v1.0/nfces', {params})
  }

  async getNfce(accessKey: string): Promise<NfeDetailOut> {
    return this.get(`/v1.0/nfces/${accessKey}`)
  }

  async emitNfce(data: NfceEmit): Promise<NfeDetailOut> {
    return this.post('/v1.0/nfces', data)
  }

  async cancelNfce(accessKey: string, justification: string, sequenceNumber = 1): Promise<NfeDetailOut> {
    return this.post(`/v1.0/nfces/${accessKey}/cancel`, {justification, sequence_number: sequenceNumber})
  }

  async substituteNfce(accessKey: string, substituteKey: string, justification: string, sequenceNumber = 1): Promise<NfeDetailOut> {
    return this.post(`/v1.0/nfces/${accessKey}/substitute`, {
      substitute_key: substituteKey,
      justification,
      sequence_number: sequenceNumber,
    })
  }

  async getNfceEvents(accessKey: string): Promise<PaginatedResponse<NfeEventOut>> {
    return this.get(`/v1.0/nfces/${accessKey}/events`, {params: {limit: 50}})
  }

  async downloadNfceXml(accessKey: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfces/${accessKey}/xml`)
  }

  async downloadNfceEventXml(accessKey: string, eventSk: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfces/${accessKey}/events/${encodeURIComponent(eventSk)}/xml`)
  }

  async downloadNfceDanfe(accessKey: string): Promise<AuxiliaryDocumentDownload> {
    return this.get(`/v1.0/nfces/${accessKey}/danfce`, {timeout: AUXILIARY_DOCUMENT_TIMEOUT_MS})
  }

  // MDF-es (modelo 58) — uses Dfe-Organization-Pk header
  async getMdfes(params?: {
    limit?: number
    cursor?: string
    year?: number
    month?: number
    day?: number
    number?: number
    sort?: 'asc' | 'desc'
  }): Promise<PaginatedResponse<MdfeListOut>> {
    return this.get('/v1.0/mdfes', {params})
  }

  async getMdfe(accessKey: string): Promise<MdfeDetailOut> {
    return this.get(`/v1.0/mdfes/${accessKey}`)
  }

  async emitMdfe(data: MdfeEmit): Promise<MdfeDetailOut> {
    return this.post('/v1.0/mdfes', data)
  }

  async previewMdfeCargo(documents: MdfeDocRef[]): Promise<MdfeCargoPreview> {
    return this.post('/v1.0/mdfes/cargo-preview', {documents})
  }

  async cancelMdfe(accessKey: string, justification: string, sequenceNumber = 1): Promise<MdfeDetailOut> {
    return this.post(`/v1.0/mdfes/${accessKey}/cancel`, {justification, sequence_number: sequenceNumber})
  }

  async closeMdfe(accessKey: string, ibgeCode: string, uf?: string, sequenceNumber = 1): Promise<MdfeDetailOut> {
    return this.post(`/v1.0/mdfes/${accessKey}/close`, {ibge_code: ibgeCode, uf, sequence_number: sequenceNumber})
  }

  async includeMdfeCondutor(accessKey: string, name: string, cpf: string, sequenceNumber = 1): Promise<MdfeDetailOut> {
    return this.post(`/v1.0/mdfes/${accessKey}/include-condutor`, {name, cpf, sequence_number: sequenceNumber})
  }

  async includeMdfeDFe(accessKey: string, loadingIbgeCode: string, loadingCity: string, documents: MdfeIncludeDFeDoc[], sequenceNumber = 1): Promise<MdfeDetailOut> {
    return this.post(`/v1.0/mdfes/${accessKey}/include-dfe`, {
      loading_ibge_code: loadingIbgeCode,
      loading_city: loadingCity,
      documents,
      sequence_number: sequenceNumber,
    })
  }

  async getMdfeEvents(accessKey: string): Promise<PaginatedResponse<NfeEventOut>> {
    return this.get(`/v1.0/mdfes/${accessKey}/events`, {params: {limit: 50}})
  }

  async downloadMdfeXml(accessKey: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/mdfes/${accessKey}/xml`)
  }

  async downloadMdfeEventXml(accessKey: string, eventSk: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/mdfes/${accessKey}/events/${encodeURIComponent(eventSk)}/xml`)
  }

  async downloadMdfeDamdfe(accessKey: string): Promise<AuxiliaryDocumentDownload> {
    return this.get(`/v1.0/mdfes/${accessKey}/damdfe`, {timeout: AUXILIARY_DOCUMENT_TIMEOUT_MS})
  }

  // NFS-e — id é sempre o id_dps (45 chars), nunca a chave de acesso (spec §7 decisão 2)
  async getNfses(params?: {
    limit?: number;
    cursor?: string;
    status?: string;
    number?: number;
    year?: number;
    month?: number;
    sort?: 'asc' | 'desc';
  }): Promise<PaginatedResponse<NfseListOut>> {
    return this.get('/v1.0/nfses', {params})
  }

  async getNfse(id: string): Promise<NfseDetailOut> {
    return this.get(`/v1.0/nfses/${id}`)
  }

  async emitNfse(data: NfseEmit): Promise<NfseDetailOut> {
    return this.post('/v1.0/nfses', data)
  }

  async substituteNfse(id: string, data: NfseEmit): Promise<NfseDetailOut> {
    return this.post(`/v1.0/nfses/${id}/substitute`, data)
  }

  async cancelNfse(id: string, reasonCode: string, reasonDescription: string, sequenceNumber?: number): Promise<NfseDetailOut> {
    return this.post(`/v1.0/nfses/${id}/cancel`, {
      reason_code: reasonCode, reason_description: reasonDescription, sequence_number: sequenceNumber,
    })
  }

  async sendNfseEvent(id: string, data: NfseEventBody): Promise<NfseEventOut> {
    return this.post(`/v1.0/nfses/${id}/events`, data)
  }

  async getNfseEvents(id: string): Promise<PaginatedResponse<NfseEventOut>> {
    return this.get(`/v1.0/nfses/${id}/events`, {params: {limit: 50}})
  }

  async downloadNfseXml(id: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfses/${id}/xml`)
  }

  async downloadNfseDpsXml(id: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfses/${id}/dps-xml`)
  }

  async downloadNfseEventXml(id: string, eventSk: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfses/${id}/events/${encodeURIComponent(eventSk)}/xml`)
  }

  async downloadDanfse(id: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/nfses/${id}/danfse`, {timeout: 12_000})
  }

  async getMunicipalParameters(city: string, kind: string, params?: {
    service?: string;
    competence?: string;
    benefit_number?: string;
  }): Promise<MunicipalParamsOut> {
    return this.get(`/v1.0/nfse/municipal-parameters/${city}/${kind}`, {params})
  }

  // Distribuição ADN — rota dedicada (não a genérica /distributions/{docType}/*,
  // que api/internal/services/distributions.go não registra para "nfse").
  async listNfseDistributions(params?: { limit?: number; cursor?: string }): Promise<PaginatedResponse<NfseDistributionOut>> {
    return this.get('/v1.0/nfse/distributions', {params})
  }

  // ── Inutilização de numeração (NF-e / NFC-e) ──────────────────────────────

  /** Lista as inutilizações da organização, mais recentes primeiro. */
  async listInutilizations(docType: 'nfe' | 'nfce', params?: {
    limit?: number;
    cursor?: string
  }): Promise<PaginatedResponse<InutilizationOut>> {
    return this.get(`/v1.0/${docType}s/inutilizations`, {params})
  }

  /** Faixas de numeração sem documento utilizável e ainda não inutilizadas. */
  async listNumberGaps(docType: 'nfe' | 'nfce'): Promise<{ items: NumberGapOut[] }> {
    return this.get(`/v1.0/${docType}s/inutilizations/gaps`)
  }

  /** Baixa o ProcInutNFe — request assinado + retorno da SEFAZ. */
  async downloadInutilizationXml(docType: 'nfe' | 'nfce', sk: string): Promise<SignedFileDownload> {
    return this.get(`/v1.0/${docType}s/inutilizations/${encodeURIComponent(sk)}/xml`)
  }

  /** Envia uma faixa não utilizada para inutilização na SEFAZ (201). */
  async createInutilization(docType: 'nfe' | 'nfce', body: InutilizationIn): Promise<InutilizationOut> {
    return this.post(`/v1.0/${docType}s/inutilizations`, body)
  }

  async downloadDistributionXml(docType: string, nsu: number): Promise<SignedFileDownload> {
    return this.get(`/v1.0/distributions/${docType}/history/${nsu}/xml`)
  }

  async listDistributions<T extends NFeDistributionOut | NfseDistributionOut = NFeDistributionOut>(docType: string, params?: {
    limit?: number;
    cursor?: string;
    /** Filter history to NSUs containing this value (server-side). */
    nsu?: string
  }): Promise<PaginatedResponse<T>> {
    return this.get(`/v1.0/distributions/${docType}/history`, {params})
  }

  async syncDistributions(docType: string): Promise<SyncEnqueuedOut> {
    return this.post(`/v1.0/distributions/${docType}/sync`)
  }

  async lookupDistributionByNsu(docType: string, nsu: number): Promise<DistributionLookupOut> {
    return this.get(`/v1.0/distributions/${docType}/nsu/${nsu}`)
  }

  /** Enqueues an async consChNFe for the given NF-e access key (202). NF-e only — see DOCS.md. */
  async importNfeByKey(accessKey: string): Promise<{status: string}> {
    return this.post('/v1.0/distributions/nfe/key', {access_key: accessKey})
  }

  // importXML uploads a NF-e/NFC-e XML file for async import (see
  // POST /distributions/{doc_type}/import-xml). Result arrives via WebSocket
  // (new_distribution_nfe on success, import_xml_failed on rejection).
  async importXML(docType: 'nfe' | 'nfce', file: File): Promise<{ status: string }> {
    const formData = new FormData()
    formData.append('file', file)
    return (await this.http.post<{ status: string }>(
      `/v1.0/distributions/${docType}/import-xml`,
      formData,
      {headers: {'Content-Type': undefined}},
    )).data
  }

  // Billing — the account's own subscription.
  //
  // None of these routes accept the organization header: they act on the token
  // holder's account. That is what makes "only the owner creates or changes the
  // subscription" a property of the routing rather than a check someone can
  // forget. `getOrganizationPlan` is the read-only exception, for an ADMIN who
  // needs to see the plan governing the org they help run.

  async listBillingPlans(): Promise<BillingPlansResponse> {
    return this.get('/v1.0/billing/plans')
  }

  async getSubscription(): Promise<AccountSubscription> {
    return this.get('/v1.0/billing/subscription')
  }

  async chooseBillingPlan(body: PlanChoice): Promise<AccountSubscriptionWithInvoice> {
    return this.post('/v1.0/billing/subscription', body)
  }

  async changeBillingPlan(body: PlanChoice): Promise<AccountSubscriptionWithInvoice> {
    return this.post('/v1.0/billing/subscription/change', body)
  }

  async cancelBillingSubscription(atPeriodEnd: boolean): Promise<AccountSubscription> {
    return this.post('/v1.0/billing/subscription/cancel', {at_period_end: atPeriodEnd})
  }

  async listBillingInvoices(year?: number, month?: number): Promise<{ data: BillingInvoice[] }> {
    return this.get('/v1.0/billing/invoices', {params: {year, month}})
  }

  async getOrganizationPlan(orgPk: string): Promise<AccountSubscription> {
    return this.get(`/v1.0/organizations/${orgPk}/plan`)
  }

  // Audit log — org context auto-injected via Dfe-Organization-Pk header
  async getAuditLogs(params?: {
    resourceType?: string
    resourceId?: string
    userId?: string
    limit?: number
    cursor?: string
  }): Promise<PaginatedResponse<AuditLogOut>> {
    return this.get('/v1.0/audit-logs', {
      params: {
        resource_type: params?.resourceType,
        resource_id: params?.resourceId,
        user_id: params?.userId,
        limit: params?.limit,
        cursor: params?.cursor,
      },
    })
  }

  // External lookups
  async lookupOrganization(cpf_cnpj: string, uf: string): Promise<LookupOrganizationOut> {
    return this.get<LookupOrganizationOut>('/v1.0/external/lookup-organizations', {params: {cpf_cnpj, uf}})
  }

  /** Consulta pública por CNPJ, com cache curto e deduplicação em memória para
   * respeitar o limite do CNPJá sem persistir dados de terceiros no browser. */
  async lookupOpenCnpjOffice(cnpj: string): Promise<OpenCnpjOffice> {
    const clean = unformatCpfCnpj(cnpj)
    if (clean.length !== OPEN_CNPJ_DOCUMENT_LENGTH) {
      throw new ApiError(400, 'Informe um CNPJ válido para consultar o CNPJá.')
    }

    const cached = this.openCnpjCache.get(clean)
    if (cached && cached.expiresAt > Date.now()) return cached.value

    const pending = this.openCnpjPending.get(clean)
    if (pending) return pending

    const request = this.openCnpjHttp.get<OpenCnpjOffice>(`/office/${clean}`)
      .then(({data}) => {
        this.openCnpjCache.set(clean, {
          expiresAt: Date.now() + OPEN_CNPJ_CACHE_TTL_MS,
          value: data,
        })
        return data
      })
      .catch(async (error: unknown) => {
        if (!axios.isAxiosError(error)) throw error
        const data = error.response?.data as ErrorResponseBody | undefined
        const status = error.response?.status ?? 0
        const detail = status === 429
          ? 'Limite público do CNPJá atingido. Tente novamente em alguns instantes.'
          : status === 404
            ? 'CNPJ não localizado no CNPJá.'
            : data?.detail ?? data?.title ?? error.message ?? 'Erro ao consultar o CNPJá.'
        throw new ApiError(status, detail, data)
      })
      .finally(() => this.openCnpjPending.delete(clean))

    this.openCnpjPending.set(clean, request)
    return request
  }

  async searchPersonsByName(name: string, role?: PersonRole): Promise<PaginatedResponse<PersonItemOut>> {
    // Person names are stored uppercase (see PersonForm/EntityForm), so the query
    // is uppercased to keep name matching assertive regardless of typed case.
    // `q` (not the legacy `name`) is what the backend pairs with `role`.
    return this.get('/v1.0/persons', {params: {q: name.toUpperCase(), limit: 8, role}})
  }

  async getPersonByCpfCnpj(cpfCnpj: string): Promise<PersonItemOut> {
    return this.get(`/v1.0/persons/${cpfCnpj}`)
  }

  private async get<T>(path: string, config?: AxiosRequestConfig): Promise<T> {
    return (await this.http.get<T>(path, config)).data
  }

  private async post<T>(path: string, body?: unknown, config?: AxiosRequestConfig): Promise<T> {
    return (await this.http.post<T>(path, body, config)).data
  }

  private async put<T>(path: string, body?: unknown): Promise<T> {
    return (await this.http.put<T>(path, body)).data
  }

  private async del<T>(path: string): Promise<T> {
    return (await this.http.delete<T>(path)).data
  }
}

export const apiClient = new ApiClient()
