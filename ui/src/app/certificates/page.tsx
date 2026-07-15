'use client'

import {useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Button} from '@/components/ui/button'
import {Modal} from '@/components/ui/modal'
import {EmptyState} from '@/components/ui/empty-state'
import {ShieldIcon} from '@/components/ui/icon'
import {CertificateFields} from '@/components/organizations/CertificateFields'
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'
import type {CertificateOut} from '@/lib/types/api'

function certStatus(expiresAt: string): 'valid' | 'expiring' | 'expired' {
  const now = Date.now()
  const exp = new Date(expiresAt).getTime()
  if (exp < now) return 'expired'
  if (exp - now < 30 * 24 * 60 * 60 * 1000) return 'expiring'
  return 'valid'
}

const STATUS_BADGE: Record<ReturnType<typeof certStatus>, string> = {
  valid: 'bg-green-100 text-green-700',
  expiring: 'bg-yellow-100 text-yellow-700',
  expired: 'bg-red-100 text-red-700',
}

const STATUS_LABEL: Record<ReturnType<typeof certStatus>, string> = {
  valid: 'Válido',
  expiring: 'Expirando',
  expired: 'Expirado',
}

function UploadModal({
                       onClose,
                       onUpload,
                       loading,
                       serverError,
                     }: {
  onClose: () => void
  onUpload: (file: File, password: string) => void
  loading: boolean
  serverError: string | null
}) {
  const [file, setFile] = useState<File | null>(null)
  const [password, setPassword] = useState('')
  const [fileError, setFileError] = useState<string | null>(null)
  const [passwordError, setPasswordError] = useState<string | null>(null)
  
  const handleSubmit = () => {
    let ok = true
    if (!file) {
      setFileError('Selecione um arquivo')
      ok = false
    }
    if (!password) {
      setPasswordError('Senha é obrigatória')
      ok = false
    }
    if (ok && file) onUpload(file, password)
  }
  
  return (
    <Modal isOpen title="Importar Certificado" onClose={onClose} onSubmit={handleSubmit}
           submitLabel="Importar" loading={loading}>
      <div className="space-y-4">
        {serverError && (
          <div
            className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {serverError}
          </div>
        )}
        <CertificateFields
          file={file}
          onFileChange={(f) => {
            setFile(f);
            setFileError(null)
          }}
          password={password}
          onPasswordChange={(p) => {
            setPassword(p);
            setPasswordError(null)
          }}
          fileError={fileError}
          passwordError={passwordError}
        />
      </div>
    </Modal>
  )
}

function CertRow({cert, onDelete, isDeleting}: { cert: CertificateOut; onDelete: () => void; isDeleting: boolean }) {
  const status = certStatus(cert.expires_at)
  return (
    <tr className={TABLE_ROW}>
      <td data-label="Certificado" className={`${TABLE_CELL} pl-6`}>
        <p className="font-medium text-gray-900 text-sm">{cert.alias}</p>
        <p className="text-xs text-gray-400 font-mono mt-0.5">{cert.md5}</p>
      </td>
      <td data-label="Validade" className={`${TABLE_CELL} pr-6`}>
        <div className="flex items-center gap-2">
          <span
            className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[status]}`}>
            {STATUS_LABEL[status]}
          </span>
          <span className="text-sm text-gray-600">
            {new Date(cert.expires_at).toLocaleDateString('pt-BR')}
          </span>
        </div>
      </td>
      <td data-label="Importado em" className={`${TABLE_CELL} pr-6 text-sm text-gray-500`}>
        {new Date(cert.created_at).toLocaleDateString('pt-BR')}
      </td>
      <td className={`${TABLE_CELL} pr-6 text-right`}>
        <Button
          variant="ghost"
          size="xs"
          onClick={onDelete}
          disabled={isDeleting}
          className="text-danger hover:text-red-700"
        >
          Remover
        </Button>
      </td>
    </tr>
  )
}

function CertificatesContent() {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  
  const pk = selectedOrg?.pk ?? ''
  
  const {data: certs, isPending, error: fetchError} = useQuery({
    queryKey: queryKeys.certificates(pk),
    queryFn: () => apiClient.getCertificates(pk),
    enabled: !!pk,
  })
  
  const uploadMutation = useMutation({
    mutationFn: ({file, password}: { file: File; password: string }) =>
      apiClient.uploadCertificate(pk, file, password),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.certificates(pk)})
      setShowModal(false)
      setUploadError(null)
    },
    onError: (err) => {
      setUploadError(err instanceof ApiError ? err.detail : 'Erro ao importar certificado')
    },
  })
  
  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<CertificateOut>({
    mutationFn: (md5) => apiClient.deleteCertificate(pk, md5),
    getId: (cert) => cert.md5,
    getDeletedMessage: (cert) => `Certificado "${cert.alias}" excluído`,
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.certificates(pk)}),
  })
  const visibleCerts = filterVisible(certs ?? [])

  const handleCloseModal = () => {
    setShowModal(false)
    setUploadError(null)
  }
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-8 flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">Certificados Digitais</h1>
            <p className="mt-1 text-sm text-gray-500">
              Gerencie os certificados A1 utilizados na emissão de documentos fiscais
            </p>
          </div>
          {selectedOrg && (
            <Button onClick={() => setShowModal(true)} className="shrink-0">
              Importar certificado
            </Button>
          )}
        </div>
        
        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : (
          <>
            {fetchError && (
              <div
                className="mb-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                {fetchError.message}
              </div>
            )}
            
            {isPending ? (
              <div className="space-y-2">
                {[1, 2].map((i) => (
                  <div key={i} className="h-14 rounded-lg bg-gray-100 animate-pulse"/>
                ))}
              </div>
            ) : !visibleCerts.length ? (
              <EmptyState
                title="Nenhum certificado cadastrado"
                description="Importe um certificado A1 no formato .pfx ou .p12"
                action={{label: 'Importar certificado', onClick: () => setShowModal(true)}}
                icon={<ShieldIcon width={20} height={20}/>}
              />
            ) : (
              <TableShell
                ariaLabel="Certificados"
                minWidth={480}
                headers={['Certificado', 'Validade', 'Importado em', {label: '', align: 'right'}]}
              >
                {visibleCerts.map((cert) => (
                  <CertRow
                    key={cert.md5}
                    cert={cert}
                    onDelete={() => handleDelete(cert)}
                    isDeleting={isDeleting}
                  />
                ))}
              </TableShell>
            )}
          </>
        )}
      </div>
      
      {showModal && (
        <UploadModal
          onClose={handleCloseModal}
          onUpload={(file, password) => uploadMutation.mutate({file, password})}
          loading={uploadMutation.isPending}
          serverError={uploadError}
        />
      )}
    </RootLayout>
  )
}

export default function CertificatesPage() {
  return (
    <ProtectedRoute>
      <CertificatesContent/>
    </ProtectedRoute>
  )
}
