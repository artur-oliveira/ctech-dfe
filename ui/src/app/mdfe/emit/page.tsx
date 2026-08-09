'use client'

import Link from 'next/link'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig'
import {MdfeEmitForm} from '@/components/mdfe/MdfeEmitForm'

function MdfeEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/mdfe" className="hover:text-brand-600">MDF-e</Link>
            <span>/</span>
            <span className="text-gray-600">Novo MDF-e</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">Emitir MDF-e</h1>
          <p className="text-gray-500 text-sm mt-0.5">Manifesto Eletrônico de Documentos Fiscais</p>
        </div>
        
        <RequireFiscalConfig variant="mdfe">
          <MdfeEmitForm/>
        </RequireFiscalConfig>
      </div>
    </RootLayout>
  )
}

export default function MdfeEmitPage() {
  return (
    <ProtectedRoute>
      <MdfeEmitContent/>
    </ProtectedRoute>
  )
}
