'use client'

import {useEffect, useRef, useState} from 'react'
import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useQueryClient} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {Button} from '@/components/ui/button'
import {HANDOFF_STATE_KEY} from '@/lib/handoff'

/**
 * The handoff's return leg: the person created a company in the CTech account
 * and came back with both ids.
 *
 * It links and then leaves. There is no form and no confirmation step — the
 * decision was made on the other side, and asking again here would be asking
 * somebody to confirm what they just did.
 *
 * The query is read from `window.location`, NOT from `useSearchParams`. This
 * app builds with `output: 'export'`, so this route is a static file with no
 * query at build time, and on the hydrating render `useSearchParams()` answers
 * with an empty set before the real one arrives. This page acts once on mount,
 * so it read that empty set and refused a return that was perfectly complete —
 * which is exactly the bug this comment exists to stop somebody reintroducing.
 * `window.location.search` is the URL the browser is actually on.
 */
function LinkCompanyContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const {refreshUser, setSelectedOrg} = useAuth()
  const [error, setError] = useState<string | null>(null)
  // An organization came back without a company: recoverable, and not by
  // starting over.
  const [incompleteOrg, setIncompleteOrg] = useState<string | null>(null)
  const [cancelled, setCancelled] = useState(false)
  // A React 18 dev double-mount would otherwise link twice. The server is
  // idempotent, so this is about not showing two errors, not about correctness.
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true

    // Everything, including the validation, runs inside the async continuation.
    // A synchronous setState in an effect is both a lint error and a wasted
    // render, and there is nothing here the first paint needs.
    void (async () => {
      const params = new URLSearchParams(window.location.search)
      if (params.get('cancelled') === '1') {
        setCancelled(true)
        return
      }
      const organizationId = params.get('organization_id') ?? ''
      const companyId = params.get('company_id') ?? ''

      // An organization with no company is its own case, and telling somebody
      // to "start over" here would be wrong twice: the workspace they just
      // created still exists, and starting over creates a second one. The way
      // out is to add the company to the workspace that is already there.
      if (organizationId && !companyId) {
        setIncompleteOrg(organizationId)
        return
      }
      if (!organizationId || !companyId) {
        setError('O endereço de retorno está incompleto. Comece novamente pela lista de empresas.')
        return
      }
      // The state we sent must be the state that came back. It is what tells a
      // real return apart from somebody opening this URL with ids they typed.
      const expected = sessionStorage.getItem(HANDOFF_STATE_KEY)
      if (expected && params.get('state') !== expected) {
        setError('Este retorno não corresponde ao cadastro iniciado aqui. Comece novamente.')
        return
      }

      try {
        const org = await apiClient.linkCompany(organizationId, companyId)
        sessionStorage.removeItem(HANDOFF_STATE_KEY)
        void qc.invalidateQueries({queryKey: queryKeys.organizations.all()})
        const me = await refreshUser()
        const linked = me?.organizations.find((o) => o.pk === org.pk)
        // Navigating without it would leave the PREVIOUS company selected, and
        // every request from the company's own screen would carry that
        // company's pk — the person edits one company and writes to another.
        // The link itself already succeeded, so this says so.
        if (!linked) {
          setError('A empresa foi vinculada, mas ainda não apareceu na sua lista. Recarregue a página em instantes.')
          return
        }
        setSelectedOrg(linked)
        // Straight to the company's own screen: the fiscal side is empty, and
        // that is the next thing the person has to do. `from=link` says this
        // edit is the tail of the handoff, which is what lets that screen fill
        // the blanks from the CNPJ and then continue the setup flow.
        router.replace(`/organizations/edit?pk=${encodeURIComponent(org.pk)}&from=link`)
      } catch (e) {
        setError(e instanceof ApiError ? e.detail : 'Não foi possível vincular a empresa.')
      }
    })()
  }, [qc, refreshUser, setSelectedOrg, router])

  if (cancelled) {
    return (
      <Shell title="Cadastro cancelado">
        <p className="text-sm text-gray-600">
          Nenhuma empresa foi criada. Você pode tentar novamente quando quiser.
        </p>
        <Link href="/organizations">
          <Button variant="outline" className="mt-4">Voltar para empresas</Button>
        </Link>
      </Shell>
    )
  }

  if (incompleteOrg) {
    const accountUrl = process.env.NEXT_PUBLIC_CTECH_CLIENT_URL ?? ''
    return (
      <Shell title="Falta a empresa">
        <p className="text-sm text-gray-600">
          O espaço de trabalho foi criado, mas nenhuma empresa foi cadastrada nele — e é a
          empresa (o CNPJ) que emite os documentos. Cadastre a empresa na conta CTech e volte.
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            onClick={() => window.open(
              `${accountUrl}/account/organizations/detail?id=${encodeURIComponent(incompleteOrg)}`,
              '_blank', 'noopener,noreferrer')}>
            Cadastrar a empresa
          </Button>
          <Link href="/organizations">
            <Button variant="outline">Voltar para empresas</Button>
          </Link>
        </div>
      </Shell>
    )
  }

  if (error) {
    return (
      <Shell title="Não foi possível vincular">
        <p className="text-sm text-gray-600">{error}</p>
        <Link href="/organizations">
          <Button variant="outline" className="mt-4">Voltar para empresas</Button>
        </Link>
      </Shell>
    )
  }

  return (
    <Shell title="Vinculando empresa">
      <p className="text-sm text-gray-500">Só um momento…</p>
    </Shell>
  )
}

function Shell({title, children}: { title: string; children: React.ReactNode }) {
  return (
    <RootLayout>
      <div className="p-4 md:p-8 max-w-md">
        <h1 className="text-2xl font-semibold text-gray-900 mb-2">{title}</h1>
        {children}
      </div>
    </RootLayout>
  )
}

export default function LinkCompanyPage() {
  return (
    <ProtectedRoute>
      <LinkCompanyContent/>
    </ProtectedRoute>
  )
}
