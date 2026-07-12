'use client'

import {Suspense, useState} from 'react'
import {useMutation, useQuery} from '@tanstack/react-query'
import {toast} from 'sonner'
import Link from 'next/link'
import {apiClient, ApiError} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PenaltyBanner} from '@/components/ui/penalty-banner'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import type {MdfeListOut, NFeDistributionOut} from '@/lib/types/api'
import {formatCurrency, formatDate} from '@/lib/utils/helpers'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {formatDatetimeBR, formatNsu, triggerDownload} from '@/lib/utils/dfe'
import {mdfeSchemaLabel} from '@/lib/constants/distributions'
import {MdfeStatusCell} from '@/components/mdfe/MdfeStatusBadge'
import {useMdfeActions} from '@/components/mdfe/MdfeActions'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'

type Tab = 'emitidos' | 'recebidos' | 'distribuicao'

// ─── emitted MDF-e listing ───────────────────────────────────────────────────

function MdfeList({orgPk, onCancel, onClose}: {
  orgPk: string
  onCancel: (m: MdfeListOut) => void
  onClose: (m: MdfeListOut) => void
}) {
  const queryParams = {sort: 'desc' as const}
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<MdfeListOut>({
    queryKey: queryKeys.mdfes.list(orgPk, queryParams),
    queryFn: (cursor) => apiClient.getMdfes({...queryParams, cursor}),
    enabled: true,
  })

  if (isLoading) {
    return (
      <div className="space-y-2">
        {[...Array(4)].map((_, i) => <div key={i} className="h-12 bg-gray-100 rounded-lg animate-pulse"/>)}
      </div>
    )
  }
  if (items.length === 0) {
    return (
      <EmptyState title="Nenhum MDF-e emitido"
                  description="Emita o primeiro Manifesto Eletrônico de Documentos Fiscais da organização."/>
    )
  }

  return (
    <>
      <div
        className={`bg-white rounded-xl border border-gray-200 overflow-hidden overflow-x-auto transition-opacity ${isFetching ? 'opacity-60' : ''}`}>
        <table className="w-full text-sm min-w-150">
          <thead className="bg-gray-50 border-b border-gray-200">
          <tr>
            {['Nº / Série', 'Trajeto', 'Carga', 'Status', 'Emissão', ''].map((h) => (
              <th key={h}
                  className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
          {items.map((mdfe) => (
            <tr key={mdfe.sk} className="hover:bg-gray-50 transition-colors">
              <td className="px-5 py-3.5">
                <span className="font-mono font-medium text-gray-900">{mdfe.number}</span>
                <span className="text-gray-400 text-xs ml-1">/ {mdfe.serie}</span>
              </td>
              <td className="px-5 py-3.5 text-gray-700 whitespace-nowrap">
                <span className="font-medium">{mdfe.uf_start}</span>
                <span className="text-gray-400 mx-1.5">→</span>
                <span className="font-medium">{mdfe.uf_end}</span>
              </td>
              <td className="px-5 py-3.5 text-gray-700 whitespace-nowrap">{formatCurrency(mdfe.cargo_value)}</td>
              <td className="px-5 py-3.5">
                <MdfeStatusCell status={mdfe.status} sefazMotive={mdfe.sefaz_motive}/>
              </td>
              <td className="px-5 py-3.5 text-gray-500 whitespace-nowrap text-xs">
                {mdfe.dh_emi ? new Date(mdfe.dh_emi).toLocaleDateString('pt-BR') : formatDate(mdfe.year, mdfe.month, mdfe.day)}
              </td>
              <td className="px-5 py-3.5 text-right">
                <div className="flex items-center justify-end gap-3">
                  <Link href={`/mdfe/detail?key=${mdfe.sk}`}
                        className="text-xs font-medium text-brand-600 hover:text-brand-700">
                    Detalhes
                  </Link>
                  {(mdfe.status === 'authorized' || mdfe.status === 'closed' || mdfe.status === 'cancelled') && (
                    <DownloadPdfButton fetchPdf={() => apiClient.downloadMdfeDamdfe(mdfe.sk)}
                                       filename={mdfe.sk} label="DAMDFE"/>
                  )}
                  {mdfe.status === 'authorized' && (
                    <>
                      <Button variant="ghost" size="xs" onClick={() => onClose(mdfe)}
                              className="text-blue-600 hover:text-blue-700">Encerrar</Button>
                      <Button variant="ghost" size="xs" onClick={() => onCancel(mdfe)}
                              className="text-red-500 hover:text-red-700">Cancelar</Button>
                    </>
                  )}
                </div>
              </td>
            </tr>
          ))}
          </tbody>
        </table>
      </div>
      <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                  isLoading={isFetching}/>
    </>
  )
}

// ─── received MDF-e (distribution) ───────────────────────────────────────────

function MDFeRow({item}: { item: NFeDistributionOut }) {
  const [xmlLoading, setXmlLoading] = useState(false)

  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      const blob = await apiClient.downloadDistributionXml('mdfe', item.nsu)
      triggerDownload(blob, `NSU_${formatNsu(item.nsu)}.xml`)
    } catch {
      toast.error('Erro ao baixar XML.')
    } finally {
      setXmlLoading(false)
    }
  }

  return (
    <tr className="hover:bg-gray-50 transition-colors">
      <td className="px-4 py-3 font-mono text-xs text-gray-500">{formatNsu(item.nsu)}</td>
      <td className="px-4 py-3">
        <p className="text-sm font-medium text-gray-900">{mdfeSchemaLabel(item)}</p>
        {item.parse_error && <p className="text-xs text-red-500 mt-0.5">Erro ao processar documento</p>}
      </td>
      <td className="px-4 py-3 font-mono text-xs text-gray-400">
        {item.access_key ?? <span className="text-gray-300">—</span>}
      </td>
      <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">{formatDatetimeBR(item.created_at)}</td>
      <td className="px-4 py-3 text-right">
        {item.xml_s3_key && (
          <Button variant="ghost" size="xs" onClick={handleDownloadXml} disabled={xmlLoading}
                  className="text-brand-600 hover:text-brand-700">
            {xmlLoading ? 'Baixando…' : 'XML'}
          </Button>
        )}
      </td>
    </tr>
  )
}

function MDFeDistributionList({orgPk, showSync}: { orgPk: string; showSync: boolean }) {
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)

  const {data: config} = useQuery({
    queryKey: queryKeys.mdfeConfig(orgPk),
    queryFn: () => apiClient.getMDFeConfig(orgPk),
    enabled: !!orgPk,
  })

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NFeDistributionOut>({
    queryKey: queryKeys.distributions.history('mdfe', orgPk),
    queryFn: (cursor) => apiClient.listDistributions('mdfe', {limit: 8, cursor}),
    enabled: true,
  })

  const syncMutation = useMutation({
    mutationFn: () => apiClient.syncDistributions('mdfe'),
    onSuccess: () => {
      setPenaltyMessage(null)
      toast.info('Consulta enfileirada. Novos documentos aparecerão automaticamente.')
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError && err.status === 429) {
        setPenaltyMessage(err.detail)
      } else {
        toast.error(err instanceof Error ? err.message : 'Erro ao enfileirar consulta.')
      }
    },
  })

  const isProd = config?.environment === 1
  const nsu = config ? (isProd ? config.prod_nsu : config.hom_nsu) : null
  const lastAt = config ? (isProd ? config.prod_last_dist_nsu_at : config.hom_last_dist_nsu_at) : null
  const nextAt = lastAt ? new Date(new Date(lastAt).getTime() + 30 * 60 * 1000) : null

  return (
    <div className="space-y-4">
      {showSync && (
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div>
            <p className="text-sm text-gray-500">MDF-es recebidos via distribuição SEFAZ</p>
            {config && (
              <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-xs text-gray-400">
                <span className="font-mono">Último NSU: {nsu != null ? formatNsu(nsu) : '—'}</span>
                {lastAt && <span>Última consulta: {formatDatetimeBR(lastAt)}</span>}
                {nextAt && <span>Próxima estimada: {formatDatetimeBR(nextAt.toISOString())}</span>}
              </div>
            )}
          </div>
          <Button variant="outline" size="sm" onClick={() => syncMutation.mutate()}
                  disabled={syncMutation.isPending}
                  className="text-brand-600 border-brand-200 hover:bg-brand-50">
            {syncMutation.isPending ? 'Enfileirando…' : 'Consultar SEFAZ'}
          </Button>
        </div>
      )}

      {penaltyMessage && <PenaltyBanner message={penaltyMessage} onDismiss={() => setPenaltyMessage(null)}/>}

      <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
        {isLoading ? (
          <DistributionSkeleton/>
        ) : items.length === 0 ? (
          <EmptyState title="Nenhum MDF-e recebido"
                      description="Clique em «Consultar SEFAZ» para buscar MDF-es emitidos para o seu CNPJ."/>
        ) : (
          <table className="w-full text-sm min-w-120">
            <thead className="bg-gray-50 border-b border-gray-100">
            <tr>
              {['NSU', 'Tipo', 'Chave', 'Recebido em', ''].map(h => (
                <th key={h} className="px-4 py-2.5 text-left text-xs font-semibold text-gray-500">{h}</th>
              ))}
            </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
            {items.map(item => <MDFeRow key={item.nsu} item={item}/>)}
            </tbody>
          </table>
        )}
      </div>

      {(hasNext || hasPrevious) && (
        <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                    isLoading={isFetching}/>
      )}
    </div>
  )
}

