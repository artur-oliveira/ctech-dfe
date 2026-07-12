'use client'

import Link from 'next/link'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {NfceEmitForm} from '@/components/nfce/NfceEmitForm'

function NfceEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/nfce" className="hover:text-brand-600">NFC-e</Link>
            <span>/</span>
            <span className="text-gray-600">Nova NFC-e</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">Emitir NFC-e</h1>
          <p className="text-gray-500 text-sm mt-0.5">Nota Fiscal de Consumidor Eletrônica</p>
        </div>

        <NfceEmitForm/>
      </div>
    </RootLayout>
  )
}

export default function NfceEmitPage() {
  return (
    <ProtectedRoute>
      <NfceEmitContent/>
    </ProtectedRoute>
  )
}
