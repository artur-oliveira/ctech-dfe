'use client'

import {Suspense} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {useRouter, useSearchParams} from 'next/navigation'
import Link from 'next/link'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {ServiceIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {OptionsSelect} from '@/components/ui/options-select'
import {NumericInput} from '@/components/ui/numeric-input'
import {Button} from '@/components/ui/button'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'
import type {NfseListOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency} from '@/lib/utils/helpers'
import {formatDatetimeBR, triggerDownload} from '@/lib/utils/dfe'
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {DfeStatusCell} from '@/components/dfe/DfeStatusBadge'
import {dfeStatusOptions, NFSE_STATUSES} from '@/lib/data/dfe_status'
import {CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal'
import {NfseCancelModal} from '@/components/nfse/NfseCancelModal'
import {useState} from 'react'
import {toast} from 'sonner'

const MONTHS = [
  {value: 1, label: 'Jan'}, {value: 2, label: 'Fev'}, {value: 3, label: 'Mar'},
  {value: 4, label: 'Abr'}, {value: 5, label: 'Mai'}, {value: 6, label: 'Jun'},
  {value: 7, label: 'Jul'}, {value: 8, label: 'Ago'}, {value: 9, label: 'Set'},
  {value: 10, label: 'Out'}, {value: 11, label: 'Nov'}, {value: 12, label: 'Dez'},
]

const CURRENT_YEAR = new Date().getFullYear()
const YEARS = Array.from({length: 5}, (_, i) => CURRENT_YEAR - i)

const STATUS_OPTIONS = [
  {value: '', label: 'Todos'},
  ...dfeStatusOptions(NFSE_STATUSES),
]

function NfsesContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const params = useSearchParams()
  const qc = useQueryClient()

  const [cancelTarget, setCancelTarget] = useState<NfseListOut | null>(null)
  const [xmlLoading, setXmlLoading] = useState<string | null>(null)

  const orgPk = selectedOrg?.pk ?? ''

  const {data: nfseConfig} = useQuery({
    queryKey: queryKeys.nfseConfig(orgPk),
    queryFn: () => apiClient.getNfseConfig(orgPk),
    enabled: !!orgPk,
  })

  const filterYear = params.get('year') ?? ''
  const filterMonth = params.get('month') ?? ''
  const numberSearch = params.get('number') ?? ''
  const filterStatus = params.get('status') ?? ''

  const setFilter = (key: string, value: string) => {
    const sp = new URLSearchParams(params.toString())
    if (value) sp.set(key, value); else sp.delete(key)
    if (key === 'year' && !value) sp.delete('month')
    router.replace(`/nfse?${sp.toString()}`, {scroll: false})
  }

  const hasFilters = numberSearch || filterYear || filterMonth || filterStatus

  const queryParams = {
    sort: 'desc' as const,
    limit: 10,
    ...(numberSearch ? {number: parseInt(numberSearch, 10)} : {}),
    ...(filterYear ? {year: parseInt(filterYear, 10)} : {}),
    ...(filterMonth ? {month: parseInt(filterMonth, 10)} : {}),
    ...(filterStatus ? {status: filterStatus} : {}),
  }

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NfseListOut>({
    queryKey: queryKeys.nfses.list(orgPk, queryParams),
    queryFn: (cursor) => apiClient.getNfses({...queryParams, cursor}),
    enabled: !!orgPk,
  })

  const cancelMutation = useMutation({
    mutationFn: ({id, reasonCode, reasonDescription}: { id: string; reasonCode: string; reasonDescription: string }) =>
      apiClient.cancelNfse(id, reasonCode, reasonDescription),
    onSuccess: (_data, {id}) => {
      setCancelTarget(null)
      setDocStatusOptimistic(qc, queryKeys.nfses.lists(orgPk), id, 'cancelled')
      void qc.invalidateQueries({queryKey: queryKeys.nfses.detail(id)})
    },
  })

  const handleDownloadXml = async (item: NfseListOut) => {
    setXmlLoading(item.sk)
    try {
      triggerDownload(await apiClient.downloadNfseXml(item.sk), `${item.sk}.xml`)
    } catch {
      toast.error('Erro ao baixar XML.')
    } finally {
      setXmlLoading(null)
    }
  }

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center justify-between mb-4 gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">NFS-e</h1>
            <p className="text-gray-500 text-sm mt-0.5">Nota Fiscal de Serviços Eletrônica</p>
          </div>
          {selectedOrg && (
            <Button variant="brand" render={<Link href="/nfse/emit"/>}>
              <span className="text-base leading-none">+</span>
              Emitir NFS-e
            </Button>
          )}
        </div>

        <HomologationBanner environment={nfseConfig?.environment}/>

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : (
          <>
            <form onSubmit={(e) => e.preventDefault()} className="flex items-start gap-3 mb-5 flex-wrap">
              <div className="flex flex-col gap-1">
                <label htmlFor="nfse-filter-year" className="text-xs font-medium text-gray-600">Ano</label>
                <OptionsSelect
                  id="nfse-filter-year"
                  value={filterYear}
                  onValueChange={(v) => setFilter('year', v)}
                  options={[{value: '', label: 'Todos'}, ...YEARS.map(y => ({value: String(y), label: String(y)}))]}
                  placeholder="Todos"
                  className="h-8 w-20 text-sm"
                />
              </div>

              {filterYear && (
                <div className="flex flex-col gap-1">
                  <label htmlFor="nfse-filter-month" className="text-xs font-medium text-gray-600">Mês</label>
                  <OptionsSelect
                    id="nfse-filter-month"
                    value={filterMonth}
                    onValueChange={(v) => setFilter('month', v)}
                    options={[{value: '', label: 'Todos'}, ...MONTHS.map(m => ({value: String(m.value), label: m.label}))]}
                    placeholder="Todos"
                    className="h-8 w-18 text-sm"
                  />
                </div>
              )}

              <div className="flex flex-col gap-1">
                <label htmlFor="nfse-filter-number" className="text-xs font-medium text-gray-600">Número</label>
                <NumericInput
                  id="nfse-filter-number"
                  value={numberSearch}
                  onChange={(v) => setFilter('number', v)}
                  placeholder="Ex: 42"
                  integerPlaces={9}
                  debounceMs={300}
                  className="w-28"
                />
              </div>

              <div className="flex flex-col gap-1">
                <label htmlFor="nfse-filter-status" className="text-xs font-medium text-gray-600">Status</label>
                <OptionsSelect
                  id="nfse-filter-status"
                  value={filterStatus}
                  onValueChange={(v) => setFilter('status', v)}
                  options={STATUS_OPTIONS}
                  placeholder="Todos"
                  className="h-8 w-36 text-sm"
                />
              </div>

              {hasFilters && (
                <Button type="button" variant="outline" size="sm" onClick={() => router.replace('/nfse', {scroll: false})}
                        className="self-end text-gray-500 hover:text-gray-700">
                  Limpar
                </Button>
              )}
            </form>

            {isLoading ? (
              <LoadingSkeleton/>
            ) : items.length === 0 ? (
              <EmptyState title="Nenhuma NFS-e emitida" description="Emita a primeira Nota Fiscal de Serviços da organização."
                          icon={<ServiceIcon width={20} height={20}/>}/>
            ) : (
              <TableShell
                ariaLabel="NFS-es emitidas"
                minWidth={150}
                dimmed={isFetching}
                headers={['Nº / Série', 'Competência', 'Tomador', 'Valor', 'Status', 'Emitida em', {label: '', align: 'right'}]}
              >
                {items.map((nfse) => (
                  <tr key={nfse.sk} className={TABLE_ROW}>
                    <td className={TABLE_CELL} data-label="Nº / Série">
                      <span className="font-mono font-medium text-gray-900">{nfse.number}</span>
                      <span className="text-gray-400 text-xs ml-1">/ {nfse.serie}</span>
                    </td>
                    <td className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`} data-label="Competência">
                      {nfse.competence}
                    </td>
                    <td className={TABLE_CELL} data-label="Tomador">
                      {nfse.dest_name ? (
                        <>
                          <p className="font-medium text-gray-900 truncate max-w-50">{nfse.dest_name}</p>
                          {nfse.dest_cpf_cnpj && <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(nfse.dest_cpf_cnpj)}</p>}
                        </>
                      ) : (
                        <span className="text-gray-300">—</span>
                      )}
                    </td>
                    <td className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`} data-label="Valor">{formatCurrency(nfse.total)}</td>
                    <td className={TABLE_CELL} data-label="Status">
                      <DfeStatusCell status={nfse.status} sefazMotive={nfse.sefaz_motive}/>
                    </td>
                    <td className={`${TABLE_CELL} text-gray-500 whitespace-nowrap text-xs`} data-label="Emitida em">
                      {formatDatetimeBR(nfse.created_at)}
                    </td>
                    <td className={`${TABLE_CELL} text-right`}>
                      <div className="flex items-center justify-end gap-3">
                        <Link href={`/nfse/detail?id=${encodeURIComponent(nfse.sk)}`}
                              className="flex items-center min-h-11 sm:min-h-0 text-xs font-medium text-brand-600 hover:text-brand-700">
                          Detalhes
                        </Link>
                        {nfse.xml_s3_key && (
                          <Button variant="ghost" size="xs" onClick={() => handleDownloadXml(nfse)}
                                  disabled={xmlLoading === nfse.sk} className="text-brand-600 hover:text-brand-700">
                            {xmlLoading === nfse.sk ? 'Baixando…' : 'XML'}
                          </Button>
                        )}
                        {nfse.status === 'authorized' && nfse.provider === 'nacional' && (
                          <DownloadPdfButton fetchPdf={() => apiClient.downloadDanfse(nfse.sk)} filename={nfse.sk} label="DANFSE"/>
                        )}
                        {nfse.status === 'authorized' && (
                          <Button variant="ghost" size="default" onClick={() => setCancelTarget(nfse)}
                                  className="text-xs text-danger hover:text-red-700">
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
        )}
      </div>

      <NfseCancelModal
        isOpen={cancelTarget !== null}
        docNumber={cancelTarget?.number ?? ''}
        loading={cancelMutation.isPending}
        error={cancelMutation.error}
        onClose={() => setCancelTarget(null)}
        onConfirm={({reasonCode, reasonDescription}) => {
          if (!cancelTarget) return
          if (reasonDescription.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return
          cancelMutation.mutate({id: cancelTarget.sk, reasonCode, reasonDescription: reasonDescription.trim()})
        }}
      />
    </RootLayout>
  )
}

export default function NfsesPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfsesContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
