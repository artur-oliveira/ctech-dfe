'use client'

import Link from 'next/link'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {NfseEmitForm} from '@/components/nfse/NfseEmitForm'

function NfseEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/nfse" className="hover:text-brand-600">NFS-e</Link>
            <span>/</span>
            <span className="text-gray-600">Nova NFS-e</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">Emitir NFS-e</h1>
          <p className="text-gray-500 text-sm mt-0.5">Preencha os dados para emitir uma Nota Fiscal de Serviços Eletrônica</p>
        </div>

        <NfseEmitForm/>
      </div>
    </RootLayout>
  )
}

export default function NfseEmitPage() {
  return (
    <ProtectedRoute>
      <NfseEmitContent/>
    </ProtectedRoute>
  )
}
