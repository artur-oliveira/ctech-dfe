import type {AccountSubscription} from '@/lib/types/billing';

/**
 * The DF-e plan belongs to the ctech-account organization of the selected
 * company. Its owners and admins manage it; everybody else reads it. The API
 * says which with `manageable`; the screen never derives it from the DF-e role.
 */

export const MEMBER_NO_PLAN_MESSAGE =
  'A organização ainda não escolheu um plano. Peça a um proprietário ou administrador da organização para escolher.';

export function organizationPlanTitle(sub?: AccountSubscription): string {
  const name = sub?.organization?.name?.trim();
  return name ? `Plano da organização ${name}` : 'Plano da organização';
}

export function canManagePlan(sub?: AccountSubscription): boolean {
  return sub?.manageable === true;
}
