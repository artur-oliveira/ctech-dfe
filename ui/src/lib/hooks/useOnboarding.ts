'use client'

import {useCallback, useMemo, useSyncExternalStore} from 'react'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {STORAGE_KEY_ONBOARDING_SKIPPED_PREFIX} from '@/lib/constants/storage'
import {
  ONBOARDING_STEPS,
  PRODUCT_DOC_VARIANTS,
  SERVICE_DOC_VARIANTS,
  STEP_COMPANY,
  STEP_DOCUMENTS,
  STEP_DONE,
  STEP_PLAN,
  STEP_PRODUCTS,
  STEP_SERVICES,
  type OnboardingStep,
  type StepDefinition,
} from '@/lib/constants/onboarding'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

/** One item of a probe list — we only ever ask "is there at least one?". */
const EXISTENCE_PROBE_LIMIT = 1

export interface OnboardingStepState extends StepDefinition {
  done: boolean
  /** Conditional steps are hidden when the chosen documents don't need them. */
  applicable: boolean
  /** Optional steps end the flow instead of blocking it. */
  optional: boolean
}

function skipStorageKey(orgPk: string | undefined): string {
  return `${STORAGE_KEY_ONBOARDING_SKIPPED_PREFIX}_${orgPk ?? 'none'}`
}

function readSkipped(key: string): OnboardingStep[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as OnboardingStep[]) : []
  } catch {
    // A corrupted preference is not worth an error boundary — the cost of
    // treating it as "nothing skipped" is one extra card on the dashboard.
    return []
  }
}

/** localStorage fires no event for same-tab writes, so skips broadcast their own. */
const SKIP_EVENT = 'dfe:onboarding-skip'

function subscribeSkips(onChange: () => void) {
  window.addEventListener('storage', onChange)
  window.addEventListener(SKIP_EVENT, onChange)
  return () => {
    window.removeEventListener('storage', onChange)
    window.removeEventListener(SKIP_EVENT, onChange)
  }
}

/**
 * Where the user is in first-run setup, derived rather than stored.
 *
 * Every required layer is answered by something the product already knows: the
 * subscription says whether a plan was chosen, `/auth/me` says whether a company
 * exists, the fiscal configs say which documents were set up. Nothing tracks
 * progress separately, so the flow is resumable from any device and cannot
 * disagree with reality — a progress row that says "company: done" for an
 * account with no company is the failure mode this avoids.
 *
 * The optional layers (products, services) are the exception: declining them is
 * a preference, kept in localStorage.
 */
export function useOnboarding() {
  const {user, selectedOrg} = useAuth()
  const {subscription, isPending: subPending} = useSubscription()

  const orgPk = selectedOrg?.pk
  const storageKey = skipStorageKey(orgPk)

  const skipped = useSyncExternalStore(
    subscribeSkips,
    useCallback(() => window.localStorage.getItem(storageKey) ?? '', [storageKey]),
    () => '',
  )
  const skippedSteps = useMemo(() => (skipped ? readSkipped(storageKey) : []), [skipped, storageKey])

  const skip = useCallback(
    (step: OnboardingStep) => {
      const next = Array.from(new Set([...readSkipped(storageKey), step]))
      window.localStorage.setItem(storageKey, JSON.stringify(next))
      window.dispatchEvent(new Event(SKIP_EVENT))
    },
    [storageKey],
  )

  // One query per document type; each treats 404 as "not configured yet".
  const nfe = useFiscalConfig('nfe', orgPk)
  const nfce = useFiscalConfig('nfce', orgPk)
  const cte = useFiscalConfig('cte', orgPk)
  const mdfe = useFiscalConfig('mdfe', orgPk)
  const nfse = useFiscalConfig('nfse', orgPk)

  const configured = useMemo<Record<DocVariant, boolean>>(
    () => ({
      nfe: !!nfe.config,
      nfce: !!nfce.config,
      cte: !!cte.config,
      mdfe: !!mdfe.config,
      nfse: !!nfse.config,
    }),
    [nfe.config, nfce.config, cte.config, mdfe.config, nfse.config],
  )

  const configsPending =
    nfe.isPending || nfce.isPending || cte.isPending || mdfe.isPending || nfse.isPending

  const needsProducts = PRODUCT_DOC_VARIANTS.some((v) => configured[v])
  const needsServices = SERVICE_DOC_VARIANTS.some((v) => configured[v])

  const productsQuery = useQuery({
    queryKey: queryKeys.products.probe(orgPk),
    queryFn: () => apiClient.getProducts({limit: EXISTENCE_PROBE_LIMIT}),
    enabled: !!orgPk && needsProducts,
  })
  const servicesQuery = useQuery({
    queryKey: queryKeys.services.probe(orgPk),
    queryFn: () => apiClient.getServices({limit: EXISTENCE_PROBE_LIMIT}),
    enabled: !!orgPk && needsServices,
  })

  const hasSubscription = !!subscription?.has_subscription || !!subscription?.no_charge
  const hasCompany = (user?.organizations.length ?? 0) > 0
  const hasAnyConfig = Object.values(configured).some(Boolean)
  const hasProducts = (productsQuery.data?.items.length ?? 0) > 0
  const hasServices = (servicesQuery.data?.items.length ?? 0) > 0

  const steps = useMemo<OnboardingStepState[]>(() => {
    const doneById: Record<OnboardingStep, boolean> = {
      [STEP_PLAN]: hasSubscription,
      [STEP_COMPANY]: hasCompany,
      [STEP_DOCUMENTS]: hasAnyConfig,
      [STEP_PRODUCTS]: hasProducts || skippedSteps.includes(STEP_PRODUCTS),
      [STEP_SERVICES]: hasServices || skippedSteps.includes(STEP_SERVICES),
      [STEP_DONE]: false,
    }
    const applicableById: Record<OnboardingStep, boolean> = {
      [STEP_PLAN]: true,
      [STEP_COMPANY]: true,
      [STEP_DOCUMENTS]: true,
      [STEP_PRODUCTS]: needsProducts,
      [STEP_SERVICES]: needsServices,
      [STEP_DONE]: true,
    }
    const optionalById: Record<OnboardingStep, boolean> = {
      [STEP_PLAN]: false,
      [STEP_COMPANY]: false,
      [STEP_DOCUMENTS]: false,
      [STEP_PRODUCTS]: true,
      [STEP_SERVICES]: true,
      [STEP_DONE]: false,
    }
    return ONBOARDING_STEPS.map((s) => ({
      ...s,
      done: doneById[s.id],
      applicable: applicableById[s.id],
      optional: optionalById[s.id],
    }))
  }, [hasSubscription, hasCompany, hasAnyConfig, hasProducts, hasServices, needsProducts, needsServices, skippedSteps])

  const visibleSteps = useMemo(() => steps.filter((s) => s.applicable), [steps])
  const nextStep = useMemo(() => visibleSteps.find((s) => !s.done), [visibleSteps])

  return {
    steps: visibleSteps,
    /** The first unfinished layer — where "continuar" goes. */
    nextStep,
    /** Every required layer answered; only the optional ones may remain. */
    complete: visibleSteps.every((s) => s.done || s.id === STEP_DONE),
    configured,
    hasSubscription,
    hasCompany,
    isPending: subPending || (!!orgPk && configsPending),
    skip,
  }
}
