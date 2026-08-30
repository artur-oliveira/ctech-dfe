'use client'

import {useState} from 'react'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {CertificateFields} from '@/components/organizations/CertificateFields'
import {Button} from '@/components/ui/button'
import {useAuth} from '@/lib/hooks/useAuth'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {ONBOARDING_ROOT, STEP_CERTIFICATE, STEP_DOCUMENTS} from '@/lib/constants/onboarding'

/**
 * The certificate layer.
 *
 * It is its own step because the A1 certificate is what signs every document —
 * without it the SEFAZ refuses every emission — and because a company created
 * through the ctech-account handoff never passes through a form that asks for
 * one. An account could otherwise reach the end of setup fully configured and
 * unable to issue anything.
 *
 * Nothing here is stored: the step is answered by the certificate list itself,
 * so a reload, a second device or a browser that died mid-upload all resume to
 * the same place.
 */
function CertificateStepContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const {selectedOrg} = useAuth()
  const {hasCertificate, certificateInherited, isPending} = useOnboarding()

  const [file, setFile] = useState<File | null>(null)
  const [password, setPassword] = useState('')
  const [fileError, setFileError] = useState<string | null>(null)

  const next = `${ONBOARDING_ROOT}/${STEP_DOCUMENTS}`
  const orgPk = selectedOrg?.pk

  const upload = useMutation({
    mutationFn: () => apiClient.uploadCertificate(orgPk as string, file as File, password),
    onSuccess: async () => {
      // The step is derived from this list, so it has to be re-read before the
      // next screen decides what is left — awaited, not fired and forgotten.
      await qc.invalidateQueries({queryKey: queryKeys.certificates(orgPk ?? '')})
      router.push(next)
    },
  })

  const submit = () => {
    setFileError(null)
    if (!file) {
      setFileError('Selecione o arquivo .pfx ou .p12')
      return
    }
    if (!password) return
    upload.mutate()
  }

  const done = !isPending && hasCertificate

  return (
    <OnboardingShell
      current={STEP_CERTIFICATE}
      title="Envie o certificado A1"
      description="É o certificado que assina cada documento. Sem ele a SEFAZ recusa a emissão — o resto da configuração funciona, a emissão não."
      action={
        // The escape is honest rather than hidden: setup is resumable, and
        // somebody whose contador holds the file should be able to keep going
        // instead of abandoning the flow here.
        done ? undefined : (
          <Button variant="ghost" onClick={() => router.push(next)}>
            Enviar depois
          </Button>
        )
      }
    >
      {done ? (
        <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
          <h2 className="text-base font-semibold text-gray-900">
            {certificateInherited ? 'Esta empresa já assina com o certificado da matriz' : 'Certificado ativo'}
          </h2>
          <p className="mt-1.5 text-sm leading-relaxed text-gray-600 text-pretty">
            {certificateInherited
              ? 'Ela faz parte de um grupo que você já administra (mesma raiz de CNPJ), então nada precisa ser enviado aqui.'
              : 'Nada a fazer nesta etapa. Você troca o certificado quando quiser, em Certificados.'}
          </p>
          <Button size="lg" className="mt-5 w-full sm:w-auto" onClick={() => router.push(next)}>
            Continuar
          </Button>
        </div>
      ) : (
        <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
          <CertificateFields
            file={file}
            onFileChange={(f) => {
              setFile(f)
              setFileError(null)
            }}
            password={password}
            onPasswordChange={setPassword}
            fileError={fileError}
            hint="Arquivo .pfx ou .p12 emitido para o CNPJ desta empresa, e a senha dele. A senha é usada para assinar e não fica visível em nenhuma tela."
          />

          {upload.error && (
            <p role="alert"
               className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {upload.error.message}
            </p>
          )}

          <Button
            size="lg"
            className="mt-5 w-full sm:w-auto"
            disabled={!orgPk || upload.isPending || !file || !password}
            onClick={submit}
          >
            {upload.isPending ? 'Enviando…' : 'Enviar certificado'}
          </Button>
        </div>
      )}
    </OnboardingShell>
  )
}

export default function CertificateStepPage() {
  return (
    <ProtectedRoute>
      <CertificateStepContent/>
    </ProtectedRoute>
  )
}
