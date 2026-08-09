'use client'

import {Suspense, useState} from 'react'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {useSearchParams} from 'next/navigation'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {FiscalConfigForm} from '@/components/fiscal-config/FiscalConfigForm'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

const TABS: { id: DocVariant; label: string; description: string }[] = [
  {id: 'nfe', label: 'NF-e', description: 'Nota Fiscal Eletrônica'},
  {id: 'nfce', label: 'NFC-e', description: 'Nota Fiscal de Consumidor Eletrônica'},
  {id: 'cte', label: 'CT-e', description: 'Conhecimento de Transporte Eletrônico'},
  {id: 'mdfe', label: 'MDF-e', description: 'Manifesto Eletrônico de Documentos Fiscais'},
  {id: 'nfse', label: 'NFS-e', description: 'Nota Fiscal de Serviços Eletrônica'},
]

function isDocVariant(v: string | null): v is DocVariant {
  return TABS.some((t) => t.id === v)
}

function FiscalConfigContent() {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const params = useSearchParams()
  const tabParam = params.get('tab')
  const [activeTab, setActiveTab] = useState<DocVariant>(isDocVariant(tabParam) ? tabParam : 'nfe')

  const pk = selectedOrg?.pk ?? ''

  // Fetch all configs in parallel; the hook treats 404 as null (not configured yet)
  const nfeQuery = useFiscalConfig('nfe', pk)
  const nfceQuery = useFiscalConfig('nfce', pk)
  const cteQuery = useFiscalConfig('cte', pk)
  const mdfeQuery = useFiscalConfig('mdfe', pk)
  const nfseQuery = useFiscalConfig('nfse', pk)

  const nfeMutation = useMutation({
    mutationFn: (d: object) => apiClient.upsertNFeConfig(pk, d),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.nfeConfig(pk)}),
  })
  const nfceMutation = useMutation({
    mutationFn: (d: object) => apiClient.upsertNFCeConfig(pk, d),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.nfceConfig(pk)}),
  })
  const cteMutation = useMutation({
    mutationFn: (d: object) => apiClient.upsertCTeConfig(pk, d),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.cteConfig(pk)}),
  })
  const mdfeMutation = useMutation({
    mutationFn: (d: object) => apiClient.upsertMDFeConfig(pk, d),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.mdfeConfig(pk)}),
  })
  const nfseMutation = useMutation({
    mutationFn: (d: object) => apiClient.upsertNfseConfig(pk, d),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.nfseConfig(pk)}),
  })

  const configByTab = {
    nfe: {query: nfeQuery, mutation: nfeMutation},
    nfce: {query: nfceQuery, mutation: nfceMutation},
    cte: {query: cteQuery, mutation: cteMutation},
    mdfe: {query: mdfeQuery, mutation: mdfeMutation},
    nfse: {query: nfseQuery, mutation: nfseMutation},
  }
  
  const active = configByTab[activeTab]
  const queryError = active.query.error?.message
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-2xl font-semibold text-gray-900">Configuração Fiscal</h1>
          <p className="mt-1 text-sm text-gray-500">
            Ambiente, numeração e parâmetros de emissão por tipo de documento
          </p>
        </div>
        
        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : (
          <div className="mx-auto max-w-2xl">
            {/* Tabs */}
            <div className="mb-6 flex gap-1 rounded-xl bg-gray-100 p-1">
              {TABS.map((tab) => {
                const isActive = tab.id === activeTab
                const {query} = configByTab[tab.id]
                const hasConfig = !!query.config
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id)}
                    className={`relative flex flex-1 flex-col items-center rounded-lg px-3 py-2 text-sm font-medium transition-all ${
                      isActive
                        ? 'bg-white text-gray-900 shadow-card'
                        : 'text-gray-500 hover:text-gray-700'
                    }`}
                  >
                    <span>{tab.label}</span>
                    {!query.isPending && (
                      <span
                        className={`mt-0.5 h-1.5 w-1.5 rounded-full ${
                          hasConfig ? 'bg-green-500' : 'bg-gray-300'
                        }`}
                      />
                    )}
                  </button>
                )
              })}
            </div>
            
            {/* Tab description */}
            <p className="mb-6 text-sm text-muted-foreground">
              {TABS.find((t) => t.id === activeTab)?.description}
            </p>
            
            {/* Error from fetch */}
            {queryError && (
              <div
                className="mb-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                {queryError}
              </div>
            )}
            
            {/* Loading skeleton */}
            {active.query.isPending && (
              <div className="space-y-3">
                {['w-80', 'w-60', 'w-100', 'w-60'].map((w, i) => (
                  <div key={i} className={`h-8 ${w} rounded-md bg-gray-100 animate-pulse`}/>
                ))}
              </div>
            )}
            
            {/* Form */}
            {!active.query.isPending && (
              <div className="rounded-xl border border-gray-200 bg-white p-6">
                <FiscalConfigForm
                  key={activeTab}
                  variant={activeTab}
                  initialData={active.query.config ?? null}
                  onSave={async (data) => {
                    await active.mutation.mutateAsync(data)
                  }}
                  loading={active.mutation.isPending}
                />
              </div>
            )}
          </div>
        )}
      </div>
    </RootLayout>
  )
}

export default function FiscalConfigPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <FiscalConfigContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
