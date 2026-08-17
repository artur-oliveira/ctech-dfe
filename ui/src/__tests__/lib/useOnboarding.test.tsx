import {beforeEach, describe, expect, it, vi} from 'vitest'
import {renderHook, waitFor} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import type {ReactNode} from 'react'
import {apiClient, ApiError} from '@/lib/api/client'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {STEP_DOCUMENTS, STEP_PRODUCTS, STEP_SERVICES} from '@/lib/constants/onboarding'
import type {AccountSubscription} from '@/lib/types/billing'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

const ORG_PK = 'CNPJ_00000000000191'

const authState = {
  user: {organizations: [{pk: ORG_PK, name: 'Empresa', role: 'OWNER'}]} as never,
  selectedOrg: {pk: ORG_PK, name: 'Empresa', role: 'OWNER'} as never,
}

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => authState,
}))

function wrapper({children}: { children: ReactNode }) {
  const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

const SUBSCRIBED: AccountSubscription = {
  has_subscription: true,
  status: 'ACTIVE',
  plan: 'pro',
  grants_service: true,
  cancel_at_period_end: false,
  period_start: '2026-08-01',
  period_end: '2026-09-01',
  quotas: {nfe: 1200},
  no_charge: false,
}

/** Only the listed document types answer with a config; the rest 404. */
function mockConfigs(present: DocVariant[]) {
  const getters = {
    nfe: 'getNFeConfig',
    nfce: 'getNFCeConfig',
    cte: 'getCTeConfig',
    mdfe: 'getMDFeConfig',
    nfse: 'getNfseConfig',
  } as const
  for (const [variant, method] of Object.entries(getters) as [DocVariant, keyof typeof apiClient][]) {
    const spy = vi.spyOn(apiClient, method) as unknown as {
      mockResolvedValue: (v: unknown) => void
      mockRejectedValue: (v: unknown) => void
    }
    if (present.includes(variant)) spy.mockResolvedValue({environment: '1'})
    else spy.mockRejectedValue(new ApiError(404, 'not found'))
  }
}

describe('useOnboarding', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    vi.spyOn(apiClient, 'getSubscription').mockResolvedValue(SUBSCRIBED)
    vi.spyOn(apiClient, 'getProducts').mockResolvedValue({items: [], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false} as never)
    vi.spyOn(apiClient, 'getServices').mockResolvedValue({items: [], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false} as never)
  })

  it('hides the product and service layers when no document consumes them', async () => {
    mockConfigs(['mdfe'])
    const {result} = renderHook(() => useOnboarding(), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    // A carrier that only issues MDF-e has neither a catalogue of products nor
    // one of services — asking is how a setup flow turns into a form to endure.
    const ids = result.current.steps.map((s) => s.id)
    expect(ids).not.toContain(STEP_PRODUCTS)
    expect(ids).not.toContain(STEP_SERVICES)
  })

  it('offers the product layer once NF-e or NFC-e is configured', async () => {
    mockConfigs(['nfce'])
    const {result} = renderHook(() => useOnboarding(), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.steps.map((s) => s.id)).toContain(STEP_PRODUCTS)
    expect(result.current.nextStep?.id).toBe(STEP_PRODUCTS)
  })

  it('offers the service layer for NFS-e', async () => {
    mockConfigs(['nfse'])
    const {result} = renderHook(() => useOnboarding(), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.steps.map((s) => s.id)).toContain(STEP_SERVICES)
  })

  it('points at the document layer while no document type is configured', async () => {
    mockConfigs([])
    const {result} = renderHook(() => useOnboarding(), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.nextStep?.id).toBe(STEP_DOCUMENTS)
  })

  it('treats a skipped optional layer as answered', async () => {
    mockConfigs(['nfe'])
    const {result, rerender} = renderHook(() => useOnboarding(), {wrapper})

    await waitFor(() => expect(result.current.nextStep?.id).toBe(STEP_PRODUCTS))
    result.current.skip(STEP_PRODUCTS)
    rerender()

    // Declining is a preference, not fiscal state — but the flow must stop
    // asking, or "skip" is just a slower way to be nagged.
    await waitFor(() => expect(result.current.nextStep?.id).not.toBe(STEP_PRODUCTS))
  })

  it('counts a no-charge installation as having answered the plan layer', async () => {
    vi.spyOn(apiClient, 'getSubscription').mockResolvedValue({
      ...SUBSCRIBED,
      has_subscription: false,
      no_charge: true,
    })
    mockConfigs(['nfe'])
    const {result} = renderHook(() => useOnboarding(), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.hasSubscription).toBe(true)
  })
})
