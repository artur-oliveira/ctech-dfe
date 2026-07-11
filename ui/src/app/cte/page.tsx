'use client'

import Link from 'next/link'
import {useState} from 'react'
import {useMutation, useQuery} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ComingSoon} from '@/components/ui/coming-soon'
import {EmptyState} from '@/components/ui/empty-state'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PenaltyBanner} from '@/components/ui/penalty-banner'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import type {NFeDistributionOut} from '@/lib/types/api'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {formatDatetimeBR, formatNsu, triggerDownload} from '@/lib/utils/dfe'
import {cteSchemaLabel} from '@/lib/constants/distributions'

type Tab = 'emitidos' | 'recebidos' | 'distribuicao'

function CTeRow({item}: { item: NFeDistributionOut }) {
    const [xmlLoading, setXmlLoading] = useState(false)

    const handleDownloadXml = async () => {
        setXmlLoading(true)
        try {
            const blob = await apiClient.downloadDistributionXml('cte', item.nsu)
            triggerDownload(blob, `NSU_${formatNsu(item.nsu)}.xml`)
        } catch {
            toast.error('Erro ao baixar XML.')
        } finally {
            setXmlLoading(false)
        }
    }

    return (
        <tr className="hover:bg-gray-50 transition-colors">
            <td className="px-4 py-3 font-mono text-xs text-gray-500">
                {formatNsu(item.nsu)}
            </td>
            <td className="px-4 py-3">
                <p className="text-sm font-medium text-gray-900">{cteSchemaLabel(item)}</p>
                {item.parse_error && <p className="text-xs text-red-500 mt-0.5">Erro ao processar documento</p>}
            </td>
            <td className="px-4 py-3 font-mono text-xs text-gray-400">
                {item.access_key ?? <span className="text-gray-300">—</span>}
            </td>
            <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">
                {formatDatetimeBR(item.created_at)}
            </td>
            <td className="px-4 py-3 text-right">
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

    const {data: config} = useQuery({
        queryKey: queryKeys.cteConfig(orgPk),
        queryFn: () => apiClient.getCTeConfig(orgPk),
        enabled: !!orgPk,
    })

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

            <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
                {isLoading ? (
                    <DistributionSkeleton/>
                ) : items.length === 0 ? (
                    <EmptyState title="Nenhum CT-e recebido"
                                description="Clique em «Consultar SEFAZ» para buscar CT-es emitidos para o seu CNPJ."/>
                ) : (
                    <table className="w-full text-sm min-w-120">
                        <thead className="bg-gray-50 border-b border-gray-100">
                        <tr>
                            {['NSU', 'Tipo', 'Chave', 'Recebido em', ''].map(h => (
                                <th key={h}
                                    className="px-4 py-2.5 text-left text-xs font-semibold text-gray-500">{h}</th>
                            ))}
                        </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                        {items.map(item => <CTeRow key={item.nsu} item={item}/>)}
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

function CTeContent() {
    const {selectedOrg} = useAuth()
    const [activeTab, setActiveTab] = useState<Tab>('recebidos')

    const {data: cteConfig} = useQuery({
        queryKey: queryKeys.cteConfig(selectedOrg?.pk ?? ''),
        queryFn: () => apiClient.getCTeConfig(selectedOrg!.pk),
        enabled: !!selectedOrg,
    })

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
