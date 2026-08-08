'use client'

import {useState} from 'react'
import {toast} from 'sonner'
import {useMutation, useQuery} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {EmptyState} from '@/components/ui/empty-state'
import {ServiceIcon} from '@/components/ui/icon'
import {Pagination} from '@/components/ui/pagination'
import {Button} from '@/components/ui/button'
import {PenaltyBanner} from '@/components/ui/penalty-banner'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import type {NfseDistributionOut} from '@/lib/types/api'
import {formatDatetimeBR, formatNsu, triggerDownload} from '@/lib/utils/dfe'
import {EVENT_LABELS} from '@/lib/schemas/nfse'

interface NfseDistributionTabProps {
  docType: 'nfe' | 'cte' | 'mdfe' | 'nfse'
  orgPk: string
}

function DistributionRow({item, docType}: { item: NfseDistributionOut; docType: NfseDistributionTabProps['docType'] }) {
  const [xmlLoading, setXmlLoading] = useState(false)

  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      triggerDownload(await apiClient.downloadDistributionXml(docType, item.nsu), `NSU_${formatNsu(item.nsu)}.xml`)
    } catch {
      toast.error('Não foi possível baixar o XML.')
    } finally {
      setXmlLoading(false)
    }
  }

  return (
    <tr className={TABLE_ROW}>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="NSU">{formatNsu(item.nsu)}</td>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="Chave de acesso">
        {item.access_key ?? '—'}
      </td>
      <td className={`${TABLE_CELL} text-sm text-gray-700`} data-label="Tipo">{item.schema_type}</td>
      <td className={`${TABLE_CELL} text-sm text-gray-700`} data-label="Evento">
        {item.event_type ? (EVENT_LABELS[item.event_type] ?? item.event_type) : '—'}
      </td>
      <td className={`${TABLE_CELL} whitespace-nowrap text-xs text-gray-500`} data-label="Recebido em">
        {formatDatetimeBR(item.created_at)}
      </td>
      <td className={`${TABLE_CELL} text-right`} data-label="Ações">
        {item.xml_s3_key && (
          <Button variant="ghost" size="xs" onClick={handleDownloadXml} disabled={xmlLoading}
                  className="text-brand-700">
            {xmlLoading ? 'Baixando…' : 'XML'}
          </Button>
        )}
      </td>
    </tr>
  )
}

export function NfseDistributionTab({docType, orgPk}: NfseDistributionTabProps) {
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)
  const {data: config} = useQuery({
    queryKey: queryKeys.nfseConfig(orgPk),
    queryFn: () => apiClient.getNfseConfig(orgPk),
    enabled: !!orgPk,
  })
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NfseDistributionOut>({
    queryKey: queryKeys.distributions.history(docType, orgPk),
    queryFn: (cursor) => apiClient.listDistributions<NfseDistributionOut>(docType, {limit: 10, cursor}),
    enabled: !!orgPk,
  })

  const syncMutation = useMutation({
    mutationFn: () => apiClient.syncDistributions(docType),
    onSuccess: () => {
      setPenaltyMessage(null)
      toast.info('Consulta ao ADN enfileirada. Novos documentos aparecerão automaticamente.')
    },
    onError: (error: unknown) => {
      if (error instanceof ApiError && error.status === 429) setPenaltyMessage(error.detail)
      else toast.error(error instanceof ApiError ? error.detail : 'Não foi possível consultar o ADN agora.')
    },
  })

  const isProd = config?.environment === 1
  const nsu = config ? (isProd ? config.prod_nsu : config.hom_nsu) : null
  const lastAt = config ? (isProd ? config.prod_last_dist_nsu_at : config.hom_last_dist_nsu_at) : null
  const nextAt = lastAt ? new Date(new Date(lastAt).getTime() + 60 * 60 * 1000) : null

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-sm text-gray-500">NFS-e recebidas pela organização via Ambiente de Dados Nacional (ADN).</p>
          {config && (
            <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-xs text-gray-500">
              <span className="font-mono">Último NSU: {nsu != null ? formatNsu(nsu) : '—'}</span>
              {lastAt && <span>Última consulta: {formatDatetimeBR(lastAt)}</span>}
              {nextAt && <span>Próxima consulta disponível: {formatDatetimeBR(nextAt.toISOString())}</span>}
            </div>
          )}
        </div>
        <Button variant="outline" size="sm" onClick={() => syncMutation.mutate()}
                disabled={syncMutation.isPending} className="border-brand-200 text-brand-700 hover:bg-brand-50">
          {syncMutation.isPending ? 'Enfileirando…' : 'Consultar ADN'}
        </Button>
      </div>

      {penaltyMessage && <PenaltyBanner message={penaltyMessage} onDismiss={() => setPenaltyMessage(null)}/>}
      <TableShell
        ariaLabel="Distribuição de NFS-e"
        minWidth={480}
        dimmed={isFetching}
        headers={['NSU', 'Chave de acesso', 'Tipo', 'Evento', 'Recebido em', {label: '', align: 'right'}]}
      >
        {isLoading ? (
          <tr><td className={TABLE_CELL} colSpan={6}><DistributionSkeleton/></td></tr>
        ) : items.length === 0 ? (
          <tr><td className={TABLE_CELL} colSpan={6}>
            <EmptyState
              title="Nenhum documento recebido"
              description="A distribuição roda periodicamente. As NFS-e recebidas aparecerão aqui."
              icon={<ServiceIcon width={20} height={20}/>}
            />
          </td></tr>
        ) : items.map((item) => <DistributionRow key={item.nsu} item={item} docType={docType}/>) }
      </TableShell>

      {(hasNext || hasPrevious) && (
        <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                    isLoading={isFetching}/>
      )}
    </div>
  )
}
