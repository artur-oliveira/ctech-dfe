'use client'

import {useAuth} from '@/lib/hooks/useAuth'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {noticeForSubscription, type BillingNotice} from '@/lib/billing/notice'
import {ROLE_OWNER} from '@/lib/data/roles'

/**
 * The standing billing warning for the current user, or null when there is none.
 *
 * Only the owner sees it. `GET /v1.0/billing/subscription` answers about the
 * caller's **own** account, and an invited member has no subscription of their
 * own — reading that snapshot for them would produce "escolha um plano" on a
 * screen governed by somebody else's plan that is working fine.
 */
export function useSubscriptionNotice(): { notice: BillingNotice | null; isPending: boolean } {
  const {selectedOrg} = useAuth()
  const {subscription, isPending} = useSubscription()

  if (selectedOrg && selectedOrg.role !== ROLE_OWNER) return {notice: null, isPending: false}
  return {notice: noticeForSubscription(subscription), isPending}
}
