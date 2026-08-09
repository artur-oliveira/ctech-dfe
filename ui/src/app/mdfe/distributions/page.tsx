'use client'

import {useState, useEffect} from 'react'
import {useMutation} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {MdfeIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PenaltyBanner} from '@/components/ui/penalty-banner'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TableShell, TABLE_CELL} from '@/components/ui/table-shell'
import {DebouncedInput} from '@/components/ui/debounced-input'
import {SavedFilterViews} from '@/components/ui/saved-filter-views'
import type {NFeDistributionOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatDatetimeBR, formatNsu} from '@/lib/utils/dfe'
import {mdfeSchemaLabel} from '@/lib/constants/distributions'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {ConfigRequiredBanner} from '@/components/ui/config-required-banner'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'

function DistributionRow({item}: { item: NFeDistributionOut }) {
  return (
    <tr className="hover:bg-gray-50 transition-colors">
      <td data-label="NSU" className="px-4 py-3 font-mono text-xs text-gray-500">
        {formatNsu(item.nsu)}
      </td>
      <td data-label="Tipo" className="px-4 py-3">
        <p className="text-sm font-medium text-gray-900">{mdfeSchemaLabel(item)}</p>
        {item.parse_error && (
          <p className="text-xs text-red-600 mt-0.5">Erro ao processar documento</p>
        )}
      </td>
      <td data-label="Emitente" className="px-4 py-3">
        {item.emit_name ? (
          <>
            <p className="text-sm text-gray-900">{item.emit_name}</p>
            {item.emit_cpf_cnpj && (
              <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(item.emit_cpf_cnpj)}</p>
            )}
          </>
        ) : (
          <span className="text-xs text-gray-400">—</span>
        )}
      </td>
      <td data-label="Situação" className="px-4 py-3">
        {item.sefaz_motive ? (
          <p className="text-xs text-gray-600 max-w-45 truncate" title={item.sefaz_motive}>
            {item.sefaz_motive}
          </p>
        ) : (
          <span className="text-xs text-gray-400">—</span>
        )}
      </td>
      <td data-label="Recebido em" className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">
        {formatDatetimeBR(item.created_at)}
      </td>
    </tr>
  )
}

function MDFeDistributionsContent() {
  const {selectedOrg} = useAuth()
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)
  
  const {config, isMissing: configMissing} = useFiscalConfig('mdfe', selectedOrg?.pk)
  
  const [nsuFilter, setNsuFilter] = useState('')
  const nsuQuery = nsuFilter.trim() || undefined

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} = usePagination<NFeDistributionOut>({
    queryKey: [...queryKeys.distributions.history('mdfe', selectedOrg?.pk), {nsu: nsuQuery}],
    queryFn: (cursor) => apiClient.listDistributions('mdfe', {limit: 10, cursor, nsu: nsuQuery}),
    enabled: !!selectedOrg,
  })

  // Changing the NSU filter is a new result set — restart from the first page.
  useEffect(() => {
    reset()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nsuQuery])

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
  
  if (!selectedOrg) {
    return (
      <RootLayout>
        <div className="p-4 md:p-8">
          <NoOrgBanner/>
        </div>
      </RootLayout>
    )
  }
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8 space-y-6">
        <HomologationBanner environment={config?.environment}/>
        <ConfigRequiredBanner show={configMissing} variant="mdfe" docLabel="MDF-e"/>
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">Distribuição MDF-e</h1>
            <p className="text-sm text-gray-500 mt-0.5">
              Manifestos de Documentos Fiscais recebidos via SEFAZ
            </p>
            {config && (
              <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-xs text-gray-400">
                <span className="font-mono">Último NSU: {nsu != null ? formatNsu(nsu) : '—'}</span>
                {lastAt && <span>Última consulta: {formatDatetimeBR(lastAt)}</span>}
                {nextAt && <span>Próxima estimada: {formatDatetimeBR(nextAt.toISOString())}</span>}
              </div>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <DebouncedInput
              value={nsuFilter}
              onChange={setNsuFilter}
              placeholder="Filtrar por NSU"
              inputMode="numeric"
              className="h-8 w-44 text-xs"
            />
            <SavedFilterViews pageId="mdfe-distributions" currentNsu={nsuFilter} onApply={setNsuFilter}/>
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
        </div>
        
        {penaltyMessage && (
          <PenaltyBanner message={penaltyMessage} onDismiss={() => setPenaltyMessage(null)}/>
        )}
        
        <TableShell
          ariaLabel="Distribuição de MDF-e"
          minWidth={480}
          headers={['NSU', 'Tipo', 'Emitente', 'Situação', 'Recebido em']}
        >
          {isLoading ? (
            <tr><td className={TABLE_CELL} colSpan={5}><DistributionSkeleton/></td></tr>
          ) : items.length === 0 ? (
            <tr><td className={TABLE_CELL} colSpan={5}>
              <EmptyState
                title={nsuFilter ? 'Nenhum resultado para o NSU informado' : 'Nenhuma distribuição encontrada'}
                description={nsuFilter
                  ? 'Ajuste o número NSU ou limpe o filtro.'
                  : 'Clique em «Consultar SEFAZ» para buscar MDF-es emitidos para o seu CNPJ.'}
                icon={<MdfeIcon width={20} height={20}/>}
              />
            </td></tr>
          ) : (
            items.map(item => (
              <DistributionRow key={item.nsu} item={item}/>
            ))
          )}
        </TableShell>
        
        {(hasNext || hasPrevious) && (
          <Pagination
            hasNext={hasNext}
            hasPrevious={hasPrevious}
            onNext={goNext}
            onPrevious={goPrevious}
            isLoading={isFetching}
          />
        )}
      </div>
    </RootLayout>
  )
}

export default function MDFeDistributionsPage() {
  return (
    <ProtectedRoute>
      <MDFeDistributionsContent/>
    </ProtectedRoute>
  )
}
