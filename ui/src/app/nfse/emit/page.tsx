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
  const duplicateIdDps = params.get('duplicate') ?? undefined
  const mode = sourceIdDps ? 'substitute' : duplicateIdDps ? 'duplicate' : 'emit'
  const source = sourceIdDps ?? duplicateIdDps
  const title = mode === 'substitute' ? 'Substituir NFS-e' : mode === 'duplicate' ? 'Duplicar NFS-e' : 'Emitir NFS-e'

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/nfse" className="hover:text-brand-600">NFS-e</Link>
            <span>/</span>
            <span className="text-gray-600">{mode === 'emit' ? 'Nova NFS-e' : title}</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">{title}</h1>
          <p className="text-gray-500 text-sm mt-0.5">
            {mode === 'substitute'
              ? 'Confira os dados da nova DPS e informe o motivo da substituição.'
              : mode === 'duplicate'
                ? 'Revise a cópia da DPS; a competência foi avançada em um mês.'
                : 'Preencha os dados para emitir uma Nota Fiscal de Serviços Eletrônica.'}
          </p>
        </div>

        <NfseEmitForm mode={mode} sourceIdDps={source}/>
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
