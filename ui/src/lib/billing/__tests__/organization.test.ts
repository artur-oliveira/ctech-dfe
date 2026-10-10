import {describe, expect, it} from 'vitest';
import {canManagePlan, MEMBER_NO_PLAN_MESSAGE, organizationPlanTitle} from '@/lib/billing/organization';
import type {AccountSubscription} from '@/lib/types/billing';

const base: AccountSubscription = {
  has_subscription: true, status: 'ACTIVE', plan: 'pro', grants_service: true,
  cancel_at_period_end: false, period_start: '', period_end: '', quotas: {}, no_charge: false,
};

describe('organization plan', () => {
  it('names the organization', () => {
    expect(organizationPlanTitle({...base, organization: {id: 'org_1', name: 'Escritório Silva'}}))
      .toBe('Plano da organização Escritório Silva');
  });

  it('falls back when the name is unknown', () => {
    expect(organizationPlanTitle({...base, organization: {id: 'org_1', name: ''}})).toBe('Plano da organização');
    expect(organizationPlanTitle(undefined)).toBe('Plano da organização');
  });

  it('only manageable answers allow managing', () => {
    expect(canManagePlan({...base, manageable: true})).toBe(true);
    expect(canManagePlan({...base, manageable: false})).toBe(false);
    expect(canManagePlan({...base})).toBe(false);
    expect(canManagePlan(undefined)).toBe(false);
  });

  it('tells a member who chooses the plan, without a dash', () => {
    expect(MEMBER_NO_PLAN_MESSAGE).toMatch(/ainda não escolheu um plano/);
    expect(MEMBER_NO_PLAN_MESSAGE).not.toContain('—');
  });
});
