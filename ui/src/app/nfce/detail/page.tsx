'use client'

import {Suspense, useState} from 'react'
import Link from 'next/link'
import {useSearchParams} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {Button} from '@/components/ui/button'
import {DfeDetail} from '@/components/dfe/DfeDetail'
import {SubstituteModal} from '@/components/nfce/SubstituteModal'

// ─── NFC-e detail (shared DfeDetail + the NFC-e-specific "Substituir" action) ───

function NfceDetail({accessKey}: { accessKey: string }) {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const [showSubstitute, setShowSubstitute] = useState(false)

  const invalidate = () => {
    void qc.invalidateQueries({queryKey: queryKeys.nfces.detail(accessKey)})
    void qc.invalidateQueries({queryKey: queryKeys.nfces.lists(selectedOrg?.pk)})
  }

  const substituteMutation = useMutation({
    mutationFn: ({substituteKey, justification}: { substituteKey: string; justification: string }) =>
      apiClient.substituteNfce(accessKey, substituteKey, justification),
    onSuccess: () => {
      setShowSubstitute(false)
      invalidate()
      toast.success('Substituição enviada à SEFAZ.')
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao substituir NFC-e.'),
  })

  return (
    <DfeDetail
      accessKey={accessKey}
      docLabel="NFC-e"
      destLabel="Consumidor"
      enabled={!!accessKey && !!selectedOrg}
      detailQueryKey={queryKeys.nfces.detail(accessKey)}
      eventsQueryKey={queryKeys.nfces.events(accessKey)}
      listQueryKey={queryKeys.nfces.lists(selectedOrg?.pk)}
      fetchDoc={() => apiClient.getNfce(accessKey)}
      fetchEvents={() => apiClient.getNfceEvents(accessKey)}
      cancelFn={(j) => apiClient.cancelNfce(accessKey, j)}
      downloadXml={() => apiClient.downloadNfceXml(accessKey)}
      downloadDanfe={() => apiClient.downloadNfceDanfe(accessKey)}
      downloadEventXml={(sk) => apiClient.downloadNfceEventXml(accessKey, sk)}
      headerActions={(doc) =>
        doc.status === 'authorized' && doc.incoming === 0 ? (
          <Button variant="outline" size="sm" onClick={() => setShowSubstitute(true)}
                  className="text-gray-600 border-gray-300 hover:bg-gray-50">
            Substituir
          </Button>
        ) : null
      }
      renderExtra={(doc) =>
        showSubstitute ? (
          <SubstituteModal
            target={doc}
            loading={substituteMutation.isPending}
            onClose={() => setShowSubstitute(false)}
            onConfirm={(substituteKey, justification) => substituteMutation.mutate({substituteKey, justification})}
          />
        ) : null
      }
    />
  )
}

// ─── Page wrapper ─────────────────────────────────────────────────────────────

function NfceDetailContent() {
  const params = useSearchParams()
  const accessKey = params.get('key') ?? ''

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/nfce" className="hover:text-brand-600">NFC-e</Link>
          <span>/</span>
          <span className="text-gray-600 font-mono truncate max-w-[200px]">{accessKey || 'Detalhe'}</span>
        </div>
        {accessKey ? (
          <NfceDetail accessKey={accessKey}/>
        ) : (
          <p className="text-sm text-gray-500">Chave de acesso não informada.</p>
        )}
      </div>
    </RootLayout>
  )
}

export default function NfceDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfceDetailContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
