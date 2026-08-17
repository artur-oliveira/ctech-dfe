'use client'

import Link from 'next/link'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {useSubscriptionNotice} from '@/lib/hooks/useSubscriptionNotice'
import {buttonVariants} from '@/components/ui/button'
import {cn} from '@/lib/utils'
import {formatCents} from '@/lib/constants/billing'
import {formatISODateBR} from '@/lib/utils/dfe'

/**
 * The persistent strip that says the account cannot issue and why.
 *
 * It carries the invoice's amount and due date whenever there is one to pay:
 * "pagamento em atraso" with no number is a message somebody ignores until the
 * emission fails, and then calls support about.
 */
export function SubscriptionBanner() {
  const {notice} = useSubscriptionNotice()
  const {subscription} = useSubscription()
  if (!notice) return null

  const invoice = subscription?.open_invoice

  return (
    <div className="border-b border-amber-200 bg-amber-50 px-4 py-3 md:px-8">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-amber-900">
          <strong className="font-semibold">{notice.title}.</strong>{' '}
          {invoice
            ? `Fatura de ${formatCents(invoice.total_cents)} com vencimento em ${formatISODateBR(invoice.due_date)}.`
            : notice.message}
        </p>
        {invoice?.checkout_url ? (
          <a
            href={invoice.checkout_url}
            className={cn(buttonVariants({size: 'sm'}), 'shrink-0')}
          >
            Pagar fatura
          </a>
        ) : (
          <Link href={notice.href} className={cn(buttonVariants({size: 'sm'}), 'shrink-0')}>
            {notice.actionLabel}
          </Link>
        )}
      </div>
    </div>
  )
}

/**
 * The same refusal, as an empty state where a form would be.
 *
 * Rendered instead of an emission form rather than after submitting one: a
 * fiscal form is fifty fields, and finding out at the end that the account
 * cannot issue is the worst possible moment to say it.
 */
export function SubscriptionBlocked() {
  const {notice} = useSubscriptionNotice()
  if (!notice) return null

  return (
    <div className="mx-auto max-w-md rounded-xl border border-gray-200 bg-white p-6 text-center">
      <h2 className="text-base font-semibold text-gray-900">{notice.title}</h2>
      <p className="mt-2 text-sm leading-relaxed text-gray-600 text-pretty">{notice.message}</p>
      <Link href={notice.href} className={cn(buttonVariants(), 'mt-5 w-full sm:w-auto')}>
        {notice.actionLabel}
      </Link>
    </div>
  )
}
