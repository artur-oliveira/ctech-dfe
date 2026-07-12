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
import {CertificateFields} from '@/components/organizations/CertificateFields'
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
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
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md mx-4">
        <div className="border-b border-gray-200 px-6 py-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Importar Certificado</h2>
          <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Fechar">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
                 strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </Button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="p-6 space-y-4">
            {serverError && (
              <div
                className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                {serverError}
              </div>
            )}
            <CertificateFields
              file={file}
              onFileChange={(f) => { setFile(f); setFileError(null) }}
              password={password}
              onPasswordChange={(p) => { setPassword(p); setPasswordError(null) }}
              fileError={fileError}
              passwordError={passwordError}
            />
          </div>

          <div className="border-t border-gray-200 px-6 py-4 flex justify-end gap-3">
            <Button type="button" variant="outline" onClick={onClose} disabled={loading}>
              Cancelar
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Importando…' : 'Importar'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

function CertRow({cert, onDelete, isDeleting}: { cert: CertificateOut; onDelete: () => void; isDeleting: boolean }) {
  const status = certStatus(cert.expires_at)
  return (
    <tr className="border-b border-gray-100 last:border-0">
      <td className="py-3 pl-6 pr-4">
        <p className="font-medium text-gray-900 text-sm">{cert.alias}</p>
        <p className="text-xs text-gray-400 font-mono mt-0.5">{cert.md5}</p>
      </td>
      <td className="py-3 pr-4">
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
      <td className="py-3 pr-4 text-sm text-gray-500">
        {new Date(cert.created_at).toLocaleDateString('pt-BR')}
      </td>
      <td className="py-3 pr-6 text-right">
        <Button
          variant="ghost"
          size="xs"
          onClick={onDelete}
          disabled={isDeleting}
          className="text-red-500 hover:text-red-700"
        >
          {isDeleting ? 'Removendo…' : 'Remover'}
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

  const deleteMutation = useMutation({
    mutationFn: (md5: string) => apiClient.deleteCertificate(pk, md5),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.certificates(pk)}),
  })

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
            ) : !certs?.length ? (
              <div className="rounded-xl border border-dashed border-gray-300 bg-white py-16 text-center">
                <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"
                       strokeLinecap="round" strokeLinejoin="round" className="text-gray-400">
                    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                  </svg>
                </div>
                <p className="text-sm font-medium text-gray-900">Nenhum certificado cadastrado</p>
                <p className="mt-1 text-sm text-gray-500">
                  Importe um certificado A1 no formato .pfx ou .p12
                </p>
                <Button className="mt-4" onClick={() => setShowModal(true)}>
                  Importar certificado
                </Button>
              </div>
            ) : (
              <div className="rounded-xl border border-gray-200 bg-white overflow-x-auto">
                <table className="w-full min-w-[480px]">
                  <thead>
                  <tr className="border-b border-gray-100">
                    <th
                      className="py-3 pl-6 pr-4 text-left text-xs font-semibold uppercase tracking-wider text-gray-400">
                      Certificado
                    </th>
                    <th className="py-3 pr-4 text-left text-xs font-semibold uppercase tracking-wider text-gray-400">
                      Validade
                    </th>
                    <th className="py-3 pr-4 text-left text-xs font-semibold uppercase tracking-wider text-gray-400">
                      Importado em
                    </th>
                    <th className="py-3 pr-6 text-right text-xs font-semibold uppercase tracking-wider text-gray-400"/>
                  </tr>
                  </thead>
                  <tbody>
                  {certs.map((cert) => (
                    <CertRow
                      key={cert.md5}
                      cert={cert}
                      onDelete={() => deleteMutation.mutate(cert.md5)}
                      isDeleting={deleteMutation.isPending && deleteMutation.variables === cert.md5}
                    />
                  ))}
                  </tbody>
                </table>
              </div>
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
