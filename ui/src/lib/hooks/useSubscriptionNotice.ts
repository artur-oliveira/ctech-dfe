'use client';

import {useSubscription} from '@/lib/hooks/useSubscription';
import {noticeForSubscription, type BillingNotice} from '@/lib/billing/notice';
import {canManagePlan} from '@/lib/billing/organization';

/**
 * The standing billing warning for the current user, or null when there is none.
 *
 * Only the organization's owners and admins see it: they are the ones who can
 * act on it. The API says who they are (`manageable`); the DF-e role does not.
 */
export function useSubscriptionNotice(): { notice: BillingNotice | null; isPending: boolean } {
  const {subscription, isPending} = useSubscription();
  if (!canManagePlan(subscription)) return {notice: null, isPending: false};
  return {notice: noticeForSubscription(subscription), isPending};
}
