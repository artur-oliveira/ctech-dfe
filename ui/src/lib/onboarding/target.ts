import {ONBOARDING_ROOT, STEP_CHECKOUT_RETURN, STEP_COMPANY, STEP_PLAN} from '@/lib/constants/onboarding';
import {STATUS_INCOMPLETE} from '@/lib/constants/billing';
import {canManagePlan} from '@/lib/billing/organization';
import type {AccountSubscription} from '@/lib/types/billing';

/**
 * Where the first-run gate sends someone, or null to let them through.
 *
 * The company comes first: the plan belongs to the organization that holds it,
 * so there is nothing to subscribe before a company is linked. Only those who
 * can choose the plan (the organization's owners and admins) are sent to it.
 */
export function onboardingTarget({hasCompany, subscription}: {
  hasCompany: boolean
  subscription?: AccountSubscription
}): string | null {
  if (!hasCompany) return `${ONBOARDING_ROOT}/${STEP_COMPANY}`;
  if (!subscription || subscription.no_charge || !canManagePlan(subscription)) return null;
  if (subscription.status === STATUS_INCOMPLETE) return `${ONBOARDING_ROOT}/${STEP_CHECKOUT_RETURN}`;
  if (!subscription.has_subscription) return `${ONBOARDING_ROOT}/${STEP_PLAN}`;
  return null;
}
