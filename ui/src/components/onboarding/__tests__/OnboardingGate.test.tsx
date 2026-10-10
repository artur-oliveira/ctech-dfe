import {describe, expect, it, vi, beforeEach} from 'vitest';
import {render, screen} from '@testing-library/react';
import type {AccountSubscription} from '@/lib/types/billing';

const replace = vi.fn();
const nav = {pathname: '/'};
vi.mock('next/navigation', () => ({
  usePathname: () => nav.pathname,
  useRouter: () => ({replace}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({user: {organizations: [{pk: 'cmp_1', role: 'OWNER'}]}}),
}));
const noPlan: AccountSubscription = {
  has_subscription: false, status: '', plan: '', grants_service: false, cancel_at_period_end: false,
  period_start: '', period_end: '', quotas: {}, no_charge: false, manageable: true,
};
vi.mock('@/lib/hooks/useSubscription', () => ({
  useSubscription: () => ({subscription: noPlan, isPending: false, error: null}),
}));

import {OnboardingGate} from '@/components/onboarding/OnboardingGate';

describe('OnboardingGate', () => {
  beforeEach(() => replace.mockReset());

  it('sends an owner whose organization has no plan to the plan step', () => {
    nav.pathname = '/';
    render(<OnboardingGate><p>painel</p></OnboardingGate>);
    expect(replace).toHaveBeenCalledWith('/onboarding/plano');
  });

  // The company step ends on the company's own record (the handoff tail),
  // which comes before the organization's plan.
  it('lets the company record be completed before the plan', () => {
    nav.pathname = '/organizations/edit';
    render(<OnboardingGate><p>dados da empresa</p></OnboardingGate>);
    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByText('dados da empresa')).toBeInTheDocument();
  });
});
