'use client'

import {Suspense, useState} from 'react'
import Link from 'next/link'
import {useSearchParams} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {Modal} from '@/components/ui/modal'
import {Button} from '@/components/ui/button'
import {DfeDetail} from '@/components/dfe/DfeDetail'

// ─── NF-e detail (uses shared DfeDetail + the NF-e-specific CC-e action) ────────

function NfeDetail({accessKey}: { accessKey: string }) {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const [showCceModal, setShowCceModal] = useState(false)
  const [cceText, setCceText] = useState('')
  const [cceSeq, setCceSeq] = useState(1)

  const cceMutation = useMutation({
    mutationFn: () => apiClient.sendCorrectionLetter(accessKey, cceText.trim(), cceSeq),
    onSuccess: () => {
      setShowCceModal(false)
      setCceText('')
      setCceSeq(1)
      void qc.invalidateQueries({queryKey: queryKeys.nfes.detail(accessKey)})
      void qc.invalidateQueries({queryKey: queryKeys.nfes.events(accessKey)})
    },
  })

  return (
    <DfeDetail
      accessKey={accessKey}
      docLabel="NF-e"
      destLabel="Destinatário"
      enabled={!!accessKey && !!selectedOrg}
      detailQueryKey={queryKeys.nfes.detail(accessKey)}
      eventsQueryKey={queryKeys.nfes.events(accessKey)}
      listQueryKey={queryKeys.nfes.lists(selectedOrg?.pk)}
      fetchDoc={() => apiClient.getNfe(accessKey)}
      fetchEvents={() => apiClient.getNfeEvents(accessKey)}
      cancelFn={(j) => apiClient.cancelNfe(accessKey, j)}
      downloadXml={() => apiClient.downloadNfeXml(accessKey)}
      downloadEventXml={(sk) => apiClient.downloadNfeEventXml(accessKey, sk)}
      downloadDanfe={() => apiClient.downloadNfeDanfe(accessKey)}
      headerActions={(doc) =>
        doc.status === 'authorized' && doc.incoming === 0 ? (
          <Button variant="outline" size="sm"
                  onClick={() => {
                    setCceText('')
                    setCceSeq(1)
                    setShowCceModal(true)
                  }}
                  className="text-amber-600 border-amber-200 hover:bg-amber-50">
            Carta de Correção
          </Button>
        ) : null
      }
      renderExtra={(doc) => (
        <Modal
          isOpen={showCceModal}
          title={`Carta de Correção — NF-e nº ${doc.number}`}
          onClose={() => setShowCceModal(false)}
          onSubmit={() => {
            if (cceText.trim().length >= 15) cceMutation.mutate()
          }}
          submitLabel="Enviar CC-e"
          cancelLabel="Voltar"
          loading={cceMutation.isPending}
          submitDisabled={cceText.trim().length < 15 || cceText.trim().length > 1000}
        >
          <div className="space-y-4">
            <p className="text-sm text-gray-600">
              A Carta de Correção permite corrigir informações da NF-e que não afetem o valor fiscal.
              Não é possível corrigir dados do emitente, destinatário, produto ou impostos.
            </p>
            <div>
              <label htmlFor="cce-text" className="block text-sm font-medium text-gray-700 mb-1.5">Texto da correção</label>
              <textarea
                id="cce-text"
                value={cceText}
                onChange={(e) => setCceText(e.target.value)}
                rows={5}
                maxLength={1000}
                placeholder="Descreva a correção (mínimo 15 caracteres)…"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400 resize-none"
              />
              <div className="flex justify-between mt-1">
                {cceText.trim().length < 15 && cceText.length > 0 && (
                  <p className="text-xs text-red-500">Mínimo 15 caracteres ({15 - cceText.trim().length} restantes)</p>
                )}
                <p className="text-xs text-gray-400 ml-auto">{cceText.length}/1000</p>
              </div>
            </div>
            <div>
              <label htmlFor="cce-seq" className="block text-sm font-medium text-gray-700 mb-1.5">Número de sequência</label>
              <input
                id="cce-seq"
                type="number"
                min={1}
                max={20}
                value={cceSeq}
                onChange={(e) => setCceSeq(Math.max(1, Math.min(20, parseInt(e.target.value) || 1)))}
                className="w-24 rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400"
              />
              <p className="mt-1 text-xs text-gray-400">1–20 (incremente se já enviou uma CC-e anterior)</p>
            </div>
            {cceMutation.isError && (
              <p className="text-xs text-red-600">{(cceMutation.error as Error)?.message ?? 'Erro ao enviar CC-e.'}</p>
            )}
          </div>
        </Modal>
      )}
    />
  )
}

// ─── Page wrapper ─────────────────────────────────────────────────────────────

function NfeDetailContent() {
  const params = useSearchParams()
  const accessKey = params.get('key') ?? ''

  const backParams = new URLSearchParams()
  const tab = params.get('tab')
  if (tab) backParams.set('tab', tab)
  const year = params.get('year')
  if (year) backParams.set('year', year)
  const month = params.get('month')
  if (month) backParams.set('month', month)
  const day = params.get('day')
  if (day) backParams.set('day', day)
  const number = params.get('number')
  if (number) backParams.set('number', number)
  const backHref = `/nfe${backParams.toString() ? `?${backParams.toString()}` : ''}`

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href={backHref} className="hover:text-brand-600">NF-e</Link>
          <span>/</span>
          <span className="text-gray-600 font-mono truncate max-w-[200px]">{accessKey || 'Detalhe'}</span>
        </div>
        {accessKey ? (
          <NfeDetail accessKey={accessKey}/>
        ) : (
          <p className="text-sm text-gray-500">Chave de acesso não informada.</p>
        )}
      </div>
    </RootLayout>
  )
}

export default function NfeDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfeDetailContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
