'use client'

import {useEffect} from 'react'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'

/**
 * Creating a company starts in the CTech account.
 *
 * Not a form here any more. A company's identity — its CNPJ and legal name —
 * belongs to ctech-account (ctech-billing ADR 0022), and creating it locally
 * produced a record the platform never heard of: no company id, no reach edge,
 * and a next migration to go and find it.
 *
 * So this redirects, carrying `return_to` so the account can send the person
 * back with both ids. The DF-e then asks only for what is its own: the A1
 * certificate, the inscrição estadual, the série.
 */
function NewOrganizationRedirect() {
  useEffect(() => {
    const accountUrl = process.env.NEXT_PUBLIC_CTECH_CLIENT_URL ?? ''
    const returnTo = `${window.location.origin}/organizations/link`
    // state is ours and opaque to the account: it echoes it back untouched, and
    // it is what tells a return apart from somebody opening the landing URL by
    // hand.
    const state = crypto.randomUUID()
    sessionStorage.setItem('dfe:handoff-state', state)

    const url = new URL(`${accountUrl}/account/organizations/new`)
    url.searchParams.set('client_id', 'dfe')
    url.searchParams.set('return_to', returnTo)
    url.searchParams.set('state', state)
    // replace, not assign: the person must not land back here with the browser
    // Back button and start a second handoff.
    window.location.replace(url.toString())
  }, [])

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <h1 className="text-2xl font-semibold text-gray-900 mb-2">Nova empresa</h1>
        <p className="text-gray-500 text-sm">
          Levando você para a conta CTech, onde a empresa é cadastrada…
        </p>
      </div>
    </RootLayout>
  )
}

export default function NewOrganizationPage() {
  return (
    <ProtectedRoute>
      <NewOrganizationRedirect/>
    </ProtectedRoute>
  )
}
