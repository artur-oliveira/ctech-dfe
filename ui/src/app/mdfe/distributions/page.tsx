'use client'

import {useState} from 'react'
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
import {Button} from '@/components/ui/button'
import type {NFeDistributionOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatDatetimeBR, formatNsu} from '@/lib/utils/dfe'
import {mdfeSchemaLabel} from '@/lib/constants/distributions'
import {HomologationBanner} from '@/components/ui/homologation-banner'

function DistributionRow({item}: { item: NFeDistributionOut }) {
  return (
    <tr className="hover:bg-gray-50 transition-colors">
      <td className="px-4 py-3 font-mono text-xs text-gray-500">
        {formatNsu(item.nsu)}
      </td>
      <td className="px-4 py-3">
        <p className="text-sm font-medium text-gray-900">{mdfeSchemaLabel(item)}</p>
        {item.parse_error && (
          <p className="text-xs text-red-500 mt-0.5">Erro ao processar documento</p>
        )}
      </td>
      <td className="px-4 py-3">
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
      <td className="px-4 py-3">
        {item.sefaz_motive ? (
          <p className="text-xs text-gray-600 max-w-45 truncate" title={item.sefaz_motive}>
            {item.sefaz_motive}
          </p>
        ) : (
          <span className="text-xs text-gray-400">—</span>
        )}
      </td>
      <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">
        {formatDatetimeBR(item.created_at)}
      </td>
    </tr>
  )
}

function MDFeDistributionsContent() {
  const {selectedOrg} = useAuth()
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)
  
  const {data: config} = useQuery({
    queryKey: queryKeys.mdfeConfig(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getMDFeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NFeDistributionOut>({
    queryKey: queryKeys.distributions.history('mdfe', selectedOrg?.pk),
    queryFn: (cursor) => apiClient.listDistributions('mdfe', {limit: 10, cursor}),
    enabled: !!selectedOrg,
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
        
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
          {isLoading ? (
            <DistributionSkeleton/>
          ) : items.length === 0 ? (
            <EmptyState
              title="Nenhuma distribuição encontrada"
              description="Clique em «Consultar SEFAZ» para buscar MDF-es emitidos para o seu CNPJ."
            />
          ) : (
            <table className="w-full text-sm min-w-[480px]">
              <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {['NSU', 'Tipo', 'Emitente', 'Situação', 'Recebido em'].map(h => (
                  <th key={h}
                      className="px-4 py-2.5 text-left text-xs font-semibold text-gray-500">{h}</th>
                ))}
              </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
              {items.map(item => (
                <DistributionRow key={item.nsu} item={item}/>
              ))}
              </tbody>
            </table>
          )}
        </div>
        
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
