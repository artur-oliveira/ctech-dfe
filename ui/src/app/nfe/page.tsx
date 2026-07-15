'use client'

import {Suspense, useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {useRouter, useSearchParams} from 'next/navigation'
import Link from 'next/link'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
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
import type {NFeDistributionOut, NfeListOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency, formatDate} from '@/lib/utils/helpers'
import {formatDatetimeBR, formatNsu, parseAccessKey, triggerDownload} from '@/lib/utils/dfe'
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {PenaltyBanner} from '@/components/ui/penalty-banner'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'
import {NfeStatusCell} from '@/components/nfe/NfeStatusBadge'
import {EVENT_TYPE_LABELS} from "@/lib/data/dfe_event";

type Tab = 'emitidas' | 'recebidas' | 'transportadas' | 'distribuicao'

const LIST_TABS: { key: Tab; label: string; incoming: 0 | 1 | 2; emptyLabel: string; emptyDesc: string }[] = [
  {
    key: 'emitidas',
    label: 'Emitidas',
    incoming: 0,
    emptyLabel: 'Nenhuma NF-e emitida',
    emptyDesc: 'Emita a primeira Nota Fiscal Eletrônica da organização.'
  },
  {
    key: 'recebidas',
    label: 'Recebidas',
    incoming: 1,
    emptyLabel: 'Nenhuma NF-e recebida',
    emptyDesc: 'NF-es recebidas aparecerão aqui após a distribuição SEFAZ.'
  },
  {
    key: 'transportadas',
    label: 'Transportadas',
    incoming: 2,
    emptyLabel: 'Nenhuma NF-e transportada',
    emptyDesc: 'NF-es nas quais a organização consta como transportadora aparecerão aqui.'
  },
]

const ALL_TAB_LABELS: { key: Tab; label: string }[] = [
  ...LIST_TABS,
  {key: 'distribuicao', label: 'Importação/Distribuição'},
]

const DIST_SCHEMA_LABELS: Record<string, string> = {
  resNFe: 'Resumo NF-e',
  procNFe: 'NF-e Completa',
  resEvento: 'Resumo Evento',
  procEventoNFe: 'Evento',
}

const MONTHS = [
  {value: 1, label: 'Jan'}, {value: 2, label: 'Fev'}, {value: 3, label: 'Mar'},
  {value: 4, label: 'Abr'}, {value: 5, label: 'Mai'}, {value: 6, label: 'Jun'},
  {value: 7, label: 'Jul'}, {value: 8, label: 'Ago'}, {value: 9, label: 'Set'},
  {value: 10, label: 'Out'}, {value: 11, label: 'Nov'}, {value: 12, label: 'Dez'},
]

const CANCEL_JUSTIFICATION_MIN_LENGTH = 15
const CANCEL_JUSTIFICATION_MAX_LENGTH = 255
const CURRENT_YEAR = new Date().getFullYear()
const YEARS = Array.from({length: 5}, (_, i) => CURRENT_YEAR - i)

function distSchemaLabel(item: NFeDistributionOut): string {
  if (item.schema_type && DIST_SCHEMA_LABELS[item.schema_type]) {
    if (item.schema_type === 'resEvento' || item.schema_type === 'procEventoNFe') {
      const evtLabel = item.event_type ? (EVENT_TYPE_LABELS[item.event_type] ?? item.event_type) : ''
      return evtLabel ? `${DIST_SCHEMA_LABELS[item.schema_type]} — ${evtLabel}` : DIST_SCHEMA_LABELS[item.schema_type]
    }
    return DIST_SCHEMA_LABELS[item.schema_type]
  }
  return item.doc_schema
}

function DistributionRow({item, docType}: { item: NFeDistributionOut; docType: string }) {
  const [xmlLoading, setXmlLoading] = useState(false)
  const isFullNfe = item.schema_type === 'procNFe'
  
  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      const blob = await apiClient.downloadDistributionXml(docType, item.nsu)
      triggerDownload(blob, `NSU_${formatNsu(item.nsu)}.xml`)
    } catch {
      toast.error('Erro ao baixar XML.')
    } finally {
      setXmlLoading(false)
    }
  }
  const composition = item.access_key ? parseAccessKey(item.access_key) : null;
  return (
    <tr className={TABLE_ROW}>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="NSU">
        {formatNsu(item.nsu)}
      </td>
      <td className={TABLE_CELL} data-label="Tipo">
        <p className="text-sm font-medium text-gray-900">{distSchemaLabel(item)}</p>
        {item.parse_error && <p className="text-xs text-red-600 mt-0.5">Erro ao processar documento</p>}
      </td>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-400`} data-label="NF-e">
        {composition ? composition.number + ' / ' + composition.serie :
          <span className="text-gray-300">—</span>}
      </td>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-400`} data-label="Chave">
        {composition?.formatted ?? <span className="text-gray-300">—</span>}
      </td>
      <td className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`} data-label="Recebido em">
        {formatDatetimeBR(item.created_at)}
      </td>
      <td className={`${TABLE_CELL} text-right`}>
        <div className="flex items-center justify-end gap-3">
          {item.xml_s3_key && (
            <Button
              variant="ghost"
              size="xs"
              onClick={handleDownloadXml}
              disabled={xmlLoading}
              className="text-brand-600 hover:text-brand-700"
            >
              {xmlLoading ? 'Baixando…' : 'XML'}
            </Button>
          )}
          {item.access_key && (
            <Link
              href={`/nfe/detail?key=${item.access_key}&tab=distribuicao`}
              className={`text-xs font-medium ${isFullNfe ? 'text-brand-600 hover:text-brand-700' : 'text-gray-500 hover:text-gray-700'}`}
            >
              Ver NF-e
            </Link>
          )}
        </div>
      </td>
    </tr>
  )
}

function NfeDistributionTab({orgPk}: { orgPk: string }) {
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)
  
  const {data: config} = useQuery({
    queryKey: queryKeys.nfeConfig(orgPk),
    queryFn: () => apiClient.getNFeConfig(orgPk),
    enabled: !!orgPk,
  })
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NFeDistributionOut>({
    queryKey: queryKeys.distributions.history('nfe', orgPk),
    queryFn: (cursor) => apiClient.listDistributions('nfe', {limit: 10, cursor}),
    enabled: true,
  })
  
  const syncMutation = useMutation({
    mutationFn: () => apiClient.syncDistributions('nfe'),
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
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div>
          <p className="text-sm text-gray-500">Documentos fiscais recebidos como destinatário/transportador
            via
            SEFAZ</p>
          {config && (
            <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-xs text-gray-400">
              <span className="font-mono">Último NSU: {nsu != null ? formatNsu(nsu) : '—'}</span>
              {lastAt && <span>Última consulta: {formatDatetimeBR(lastAt)}</span>}
              {nextAt && <span>Próxima estimada: {formatDatetimeBR(nextAt.toISOString())}</span>}
            </div>
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => syncMutation.mutate()}
          disabled={syncMutation.isPending}
          className="text-brand-600 border-brand-200 hover:bg-brand-50"
        >
          {syncMutation.isPending ? 'Enfileirando…' : 'Consultar SEFAZ'}
        </Button>
      </div>
      
      {penaltyMessage && (
        <PenaltyBanner message={penaltyMessage} onDismiss={() => setPenaltyMessage(null)}/>
      )}
      
      <TableShell
        ariaLabel="NF-es emitidas"
        minWidth={140}
        headers={['NSU', 'Tipo', 'NF-e', 'Chave', 'Recebido em', {label: '', align: 'right'}]}
      >
        {isLoading ? (
          <tr>
            <td colSpan={6} className={TABLE_CELL}>
              <DistributionSkeleton/>
            </td>
          </tr>
        ) : items.length === 0 ? (
          <tr>
            <td colSpan={6} className={TABLE_CELL}>
              <EmptyState title="Nenhuma distribuição encontrada"
                          description="Clique em «Consultar SEFAZ» para buscar NF-es emitidas para o seu CNPJ."/>
            </td>
          </tr>
        ) : (
          items.map(item => <DistributionRow key={item.nsu} item={item} docType="nfe"/>)
        )}
      </TableShell>
      
      {(hasNext || hasPrevious) && (
        <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                    isLoading={isFetching}/>
      )}
    </div>
  )
}

function NfeListTab({
                      tab,
                      orgPk,
                      onCancelRequest,
                    }: {
  tab: typeof LIST_TABS[number]
  orgPk: string
  onCancelRequest: (nfe: NfeListOut) => void
}) {
  const router = useRouter()
  const params = useSearchParams()
  
  const filterYear = params.get('year') ?? ''
  const filterMonth = params.get('month') ?? ''
  const filterDay = params.get('day') ?? ''
  const numberSearch = params.get('number') ?? ''
  const activeTab = params.get('tab') ?? 'emitidas'
  
  const setFilterYear = (v: string) => {
    const sp = new URLSearchParams()
    sp.set('tab', activeTab)
    if (v) sp.set('year', v)
    router.replace(`/nfe?${sp.toString()}`, {scroll: false})
  }
  
  const setFilterMonth = (v: string) => {
    const sp = new URLSearchParams()
    sp.set('tab', activeTab)
    if (filterYear) sp.set('year', filterYear)
    if (v) sp.set('month', v)
    if (numberSearch) sp.set('number', numberSearch)
    router.replace(`/nfe?${sp.toString()}`, {scroll: false})
  }
  
  const setFilterDay = (v: string) => {
    const sp = new URLSearchParams()
    sp.set('tab', activeTab)
    if (filterYear) sp.set('year', filterYear)
    if (filterMonth) sp.set('month', filterMonth)
    if (v) sp.set('day', v)
    if (numberSearch) sp.set('number', numberSearch)
    router.replace(`/nfe?${sp.toString()}`, {scroll: false})
  }
  
  const setNumberSearch = (v: string) => {
    const sp = new URLSearchParams()
    sp.set('tab', activeTab)
    if (filterYear) sp.set('year', filterYear)
    if (filterMonth) sp.set('month', filterMonth)
    if (filterDay) sp.set('day', filterDay)
    if (v) sp.set('number', v)
    router.replace(`/nfe?${sp.toString()}`, {scroll: false})
  }
  
  const queryParams = {
    sort: 'desc' as const,
    limit: 8,
    incoming: tab.incoming,
    ...(numberSearch ? {number: parseInt(numberSearch, 10)} : {}),
    ...(filterYear ? {year: parseInt(filterYear, 10)} : {}),
    ...(filterMonth ? {month: parseInt(filterMonth, 10)} : {}),
    ...(filterDay ? {day: parseInt(filterDay, 10)} : {}),
  }
  
  const hasFilters = numberSearch || filterYear || filterMonth || filterDay
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} = usePagination<NfeListOut>({
    queryKey: queryKeys.nfes.list(orgPk, queryParams),
    queryFn: (cursor) => apiClient.getNfes({...queryParams, cursor}),
    enabled: true,
  })
  
  const clearFilters = () => {
    reset()
    router.replace(`/nfe?tab=${activeTab}`, {scroll: false})
  }
  
  const detailLink = (accessKey: string) => {
    const sp = new URLSearchParams()
    sp.set('key', accessKey)
    sp.set('tab', activeTab)
    if (filterYear) sp.set('year', filterYear)
    if (filterMonth) sp.set('month', filterMonth)
    if (filterDay) sp.set('day', filterDay)
    if (numberSearch) sp.set('number', numberSearch)
    return `/nfe/detail?${sp.toString()}`
  }
  
  const maxDay = filterYear && filterMonth
    ? new Date(parseInt(filterYear), parseInt(filterMonth), 0).getDate()
    : 31
  const dayOptions = Array.from({length: maxDay}, (_, i) => ({
    value: String(i + 1),
    label: String(i + 1).padStart(2, '0')
  }))
  
  return (
    <>
      <form onSubmit={(e) => e.preventDefault()} className="flex items-start gap-3 mb-5 flex-wrap">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-600">Ano</label>
          <OptionsSelect
            value={filterYear}
            onValueChange={setFilterYear}
            options={[{value: '', label: 'Todos'}, ...YEARS.map(y => ({
              value: String(y),
              label: String(y)
            }))]}
            placeholder="Todos"
            className="h-8 w-20 text-sm"
          />
        </div>
        
        {filterYear && (
          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Mês</label>
            <OptionsSelect
              value={filterMonth}
              onValueChange={setFilterMonth}
              options={[{value: '', label: 'Todos'}, ...MONTHS.map(m => ({
                value: String(m.value),
                label: m.label
              }))]}
              placeholder="Todos"
              className="h-8 w-18 text-sm"
            />
          </div>
        )}
        
        {filterMonth && (
          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Dia</label>
            <OptionsSelect
              value={filterDay}
              onValueChange={setFilterDay}
              options={[{value: '', label: 'Todos'}, ...dayOptions]}
              placeholder="Todos"
              className="h-8 w-16 text-sm"
            />
          </div>
        )}
        
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-600">Número</label>
          <NumericInput
            value={numberSearch}
            onChange={setNumberSearch}
            placeholder="Ex: 42"
            integerPlaces={9}
            debounceMs={300}
            className="w-28"
          />
        </div>
        
        {hasFilters && (
          <Button type="button" variant="outline" size="sm" onClick={clearFilters}
                  className="self-end text-gray-500 hover:text-gray-700">
            Limpar
          </Button>
        )}
      </form>
      
      {isLoading ? (
        <div className="space-y-2">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-12 bg-gray-100 rounded-lg animate-pulse"/>
          ))}
        </div>
      ) : items.length === 0 ? (
        <EmptyState title={tab.emptyLabel} description={tab.emptyDesc}/>
      ) : (
        <TableShell
          ariaLabel="NF-es recebidas"
          minWidth={150}
          dimmed={isFetching}
          headers={[
            'Nº / Série',
            tab.incoming === 0 ? 'Destinatário' : 'Emitente',
            'Total',
            'Status',
            'Emissão',
            {label: '', align: 'right'},
          ]}
        >
          {items.map((nfe) => (
            <tr key={nfe.sk} className={TABLE_ROW}>
              <td className={TABLE_CELL} data-label="Nº / Série">
                <span className="font-mono font-medium text-gray-900">{nfe.number}</span>
                <span className="text-gray-400 text-xs ml-1">/ {nfe.serie}</span>
              </td>
              <td className={TABLE_CELL} data-label={tab.incoming === 0 ? 'Destinatário' : 'Emitente'}>
                {tab.incoming === 0 ? (
                  <>
                    <p className="font-medium text-gray-900 truncate max-w-50">{nfe.dest_name}</p>
                    <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(nfe.dest_cpf_cnpj)}</p>
                  </>
                ) : (
                  <>
                    <p className="font-medium text-gray-900 truncate max-w-50">{nfe.emit_name}</p>
                    <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(nfe.emit_cpf_cnpj)}</p>
                  </>
                )}
              </td>
              <td className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`} data-label="Total">{formatCurrency(nfe.total)}</td>
              <td className={TABLE_CELL} data-label="Status">
                <NfeStatusCell status={nfe.status} sefazMotive={nfe.sefaz_motive}/>
              </td>
              <td className={`${TABLE_CELL} text-gray-500 whitespace-nowrap text-xs`} data-label="Emissão">
                {nfe.dh_emi ? new Date(nfe.dh_emi).toLocaleDateString('pt-BR') : formatDate(nfe.year, nfe.month, nfe.day)}
              </td>
              <td className={`${TABLE_CELL} text-right`}>
                <div className="flex items-center justify-end gap-3">
                  <Link href={detailLink(nfe.sk)}
                        className="text-xs font-medium text-brand-600 hover:text-brand-700">
                    Detalhes
                  </Link>
                  {nfe.status === 'authorized' && tab.incoming === 0 && (
                    <Button variant="ghost" size="xs" onClick={() => onCancelRequest(nfe)}
                            className="text-danger hover:text-red-700">
                      Cancelar
                    </Button>
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

function NfesContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const params = useSearchParams()
  const qc = useQueryClient()
  
  const {data: nfeConfig} = useQuery({
    queryKey: queryKeys.nfeConfig(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getNFeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })
  
  const activeTab = (params.get('tab') as Tab) || 'emitidas'
  
  const [cancelTarget, setCancelTarget] = useState<NfeListOut | null>(null)
  const [justification, setJustification] = useState('')
  
  const setActiveTab = (tab: Tab) => {
    router.replace(`/nfe?tab=${tab}`, {scroll: false})
  }
  
  const cancelMutation = useMutation({
    mutationFn: ({accessKey, justification}: { accessKey: string; justification: string }) =>
      apiClient.cancelNfe(accessKey, justification),
    onSuccess: (_data, {accessKey}) => {
      setCancelTarget(null)
      setJustification('')
      // Optimistically reflect the transitional "Cancelando" state. The list GSI
      // is eventually consistent, so we patch the cache instead of refetching;
      // the WebSocket delivers the final (cancelled) status when the worker finishes.
      setDocStatusOptimistic(qc, queryKeys.nfes.lists(selectedOrg?.pk), accessKey, 'cancel_pending')
      void qc.invalidateQueries({queryKey: queryKeys.nfes.detail(accessKey)})
    },
  })
  
  const openCancelModal = (nfe: NfeListOut) => {
    setJustification('')
    setCancelTarget(nfe)
  }
  
  const handleConfirmCancel = () => {
    if (!cancelTarget || justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return
    cancelMutation.mutate({accessKey: cancelTarget.sk, justification: justification.trim()})
  }
  
  const currentListTab = LIST_TABS.find(t => t.key === activeTab)
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-4 gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">NF-e</h1>
            <p className="text-gray-500 text-sm mt-0.5">Nota Fiscal Eletrônica</p>
          </div>
          {selectedOrg && (
            <Button variant="brand" render={<Link href="/nfe/emit"/>}>
              <span className="text-base leading-none">+</span>
              Emitir NF-e
            </Button>
          )}
        </div>
        
        <HomologationBanner environment={nfeConfig?.environment}/>
        
        {/* Submenu tabs */}
        <div className="flex overflow-x-auto border-b border-gray-200 mb-6">
          {ALL_TAB_LABELS.map(tab => (
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
        ) : activeTab === 'distribuicao' ? (
          <NfeDistributionTab key="nfe-distribution" orgPk={selectedOrg.pk}/>
        ) : currentListTab ? (
          <NfeListTab key={activeTab} tab={currentListTab} orgPk={selectedOrg.pk}
                      onCancelRequest={openCancelModal}/>
        ) : null}
      </div>
      
      <Modal
        isOpen={cancelTarget !== null}
        title={cancelTarget ? `Cancelar NF-e nº ${cancelTarget.number}` : ''}
        onClose={() => {
          setCancelTarget(null);
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
            Esta ação é <span className="font-medium text-red-600">irreversível</span>. A NF-e será
            cancelada junto à
            SEFAZ e não poderá ser reativada.
          </p>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">Justificativa</label>
            <textarea
              value={justification}
              onChange={(e) => setJustification(e.target.value)}
              rows={4}
              maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
              placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-red-400 resize-none"
            />
            <div className="flex justify-between mt-1">
              {justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH && justification.length > 0 && (
                <p className="text-xs text-red-600">
                  Mínimo {CANCEL_JUSTIFICATION_MIN_LENGTH} caracteres
                  ({CANCEL_JUSTIFICATION_MIN_LENGTH - justification.trim().length} restantes)
                </p>
              )}
              <p className="text-xs text-gray-400 ml-auto">{justification.length}/{CANCEL_JUSTIFICATION_MAX_LENGTH}</p>
            </div>
          </div>
        </div>
      </Modal>
    </RootLayout>
  )
}

export default function NfesPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfesContent/>
      </Suspense>
    </ProtectedRoute>
  )
}

