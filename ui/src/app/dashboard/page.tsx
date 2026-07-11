'use client'

import Link from 'next/link'
import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {DFE_DOCUMENTS} from '@/lib/constants/dfe-documents'

const quickActions = DFE_DOCUMENTS

function DashboardContent() {
  const {user, selectedOrg} = useAuth()

  return (
    <RootLayout>
      <div className="p-4 md:p-8 max-w-5xl">
        <div className="mb-8">
          <h1 className="text-2xl font-semibold text-gray-900">
            Olá, {user?.first_name}
          </h1>
          <p className="text-gray-500 mt-1 text-sm">
            {selectedOrg
              ? `Você está gerenciando ${selectedOrg.name}`
              : 'Selecione uma organização para começar'}
          </p>
        </div>

        {/* Quick access */}
        <div className="mb-10">
          <h2 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-4">
            Acesso rápido
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {quickActions.map((action) => (
              <Link
                key={action.href}
                href={action.href}
                className="group flex flex-col gap-3 p-5 bg-white rounded-xl border border-gray-200 hover:border-gray-300 hover:shadow-card-hover transition-all"
              >
                <div
                  className="w-10 h-10 rounded-lg flex items-center justify-center text-white shrink-0"
                  style={{backgroundColor: action.accent}}
                >
                  {action.icon}
                </div>
                <div>
                  <p className="text-sm font-semibold text-gray-900 group-hover:text-gray-700">
                    {action.title}
                  </p>
                  <p className="text-xs text-gray-500 mt-0.5 leading-relaxed">
                    {action.description}
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </div>

        {/* Getting started checklist */}
        {!selectedOrg && (
          <div className="bg-white rounded-xl border border-gray-200 p-6">
            <h2 className="text-base font-semibold text-gray-900 mb-4">Primeiros passos</h2>
            <ul className="space-y-3">
              {[
                {text: 'Criar uma organização', href: '/organizations'},
                {text: 'Fazer upload do certificado A1', href: '/organizations'},
                {text: 'Configurar NF-e', href: '/organizations'},
                {text: 'Cadastrar produtos', href: '/products'},
              ].map((step) => (
                <li key={step.text} className="flex items-center gap-3">
                  <div className="w-5 h-5 rounded-full border-2 border-gray-300 shrink-0"/>
                  <Link href={step.href} className="text-sm text-gray-600 hover:text-gray-900 hover:underline">
                    {step.text}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </RootLayout>
  )
}

export default function DashboardPage() {
  return (
    <ProtectedRoute>
      <DashboardContent/>
    </ProtectedRoute>
  )
}
