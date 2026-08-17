'use client'

import {useEffect, type ReactNode} from 'react'
import {useRouter} from 'next/navigation'
import {useAuth} from '@/lib/hooks/useAuth'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {ConfigRequiredBanner} from '@/components/ui/config-required-banner'
import {SubscriptionBlocked} from '@/components/billing/SubscriptionNotice'
import {useSubscriptionNotice} from '@/lib/hooks/useSubscriptionNotice'

const DOC_LABELS: Record<DocVariant, string> = {
  nfe: 'NF-e',
  nfce: 'NFC-e',
  cte: 'CT-e',
  mdfe: 'MDF-e',
  nfse: 'NFS-e',
}

/**
 * Everything that has to be true before an emission form is worth filling in:
 * an organization, an active fiscal config (numbering, environment,
 * certificate), and a subscription that grants service.
 *
 * The billing check lives here rather than in each form because the alternative
 * is finding out after fifty fields that the account cannot issue — the API
 * refuses with a 402 either way, but at the end of the form is the worst moment
 * to say so. Missing config still redirects to the config tab, with the alert as
 * a fallback in case navigation hasn't landed yet.
 */
export function RequireFiscalConfig({variant, children}: { variant: DocVariant; children: ReactNode }) {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const {isPending, isMissing} = useFiscalConfig(variant, selectedOrg?.pk)
  const {notice, isPending: subscriptionPending} = useSubscriptionNotice()

  useEffect(() => {
    if (isMissing) router.replace(`/fiscal-config?tab=${variant}`)
  }, [isMissing, variant, router])

  if (!selectedOrg) return <NoOrgBanner/>
  if (isPending || subscriptionPending) return <LoadingSkeleton/>
  if (isMissing) {
    return <ConfigRequiredBanner show variant={variant} docLabel={DOC_LABELS[variant]}/>
  }
  if (notice) return <SubscriptionBlocked/>
  return <>{children}</>
}
