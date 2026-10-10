'use client';

import {useEffect, type ReactNode} from 'react';
import {usePathname, useRouter} from 'next/navigation';
import {useAuth} from '@/lib/hooks/useAuth';
import {useSubscription} from '@/lib/hooks/useSubscription';
import {ONBOARDING_ROOT} from '@/lib/constants/onboarding';
import {onboardingTarget} from '@/lib/onboarding/target';

/**
 * Routes that must stay reachable without a plan or a company.
 *
 * The company record (`/organizations/link` and `/organizations/edit`, the
 * handoff's return leg) is part of the company step, which comes before the
 * organization's plan; saving it continues the setup flow to the plan.
 */
const EXEMPT_PREFIXES = [ONBOARDING_ROOT, '/invite', '/callback', '/login', '/terms-addendum', '/organizations/link', '/organizations/edit'];

/**
 * Sends an account that has not finished the required layers of setup into the
 * onboarding flow.
 *
 * Two rules keep this from catching the wrong people:
 *
 * - **Only those who can choose the plan are sent to it**: the organization's
 *   owners and admins, as ctech-account says (`manageable`). Everybody else
 *   operates under the organization's plan and is never asked to pick one.
 * - **A failed lookup lets the user through.** The subscription is a
 *   convenience snapshot, and a network blip is not a reason to lock an account
 *   out of a product it already pays for. The API blocks issuance on its own
 *   side; this gate only decides where to point someone.
 */
export function OnboardingGate({children}: { children: ReactNode }) {
  const {user} = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const {subscription, isPending, error} = useSubscription();

  const exempt = EXEMPT_PREFIXES.some((p) => pathname.startsWith(p));
  const organizations = user?.organizations ?? [];
  const target = onboardingTarget({hasCompany: organizations.length > 0, subscription});
  const shouldRedirect = !exempt && !error && !!target;

  useEffect(() => {
    if (shouldRedirect && target) router.replace(target);
  }, [shouldRedirect, target, router]);

  if (exempt || error) return <>{children}</>;

  // Holding the page while the snapshot loads avoids a flash of the dashboard
  // for an account that is about to be redirected out of it.
  // The subscription query is disabled without a selected company, and a
  // disabled query stays pending: only wait for it when there is a company.
  if ((isPending && organizations.length > 0) || shouldRedirect) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div
          className="h-10 w-10 animate-spin rounded-full border-4 border-brand-100 border-t-brand-600 motion-reduce:animate-none"
          role="status"
          aria-label="Carregando"
        />
      </div>
    );
  }

  return <>{children}</>;
}
