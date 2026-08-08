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
import {NfceIcon} from '@/components/ui/icon'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Modal} from '@/components/ui/modal'
import {JustificationField} from '@/components/ui/justification-field'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {OptionsSelect} from '@/components/ui/options-select'
import {NumericInput} from '@/components/ui/numeric-input'
import {Button} from '@/components/ui/button'
import type {NfeListOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency, formatDate} from '@/lib/utils/helpers'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {DfeStatusCell} from '@/components/dfe/DfeStatusBadge'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'
import {SubstituteModal} from '@/components/nfce/SubstituteModal'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
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
          <label htmlFor="nfce-filter-year" className="text-xs font-medium text-gray-600">Ano</label>
          <OptionsSelect id="nfce-filter-year" value={filterYear}
                         onValueChange={(v) => setParam({year: v, month: ''})}
                         options={[{value: '', label: 'Todos'}, ...YEARS.map(y => ({
                           value: String(y),
                           label: String(y)
                         }))]}
                         placeholder="Todos" className="h-8 w-20 text-sm"/>
        </div>
        {filterYear && (
          <div className="flex flex-col gap-1">
            <label htmlFor="nfce-filter-month" className="text-xs font-medium text-gray-600">Mês</label>
            <OptionsSelect id="nfce-filter-month" value={filterMonth}
                           onValueChange={(v) => setParam({month: v})}
                           options={[{value: '', label: 'Todos'}, ...MONTHS.map(m => ({
                             value: String(m.value),
                             label: m.label
                           }))]}
                           placeholder="Todos" className="h-8 w-18 text-sm"/>
          </div>
        )}
        <div className="flex flex-col gap-1">
          <label htmlFor="nfce-filter-number" className="text-xs font-medium text-gray-600">Número</label>
          <NumericInput id="nfce-filter-number" value={numberSearch} onChange={(v) => setParam({number: v})}
                        placeholder="Ex: 42"
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
        <LoadingSkeleton/>
      ) : items.length === 0 ? (
        <EmptyState title="Nenhuma NFC-e emitida" icon={<NfceIcon width={20} height={20}/>}
                    description="Emita a primeira Nota Fiscal de Consumidor Eletrônica da organização."/>
      ) : (
        <TableShell
          ariaLabel="NFC-es"
          minWidth={150}
          dimmed={isFetching}
          headers={[
            'Nº / Série',
            'Consumidor',
            'Total',
            'Status',
            'Emissão',
            {label: '', align: 'right'},
          ]}
        >
          {items.map((nfce) => (
            <tr key={nfce.sk} className={TABLE_ROW}>
              <td className={TABLE_CELL} data-label="Nº / Série">
                <span className="font-mono font-medium text-gray-900">{nfce.number}</span>
                <span className="text-gray-400 text-xs ml-1">/ {nfce.serie}</span>
              </td>
              <td className={TABLE_CELL} data-label="Consumidor">
                {nfce.dest_cpf_cnpj ? (
                  <>
                    <p className="font-medium text-gray-900 truncate max-w-50">{nfce.dest_name || 'Consumidor'}</p>
                    <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(nfce.dest_cpf_cnpj)}</p>
                  </>
                ) : (
                  <span className="text-gray-400">Consumidor não identificado</span>
                )}
              </td>
              <td className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`}
                  data-label="Total">{formatCurrency(nfce.total)}</td>
              <td className={TABLE_CELL} data-label="Status">
                <DfeStatusCell status={nfce.status} sefazMotive={nfce.sefaz_motive}/>
              </td>
              <td className={`${TABLE_CELL} text-gray-500 whitespace-nowrap text-xs`}
                  data-label="Emissão">
                {nfce.dh_emi ? new Date(nfce.dh_emi).toLocaleDateString('pt-BR') : formatDate(nfce.year, nfce.month, nfce.day)}
              </td>
              <td className={`${TABLE_CELL} text-right`}>
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
                              className="text-danger hover:text-red-700">Cancelar</Button>
                    </>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </TableShell>
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
            <Button variant="brand" render={<Link href="/nfce/emit"/>}>
              <span className="text-base leading-none">+</span>
              Emitir NFC-e
            </Button>
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
          <JustificationField
            id="nfce-cancel-justification"
            value={justification}
            onChange={setJustification}
            minLength={CANCEL_JUSTIFICATION_MIN_LENGTH}
            maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
            placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
          />
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
