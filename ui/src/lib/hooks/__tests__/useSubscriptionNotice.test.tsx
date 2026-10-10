import {describe, expect, it, vi} from 'vitest';
import {renderHook} from '@testing-library/react';
import type {AccountSubscription} from '@/lib/types/billing';

const state: { subscription?: AccountSubscription } = {};
vi.mock('@/lib/hooks/useSubscription', () => ({
  useSubscription: () => ({subscription: state.subscription, isPending: false}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'cmp_1', role: 'USER'}})}));

import {useSubscriptionNotice} from '@/lib/hooks/useSubscriptionNotice';

const missing: AccountSubscription = {
  has_subscription: false, status: '', plan: '', grants_service: false, cancel_at_period_end: false,
  period_start: '', period_end: '', quotas: {}, no_charge: false,
};

describe('useSubscriptionNotice', () => {
  it('warns an owner or admin of the organization, whatever their DF-e role', () => {
    state.subscription = {...missing, manageable: true};
    expect(renderHook(() => useSubscriptionNotice()).result.current.notice).not.toBeNull();
  });

  it('stays silent for anyone who cannot act on it', () => {
    state.subscription = {...missing, manageable: false};
    expect(renderHook(() => useSubscriptionNotice()).result.current.notice).toBeNull();
  });
});
