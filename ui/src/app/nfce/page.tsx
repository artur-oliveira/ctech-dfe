'use client'

import {Suspense, useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {useRouter, useSearchParams} from 'next/navigation'
import Link from 'next/link'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {Modal} from '@/components/ui/modal'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {OptionsSelect} from '@/components/ui/options-select'
import {NumericInput} from '@/components/ui/numeric-input'
import {Button} from '@/components/ui/button'
import type {NfeListOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency, formatDate} from '@/lib/utils/helpers'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {NfeStatusCell} from '@/components/nfe/NfeStatusBadge'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'
import {SubstituteModal} from '@/components/nfce/SubstituteModal'
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status'

const CANCEL_JUSTIFICATION_MIN_LENGTH = 15
const CANCEL_JUSTIFICATION_MAX_LENGTH = 255
const CURRENT_YEAR = new Date().getFullYear()
const YEARS = Array.from({length: 5}, (_, i) => CURRENT_YEAR - i)
const MONTHS = [
  {value: 1, label: 'Jan'}, {value: 2, label: 'Fev'}, {value: 3, label: 'Mar'},
  {value: 4, label: 'Abr'}, {value: 5, label: 'Mai'}, {value: 6, label: 'Jun'},
  {value: 7, label: 'Jul'}, {value: 8, label: 'Ago'}, {value: 9, label: 'Set'},
  {value: 10, label: 'Out'}, {value: 11, label: 'Nov'}, {value: 12, label: 'Dez'},
]

// ─── list ───────────────────────────────────────────────────────────────────────

function NfceList({orgPk, onCancel, onSubstitute}: {
  orgPk: string
  onCancel: (n: NfeListOut) => void
  onSubstitute: (n: NfeListOut) => void
}) {
  const router = useRouter()
  const params = useSearchParams()
  const filterYear = params.get('year') ?? ''
  const filterMonth = params.get('month') ?? ''
  const numberSearch = params.get('number') ?? ''
  
  const setParam = (next: Record<string, string>) => {
    const sp = new URLSearchParams()
    const merged = {year: filterYear, month: filterMonth, number: numberSearch, ...next}
    Object.entries(merged).forEach(([k, v]) => {
      if (v) sp.set(k, v)
    })
    router.replace(`/nfce?${sp.toString()}`, {scroll: false})
  }
  
  const queryParams = {
    sort: 'desc' as const,
    incoming: 0 as const,
    limit: 8,
    ...(numberSearch ? {number: parseInt(numberSearch, 10)} : {}),
    ...(filterYear ? {year: parseInt(filterYear, 10)} : {}),
    ...(filterMonth ? {month: parseInt(filterMonth, 10)} : {}),
  }
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} = usePagination<NfeListOut>({
    queryKey: queryKeys.nfces.list(orgPk, queryParams),
    queryFn: (cursor) => apiClient.listNfces({...queryParams, cursor}),
    enabled: true,
  })
  
  const hasFilters = numberSearch || filterYear || filterMonth
  
  return (
    <>
      <form onSubmit={(e) => e.preventDefault()} className="flex items-start gap-3 mb-5 flex-wrap">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-600">Ano</label>
          <OptionsSelect value={filterYear} onValueChange={(v) => setParam({year: v, month: ''})}
                         options={[{value: '', label: 'Todos'}, ...YEARS.map(y => ({
                           value: String(y),
                           label: String(y)
                         }))]}
                         placeholder="Todos" className="h-8 w-20 text-sm"/>
        </div>
        {filterYear && (
          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Mês</label>
            <OptionsSelect value={filterMonth} onValueChange={(v) => setParam({month: v})}
                           options={[{value: '', label: 'Todos'}, ...MONTHS.map(m => ({
                             value: String(m.value),
                             label: m.label
                           }))]}
                           placeholder="Todos" className="h-8 w-18 text-sm"/>
          </div>
        )}
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-600">Número</label>
          <NumericInput value={numberSearch} onChange={(v) => setParam({number: v})} placeholder="Ex: 42"
                        integerPlaces={9} debounceMs={300} className="w-28"/>
        </div>
        {hasFilters && (
          <Button type="button" variant="outline" size="sm" className="self-end text-gray-500"
                  onClick={() => {
                    reset()
                    router.replace('/nfce', {scroll: false})
                  }}>Limpar</Button>
        )}
      </form>
      
      {isLoading ? (
        <div className="space-y-2">
          {[...Array(4)].map((_, i) => <div key={i} className="h-12 bg-gray-100 rounded-lg animate-pulse"/>)}
        </div>
      ) : items.length === 0 ? (
        <EmptyState title="Nenhuma NFC-e emitida"
                    description="Emita a primeira Nota Fiscal de Consumidor Eletrônica da organização."/>
      ) : (
        <div
          className={`bg-white rounded-xl border border-gray-200 overflow-hidden overflow-x-auto transition-opacity ${isFetching ? 'opacity-60' : ''}`}>
          <table className="w-full text-sm min-w-150">
            <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              {['Nº / Série', 'Consumidor', 'Total', 'Status', 'Emissão', ''].map((h) => (
                <th key={h}
                    className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">{h}</th>
              ))}
            </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
            {items.map((nfce) => (
              <tr key={nfce.sk} className="hover:bg-gray-50 transition-colors">
                <td className="px-5 py-3.5">
                  <span className="font-mono font-medium text-gray-900">{nfce.number}</span>
                  <span className="text-gray-400 text-xs ml-1">/ {nfce.serie}</span>
                </td>
                <td className="px-5 py-3.5">
                  {nfce.dest_cpf_cnpj ? (
                    <>
                      <p className="font-medium text-gray-900 truncate max-w-50">{nfce.dest_name || 'Consumidor'}</p>
                      <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(nfce.dest_cpf_cnpj)}</p>
                    </>
                  ) : (
                    <span className="text-gray-400">Consumidor não identificado</span>
                  )}
                </td>
                <td className="px-5 py-3.5 text-gray-700 whitespace-nowrap">{formatCurrency(nfce.total)}</td>
                <td className="px-5 py-3.5">
                  <NfeStatusCell status={nfce.status} sefazMotive={nfce.sefaz_motive}/>
                </td>
                <td className="px-5 py-3.5 text-gray-500 whitespace-nowrap text-xs">
                  {nfce.dh_emi ? new Date(nfce.dh_emi).toLocaleDateString('pt-BR') : formatDate(nfce.year, nfce.month, nfce.day)}
                </td>
                <td className="px-5 py-3.5 text-right">
                  <div className="flex items-center justify-end gap-3">
                    <Link href={`/nfce/detail?key=${nfce.sk}`}
                          className="text-xs font-medium text-brand-600 hover:text-brand-700">
                      Detalhes
                    </Link>
                    {(nfce.status === 'authorized' || nfce.status === 'cancelled') && (
                      <DownloadPdfButton
                        fetchPdf={() => apiClient.downloadNfceDanfe(nfce.sk)}
                        filename={nfce.sk} label="DANFE"/>
                    )}
                    {nfce.status === 'authorized' && (
                      <>
                        <Button variant="ghost" size="xs" onClick={() => onSubstitute(nfce)}
                                className="text-gray-500 hover:text-gray-700">Substituir</Button>
                        <Button variant="ghost" size="xs" onClick={() => onCancel(nfce)}
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
      )}
      <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                  isLoading={isFetching}/>
    </>
  )
}

// ─── page ─────────────────────────────────────────────────────────────────────

function NfceContent() {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  
  const {data: nfceConfig} = useQuery({
    queryKey: queryKeys.nfceConfig(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getNFCeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })
  
  const [cancelTarget, setCancelTarget] = useState<NfeListOut | null>(null)
  const [justification, setJustification] = useState('')
  const [substituteTarget, setSubstituteTarget] = useState<NfeListOut | null>(null)
  
  // Optimistically show the transitional "Cancelando" state (GSI is eventually
  // consistent); the WebSocket delivers the final status when the worker finishes.
  const markCancelPending = (accessKey: string) => {
    setDocStatusOptimistic(qc, queryKeys.nfces.lists(selectedOrg?.pk), accessKey, 'cancel_pending')
    void qc.invalidateQueries({queryKey: queryKeys.nfces.detail(accessKey)})
  }
  
  const cancelMutation = useMutation({
    mutationFn: ({accessKey, justification}: { accessKey: string; justification: string }) =>
      apiClient.cancelNfce(accessKey, justification),
    onSuccess: (_data, {accessKey}) => {
      setCancelTarget(null)
      setJustification('')
      markCancelPending(accessKey)
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao cancelar NFC-e.'),
  })
  
  const substituteMutation = useMutation({
    mutationFn: ({accessKey, substituteKey, justification}: {
      accessKey: string;
      substituteKey: string;
      justification: string
    }) => apiClient.substituteNfce(accessKey, substituteKey, justification),
    onSuccess: (_data, {accessKey}) => {
      setSubstituteTarget(null)
      markCancelPending(accessKey)
      toast.success('Substituição enviada à SEFAZ.')
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao substituir NFC-e.'),
  })
  
  const handleConfirmCancel = () => {
    if (!cancelTarget || justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return
    cancelMutation.mutate({accessKey: cancelTarget.sk, justification: justification.trim()})
  }
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center justify-between mb-4 gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">NFC-e</h1>
            <p className="text-gray-500 text-sm mt-0.5">Nota Fiscal de Consumidor Eletrônica</p>
          </div>
          {selectedOrg && (
            <a href="/nfce/emit"
               className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-white text-sm font-medium shrink-0"
               style={{backgroundColor: 'var(--brand-600)'}}>
              <span className="text-base leading-none">+</span>
              Emitir NFC-e
            </a>
          )}
        </div>
        
        <HomologationBanner environment={nfceConfig?.environment}/>
        
        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : (
          <NfceList orgPk={selectedOrg.pk} onCancel={(n) => {
            setJustification('')
            setCancelTarget(n)
          }} onSubstitute={setSubstituteTarget}/>
        )}
      </div>
      
      <Modal
        isOpen={cancelTarget !== null}
        title={cancelTarget ? `Cancelar NFC-e nº ${cancelTarget.number}` : ''}
        onClose={() => {
          setCancelTarget(null)
          setJustification('')
        }}
        onSubmit={handleConfirmCancel}
        submitLabel="Confirmar cancelamento"
        cancelLabel="Voltar"
        danger
        loading={cancelMutation.isPending}
        submitDisabled={justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Esta ação é <span className="font-medium text-red-600">irreversível</span>. A NFC-e será
            cancelada junto à SEFAZ.
          </p>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">Justificativa</label>
            <textarea value={justification} onChange={(e) => setJustification(e.target.value)} rows={4}
                      maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
                      placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
                      className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-red-400 resize-none"/>
          </div>
        </div>
      </Modal>
      
      {substituteTarget && (
        <SubstituteModal
          target={substituteTarget}
          loading={substituteMutation.isPending}
          onClose={() => setSubstituteTarget(null)}
          onConfirm={(substituteKey, just) =>
            substituteMutation.mutate({accessKey: substituteTarget.sk, substituteKey, justification: just})}
        />
      )}
    </RootLayout>
  )
}

export default function NfcePage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfceContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
