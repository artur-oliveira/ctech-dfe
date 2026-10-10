'use client';

import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import type {AccountSubscription} from '@/lib/types/billing';

/**
 * The subscription of the selected company's ctech-account organization.
 *
 * Switching between two companies of one organization refetches and shows the
 * same plan.
 *
 * A snapshot the API keeps current from billing webhooks, not a synchronous
 * lookup — which is why it is cheap enough to sit behind the route gate and the
 * blocking banners at once. It is cached for a minute because the two things
 * that change it (a webhook landing, a plan being chosen) both invalidate the
 * key explicitly.
 */
export function useSubscription() {
  const {user, selectedOrg} = useAuth();
  const query = useQuery<AccountSubscription>({
    queryKey: queryKeys.billing.subscription(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getSubscription(),
    enabled: !!user && !!selectedOrg,
    staleTime: 60_000,
  });

  return {
    subscription: query.data,
    isPending: query.isPending,
    error: query.error,
    refetch: query.refetch,
  };
}
