'use client'

import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import type {AccountSubscription} from '@/lib/types/billing'

/**
 * The account's subscription.
 *
 * A snapshot the API keeps current from billing webhooks, not a synchronous
 * lookup — which is why it is cheap enough to sit behind the route gate and the
 * blocking banners at once. It is cached for a minute because the two things
 * that change it (a webhook landing, a plan being chosen) both invalidate the
 * key explicitly.
 */
export function useSubscription() {
  const {user} = useAuth()
  const query = useQuery<AccountSubscription>({
    queryKey: queryKeys.billing.subscription(),
    queryFn: () => apiClient.getSubscription(),
    enabled: !!user,
    staleTime: 60_000,
  })

  return {
    subscription: query.data,
    isPending: query.isPending,
    error: query.error,
    refetch: query.refetch,
  }
}
