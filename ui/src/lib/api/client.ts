import axios, {AxiosError, type AxiosAdapter, type AxiosInstance, type AxiosRequestConfig, type AxiosResponse} from 'axios'
import type {
  OperationCreate,
  PaymentTermCreate,
  PaymentTermItemOut,
  VehicleSetCreate,
  VehicleSetItemOut,
  OperationItemOut,
  TaxProfileCreate,
  TaxProfileItemOut,
  AuditLogOut,
  CertificateOut,
  CTeConfigOut,
  DistributionLookupOut,
  InvitationOut,
  InvitationPreview,
  LookupOrganizationOut,
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
} from '@/lib/types/api'
import {unformatCpfCnpj} from "@/lib/utils/document";
import {STORAGE_KEY_ORG} from '@/lib/constants/storage'
import {isStrippableBody, stripNulls} from '@/lib/utils/strip-nulls'
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
const ORG_HEADER = 'Dfe-Organization-Pk'

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
    headers: {'Content-Type': 'application/json'},
  })

  instance.interceptors.request.use((config) => {
    if (_accessToken) config.headers.Authorization = `Bearer ${_accessToken}`

    if (typeof window !== 'undefined') {
      const orgRaw = localStorage.getItem(STORAGE_KEY_ORG)
      if (orgRaw) {
        try {
          const org = JSON.parse(orgRaw) as { pk: string }
          if (org?.pk) config.headers[ORG_HEADER] = unformatCpfCnpj(org.pk)
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
      const original = error.config as (AxiosRequestConfig & { _retry?: boolean }) | undefined
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
          const {startOAuthFlow} = await import('@/lib/auth/oauth')
          const returnTo = window.location.pathname === '/callback' ? '/' : window.location.pathname
          await startOAuthFlow(returnTo)
        }
        return
      }
      const data = error.response ? await parseResponseErrToJson(error.response) : undefined
      const detail = data?.detail ?? data?.title ?? error.message ?? `HTTP ${error.response?.status}`
      throw new ApiError(error.response?.status ?? 0, detail, data)
    },
  )

  return instance
}

class ApiClient {
  private readonly http: AxiosInstance

  constructor() {
    this.http = createAxiosInstance()
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

  // createOrganization sends multipart: the org JSON in `data`, plus the A1
  // certificate (PFX + password) unless it can be inherited from a matriz org
  // sharing the same CNPJ root (filial).
  async createOrganization(
    data: unknown,
    cert?: { file: File; password: string },
  ): Promise<OrganizationOut> {
    const formData = new FormData()
    formData.append('data', JSON.stringify(data))
    if (cert) {
      formData.append('file', cert.file)
      formData.append('password', cert.password)
    }
    return (await this.http.post<OrganizationOut>(
      '/v1.0/organizations',
      formData,
      {headers: {'Content-Type': undefined}},
    )).data
  }

  // certificateRequirement reports whether creating the given org requires an
  // A1 upload (false when a matriz certificate can be inherited).
  async certificateRequirement(cpfOrCnpj: string): Promise<{ required: boolean }> {
    return this.get(`/v1.0/organizations/certificate-requirement?cpf_or_cnpj=${unformatCpfCnpj(cpfOrCnpj)}`)
  }

  async updateOrganization(pk: string, data: unknown): Promise<OrganizationOut> {
    return this.put<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(pk)}`, data)
  }

  async deleteOrganization(pk: string): Promise<void> {
    return this.del<void>(`/v1.0/organizations/${unformatCpfCnpj(pk)}`)
  }

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

  async downloadNfeEventXml(accessKey: string, eventSk: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfes/${accessKey}/events/${encodeURIComponent(eventSk)}/xml`, {responseType: 'blob'})).data
  }

  async downloadNfeXml(accessKey: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfes/${accessKey}/xml`, {responseType: 'blob'})).data
  }

  async downloadNfeDanfe(accessKey: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfes/${accessKey}/danfe`, {responseType: 'blob'})).data
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

  async downloadNfceXml(accessKey: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfces/${accessKey}/xml`, {responseType: 'blob'})).data
  }

  async downloadNfceEventXml(accessKey: string, eventSk: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfces/${accessKey}/events/${encodeURIComponent(eventSk)}/xml`, {responseType: 'blob'})).data
  }

  async downloadNfceDanfe(accessKey: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfces/${accessKey}/danfce`, {responseType: 'blob'})).data
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

  async downloadMdfeXml(accessKey: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/mdfes/${accessKey}/xml`, {responseType: 'blob'})).data
  }

  async downloadMdfeEventXml(accessKey: string, eventSk: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/mdfes/${accessKey}/events/${encodeURIComponent(eventSk)}/xml`, {responseType: 'blob'})).data
  }

  async downloadMdfeDamdfe(accessKey: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/mdfes/${accessKey}/damdfe`, {responseType: 'blob'})).data
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

  async downloadNfseXml(id: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfses/${id}/xml`, {responseType: 'blob'})).data
  }

  async downloadNfseDpsXml(id: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfses/${id}/dps-xml`, {responseType: 'blob'})).data
  }

  async downloadNfseEventXml(id: string, eventSk: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfses/${id}/events/${encodeURIComponent(eventSk)}/xml`, {responseType: 'blob'})).data
  }

  async downloadDanfse(id: string): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/nfses/${id}/danfse`, {responseType: 'blob'})).data
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

  async downloadDistributionXml(docType: string, nsu: number): Promise<Blob> {
    return (await this.http.get<Blob>(`/v1.0/distributions/${docType}/history/${nsu}/xml`, {responseType: 'blob'})).data
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
