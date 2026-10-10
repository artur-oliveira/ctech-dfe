import {describe, expect, it} from 'vitest';
import {onboardingTarget} from '@/lib/onboarding/target';
import type {AccountSubscription} from '@/lib/types/billing';

const none: AccountSubscription = {
  has_subscription: false, status: '', plan: '', grants_service: false, cancel_at_period_end: false,
  period_start: '', period_end: '', quotas: {}, no_charge: false,
};

describe('onboardingTarget', () => {
  it('asks for a company first: there is no organization to subscribe before it', () => {
    expect(onboardingTarget({hasCompany: false, subscription: undefined})).toBe('/onboarding/empresa');
  });

  it('sends an owner or admin of an organization with no plan to the plan step', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, manageable: true}})).toBe('/onboarding/plano');
  });

  it('never gates someone who cannot choose the plan', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, manageable: false}})).toBeNull();
  });

  it('waits for the payment of an incomplete subscription', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, has_subscription: true, status: 'INCOMPLETE', manageable: true}}))
      .toBe('/onboarding/retorno');
  });

  it('lets a second company of an organization that already pays straight through', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, has_subscription: true, status: 'ACTIVE', manageable: true}})).toBeNull();
  });

  it('no-charge installations skip the plan', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, no_charge: true, manageable: true}})).toBeNull();
  });
});
