'use client'

import {Suspense} from 'react'
import Link from 'next/link'
import {useSearchParams} from 'next/navigation'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {NfseEmitForm} from '@/components/nfse/NfseEmitForm'

function NfseEmitContent() {
  const params = useSearchParams()
  const sourceIdDps = params.get('substitute') ?? undefined
  const isSubstitution = !!sourceIdDps

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/nfse" className="hover:text-brand-600">NFS-e</Link>
            <span>/</span>
            <span className="text-gray-600">{isSubstitution ? 'Substituir NFS-e' : 'Nova NFS-e'}</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">{isSubstitution ? 'Substituir NFS-e' : 'Emitir NFS-e'}</h1>
          <p className="text-gray-500 text-sm mt-0.5">
            {isSubstitution
              ? 'Confira os dados da nova DPS e informe o motivo da substituição.'
              : 'Preencha os dados para emitir uma Nota Fiscal de Serviços Eletrônica.'}
          </p>
        </div>

        <NfseEmitForm mode={isSubstitution ? 'substitute' : 'emit'} sourceIdDps={sourceIdDps}/>
      </div>
    </RootLayout>
  )
}

export default function NfseEmitPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfseEmitContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
