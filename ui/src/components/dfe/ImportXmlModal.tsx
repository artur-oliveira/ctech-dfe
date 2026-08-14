'use client'

import {useId, useState} from 'react'
import {useMutation} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {Modal} from '@/components/ui/modal'

export interface ImportXmlModalProps {
  docType: 'nfe' | 'nfce'
  isOpen: boolean
  onClose: () => void
}

const DOC_LABEL: Record<ImportXmlModalProps['docType'], string> = {
  nfe: 'NF-e',
  nfce: 'NFC-e',
}

/**
 * Upload de XML (nfeProc ou NFe) para importação assíncrona — o worker
 * classifica o vínculo com a organização, confirma junto à SEFAZ e persiste
 * como se o documento tivesse chegado pela distribuição. Compartilhado entre
 * a aba de Distribuição de NF-e e a listagem de NFC-e (ver ImportXML no
 * apiClient e o WS `import_xml_failed`, já tratado em useRealtimeUpdates).
 */
export function ImportXmlModal({docType, isOpen, onClose}: ImportXmlModalProps) {
  const [file, setFile] = useState<File | null>(null)
  const fileId = useId()
  const docLabel = DOC_LABEL[docType]

  const mutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error('Selecione um arquivo XML.')
      return apiClient.importXML(docType, file)
    },
    onSuccess: () => {
      setFile(null)
      onClose()
      toast.info(`Importação enfileirada. A ${docLabel} aparecerá automaticamente quando processada.`)
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao importar XML.')
    },
  })

  const handleClose = () => {
    setFile(null)
    onClose()
  }

  return (
    <Modal
      isOpen={isOpen}
      title={`Importar ${docLabel} por XML`}
      onClose={handleClose}
      onSubmit={() => mutation.mutate()}
      submitLabel="Importar"
      cancelLabel="Cancelar"
      loading={mutation.isPending}
      submitDisabled={!file}
    >
      <div className="space-y-2">
        <label htmlFor={fileId} className="block text-sm font-medium text-gray-700">
          Arquivo XML
        </label>
        <input
          id={fileId}
          type="file"
          accept=".xml,application/xml,text/xml"
          className="block w-full text-sm text-gray-600 file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-gray-300 file:text-sm file:font-medium file:bg-white file:text-gray-700 hover:file:bg-gray-50 cursor-pointer"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
        {file && <p className="mt-1 text-xs text-gray-500">{file.name}</p>}
        <p className="text-xs text-gray-500">
          Aceita XML de {docLabel} completo (<code className="font-mono">nfeProc</code>) ou assinado sem
          protocolo (<code className="font-mono">NFe</code>).
        </p>
      </div>
    </Modal>
  )
}
