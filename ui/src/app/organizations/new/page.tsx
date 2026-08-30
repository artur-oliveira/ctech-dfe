'use client'

import {useEffect} from 'react'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {startCompanyHandoff} from '@/lib/handoff'

/**
 * Creating a company starts in the CTech account.
 *
 * Not a form here any more. A company's identity belongs to ctech-account
 * (ctech-billing ADR 0022), and creating it locally produced a record the
 * platform never heard of: no company id, no reach edge, and a next migration
 * to go and find it.
 */
function NewOrganizationRedirect() {
  useEffect(() => {
    startCompanyHandoff()
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
