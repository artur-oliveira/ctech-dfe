'use client'

import {useState} from 'react'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {Button} from '@/components/ui/button'
import {startCompanyHandoff} from '@/lib/handoff'
import {STEP_COMPANY} from '@/lib/constants/onboarding'

/**
 * The company layer, which happens in the CTech account.
 *
 * This step used to be the full cadastro form, writing a company straight into
 * this product. That is the record ctech-billing ADR 0022 moved: identity lives
 * in the account, and a company created here would have no company id and no
 * reach edge — a workspace the platform never heard of.
 *
 * Deliberately NOT an automatic redirect, unlike `/organizations/new`. Somebody
 * arriving at the second step of setup did not ask to leave for another domain,
 * and a screen that throws them to a different host before they read anything
 * is how a flow feels broken. One button, and they know where they are going
 * and that they come back.
 */
function CompanyStepContent() {
  const [leaving, setLeaving] = useState(false)

  return (
    <OnboardingShell
      current={STEP_COMPANY}
      title="Cadastre sua empresa"
      description="O CNPJ da empresa é cadastrado na sua conta CTech, que é a mesma para todos os produtos. Você volta para cá em seguida, com a empresa já vinculada."
    >
      <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
        <h2 className="text-base font-semibold text-gray-900">O que acontece agora</h2>
        <ol className="mt-3 flex flex-col gap-2.5 text-sm leading-relaxed text-gray-600">
          <li className="flex gap-2.5">
            <span aria-hidden="true" className="text-gray-400 tabular-nums">1.</span>
            Você informa o CNPJ na conta CTech, e os dados vêm da Receita.
          </li>
          <li className="flex gap-2.5">
            <span aria-hidden="true" className="text-gray-400 tabular-nums">2.</span>
            Voltamos para cá com a empresa vinculada, para você conferir endereço e regime.
          </li>
          <li className="flex gap-2.5">
            <span aria-hidden="true" className="text-gray-400 tabular-nums">3.</span>
            Na sequência, o certificado A1 e os documentos que você emite.
          </li>
        </ol>

        <Button
          size="lg"
          className="mt-5 w-full sm:w-auto"
          disabled={leaving}
          onClick={() => {
            setLeaving(true)
            startCompanyHandoff()
          }}
        >
          {leaving ? 'Abrindo a conta CTech…' : 'Cadastrar empresa na conta CTech'}
        </Button>
      </div>
    </OnboardingShell>
  )
}

export default function CompanyStepPage() {
  return (
    <ProtectedRoute>
      <CompanyStepContent/>
    </ProtectedRoute>
  )
}