// ─── page ────────────────────────────────────────────────────────────────────

function MDFeContent() {
  const {selectedOrg} = useAuth()
  const [activeTab, setActiveTab] = useState<Tab>('emitidos')

  const {data: mdfeConfig} = useQuery({
    queryKey: queryKeys.mdfeConfig(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getMDFeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  const {openCancel, openClose, modals} = useMdfeActions(selectedOrg?.pk)

  const tabs: { key: Tab; label: string }[] = [
    {key: 'emitidos', label: 'Emitidos'},
    {key: 'recebidos', label: 'Recebidos'},
    {key: 'distribuicao', label: 'Importação/Distribuição'},
  ]

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center justify-between mb-4 gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">MDF-e</h1>
            <p className="text-gray-500 text-sm mt-0.5">Manifesto Eletrônico de Documentos Fiscais</p>
          </div>
          {selectedOrg && activeTab === 'emitidos' && (
            <a href="/mdfe/emit"
               className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-white text-sm font-medium shrink-0"
               style={{backgroundColor: 'var(--brand-600)'}}>
              <span className="text-base leading-none">+</span>
              Emitir MDF-e
            </a>
          )}
        </div>

        <HomologationBanner environment={mdfeConfig?.environment}/>

        <div className="flex overflow-x-auto border-b border-gray-200 mb-6">
          {tabs.map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`relative shrink-0 px-4 py-2.5 text-sm font-medium transition-colors ${
                activeTab === tab.key
                  ? "text-brand-700 after:absolute after:bottom-0 after:inset-x-0 after:h-0.5 after:bg-brand-600 after:content-['']"
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : activeTab === 'emitidos' ? (
          <MdfeList orgPk={selectedOrg.pk} onCancel={openCancel} onClose={openClose}/>
        ) : activeTab === 'recebidos' ? (
          <MDFeDistributionList key="mdfe-recebidos" orgPk={selectedOrg.pk} showSync={false}/>
        ) : (
          <MDFeDistributionList key="mdfe-distribuicao" orgPk={selectedOrg.pk} showSync={true}/>
        )}
      </div>

      {modals}
    </RootLayout>
  )
}

export default function MDFePage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <MDFeContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
