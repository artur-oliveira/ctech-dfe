'use client'

import {useEffect, type ReactNode} from 'react'
import {useRouter} from 'next/navigation'
import {useAuth} from '@/lib/hooks/useAuth'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {ConfigRequiredBanner} from '@/components/ui/config-required-banner'

const DOC_LABELS: Record<DocVariant, string> = {
  nfe: 'NF-e',
  nfce: 'NFC-e',
  cte: 'CT-e',
  mdfe: 'MDF-e',
  nfse: 'NFS-e',
}

/**
 * Emission requires an active fiscal config (numbering, environment,
 * certificate). Redirects to the doc type's config tab the first time it's
 * missing, with the alert as a fallback in case navigation hasn't landed yet.
 */
export function RequireFiscalConfig({variant, children}: { variant: DocVariant; children: ReactNode }) {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const {isPending, isMissing} = useFiscalConfig(variant, selectedOrg?.pk)

  useEffect(() => {
    if (isMissing) router.replace(`/fiscal-config?tab=${variant}`)
  }, [isMissing, variant, router])

  if (!selectedOrg) return <NoOrgBanner/>
  if (isPending) return <LoadingSkeleton/>
  if (isMissing) {
    return <ConfigRequiredBanner show variant={variant} docLabel={DOC_LABELS[variant]}/>
  }
  return <>{children}</>
}
