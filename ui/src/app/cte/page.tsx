'use client'

import Link from 'next/link'
import {useState} from 'react'
import {useMutation} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ComingSoon} from '@/components/ui/coming-soon'
import {EmptyState} from '@/components/ui/empty-state'
import {CteIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PenaltyBanner} from '@/components/ui/penalty-banner'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import type {NFeDistributionOut} from '@/lib/types/api'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {ConfigRequiredBanner} from '@/components/ui/config-required-banner'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {formatDatetimeBR, formatNsu, triggerRemoteDownload} from '@/lib/utils/dfe'
import {cteSchemaLabel} from '@/lib/constants/distributions'
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'

type Tab = 'emitidos' | 'recebidos' | 'distribuicao'

function CTeRow({item}: { item: NFeDistributionOut }) {
  const [xmlLoading, setXmlLoading] = useState(false)
  
  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
		const download = await apiClient.downloadDistributionXml('cte', item.nsu)
		triggerRemoteDownload(download.url)
    } catch {
      toast.error('Erro ao baixar XML.')
    } finally {
      setXmlLoading(false)
    }
  }
  
  return (
    <tr className={TABLE_ROW}>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="NSU">
        {formatNsu(item.nsu)}
      </td>
      <td className={TABLE_CELL} data-label="Tipo">
        <p className="text-sm font-medium text-gray-900">{cteSchemaLabel(item)}</p>
        {item.parse_error && <p className="text-xs text-red-600 mt-0.5">Erro ao processar documento</p>}
      </td>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-400`} data-label="Chave">
        {item.access_key ?? <span className="text-gray-300">—</span>}
      </td>
      <td className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`} data-label="Recebido em">
        {formatDatetimeBR(item.created_at)}
      </td>
      <td className={`${TABLE_CELL} text-right`}>
        <div className="flex items-center justify-end gap-3">
          {item.xml_s3_key && (
            <Button variant="ghost" size="xs" onClick={handleDownloadXml} disabled={xmlLoading}
                    className="text-brand-600 hover:text-brand-700">
              {xmlLoading ? 'Baixando…' : 'XML'}
            </Button>
          )}
          {item.access_key && (
            <Link href={`/cte/detail?key=${item.access_key}`}
                  className="text-xs font-medium text-gray-500 hover:text-gray-700">
              Ver CT-e
            </Link>
          )}
        </div>
      </td>
    </tr>
  )
}

function CTeDistributionList({orgPk, showSync}: { orgPk: string; showSync: boolean }) {
  const [penaltyMessage, setPenaltyMessage] = useState<string | null>(null)
  
  const {config} = useFiscalConfig('cte', orgPk)
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NFeDistributionOut>({
    queryKey: queryKeys.distributions.history('cte', orgPk),
    queryFn: (cursor) => apiClient.listDistributions('cte', {limit: 8, cursor}),
    enabled: true,
  })
  
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
  
  return (
    <div className="space-y-4">
      {showSync && (
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div>
            <p className="text-sm text-gray-500">Conhecimentos de Transporte recebidos via distribuição
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
      )}
      
      {penaltyMessage && (
        <PenaltyBanner message={penaltyMessage} onDismiss={() => setPenaltyMessage(null)}/>
      )}
      
      {isLoading ? (
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
          <DistributionSkeleton/>
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
          <EmptyState title="Nenhum CT-e recebido" icon={<CteIcon width={20} height={20}/>}
                      description="Clique em «Consultar SEFAZ» para buscar CT-es emitidos para o seu CNPJ."/>
        </div>
      ) : (
        <TableShell
          ariaLabel="CT-es"
          minWidth={120}
          headers={['NSU', 'Tipo', 'Chave', 'Recebido em', {label: '', align: 'right'}]}
        >
          {items.map(item => <CTeRow key={item.nsu} item={item}/>)}
        </TableShell>
      )}
      
      {(hasNext || hasPrevious) && (
        <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                    isLoading={isFetching}/>
      )}
    </div>
  )
}

function CTeContent() {
  const {selectedOrg} = useAuth()
  const [activeTab, setActiveTab] = useState<Tab>('recebidos')
  
  const {config: cteConfig, isMissing: cteConfigMissing} = useFiscalConfig('cte', selectedOrg?.pk)
  
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
            <h1 className="text-2xl font-semibold text-gray-900">CT-e</h1>
            <p className="text-gray-500 text-sm mt-0.5">Conhecimento de Transporte Eletrônico</p>
          </div>
        </div>
        
        <HomologationBanner environment={cteConfig?.environment}/>
        <ConfigRequiredBanner show={cteConfigMissing} variant="cte" docLabel="CT-e"/>
        
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
          <ComingSoon title="Emissão de CT-e em breve"/>
        ) : activeTab === 'recebidos' ? (
          <CTeDistributionList key="cte-recebidos" orgPk={selectedOrg.pk} showSync={false}/>
        ) : (
          <CTeDistributionList key="cte-distribuicao" orgPk={selectedOrg.pk} showSync={true}/>
        )}
      </div>
    </RootLayout>
  )
}

export default function CTePage() {
  return (
    <ProtectedRoute>
      <CTeContent/>
    </ProtectedRoute>
  )
}
