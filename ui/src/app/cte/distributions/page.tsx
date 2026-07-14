'use client'

import Link from 'next/link'
import {useState, useEffect} from 'react'
import {useMutation, useQuery} from '@tanstack/react-query'
import {toast} from 'sonner'
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
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'
import {Button} from '@/components/ui/button'
import {DebouncedInput} from '@/components/ui/debounced-input'
import {SavedFilterViews} from '@/components/ui/saved-filter-views'
import type {NFeDistributionOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency} from '@/lib/utils/helpers'
import {formatDatetimeBR, formatNsu} from '@/lib/utils/dfe'
import {cteSchemaLabel} from '@/lib/constants/distributions'
import {HomologationBanner} from '@/components/ui/homologation-banner'

function DistributionRow({item}: { item: NFeDistributionOut }) {
  return (
    <tr className={TABLE_ROW}>
      <td data-label="NSU" className={`${TABLE_CELL} font-mono text-xs text-gray-500`}>
        {formatNsu(item.nsu)}
      </td>
      <td data-label="Tipo" className={TABLE_CELL}>
        <p className="text-sm font-medium text-gray-900">{cteSchemaLabel(item)}</p>
        {item.parse_error && (
          <p className="text-xs text-red-600 mt-0.5">Erro ao processar documento</p>
        )}
      </td>
      <td data-label="Emitente" className={TABLE_CELL}>
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
      <td data-label="Valor" className={`${TABLE_CELL} text-sm text-gray-700`}>
        {item.total ? formatCurrency(item.total) : <span className="text-gray-400">—</span>}
      </td>
      <td data-label="Situação" className={TABLE_CELL}>
        {item.sefaz_motive ? (
          <p className="text-xs text-gray-600 max-w-45 truncate" title={item.sefaz_motive}>
            {item.sefaz_motive}
          </p>
        ) : (
          <span className="text-xs text-gray-400">—</span>
        )}
      </td>
      <td data-label="Recebido em" className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`}>
        {formatDatetimeBR(item.created_at)}
      </td>
      <td className={`${TABLE_CELL} text-right`}>
        {item.access_key && (
          <Link
            href={`/cte/detail?key=${item.access_key}`}
            className="text-xs font-medium text-gray-500 hover:text-gray-700"
          >
            Ver CT-e
          </Link>
        )}
      </td>
    </tr>
  )
}

function CTeDistributionsContent() {
  const {selectedOrg} = useAuth()
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)
  
  const {data: config} = useQuery({
    queryKey: queryKeys.cteConfig(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getCTeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })
  
  const [nsuFilter, setNsuFilter] = useState('')
  const nsuQuery = nsuFilter.trim() || undefined

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} = usePagination<NFeDistributionOut>({
    queryKey: [...queryKeys.distributions.history('cte', selectedOrg?.pk), {nsu: nsuQuery}],
    queryFn: (cursor) => apiClient.listDistributions('cte', {limit: 10, cursor, nsu: nsuQuery}),
    enabled: !!selectedOrg,
  })

  // Changing the NSU filter is a new result set — restart from the first page.
  useEffect(() => {
    reset()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nsuQuery])

  const syncMutation = useMutation({
    mutationFn: () => apiClient.syncDistributions('cte'),
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
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">Distribuição CT-e</h1>
            <p className="text-sm text-gray-500 mt-0.5">
              Conhecimentos de Transporte recebidos como tomador via SEFAZ
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
            <SavedFilterViews pageId="cte-distributions" currentNsu={nsuFilter} onApply={setNsuFilter}/>
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
          ariaLabel="Distribuição de CT-e"
          minWidth={560}
          headers={['NSU', 'Tipo', 'Emitente', 'Valor', 'Situação', 'Recebido em', {label: '', align: 'right'}]}
        >
          {isLoading ? (
            <tr>
              <td colSpan={7} className={TABLE_CELL}>
                <DistributionSkeleton/>
              </td>
            </tr>
          ) : items.length === 0 ? (
            <tr>
              <td colSpan={7} className={TABLE_CELL}>
                <EmptyState
                  title={nsuFilter ? 'Nenhum resultado para o NSU informado' : 'Nenhuma distribuição encontrada'}
                  description={nsuFilter
                    ? 'Ajuste o número NSU ou limpe o filtro.'
                    : 'Clique em «Consultar SEFAZ» para buscar CT-es emitidos para o seu CNPJ.'}
                />
              </td>
            </tr>
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

export default function CTeDistributionsPage() {
  return (
    <ProtectedRoute>
      <CTeDistributionsContent/>
    </ProtectedRoute>
  )
}
