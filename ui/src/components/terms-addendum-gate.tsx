'use client'

import {useState} from 'react'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {DFE_TERMS_URL} from '@/lib/legal'

// Blocks access until the user explicitly accepts the dfe-specific terms
// addendum — shown once, right after first login, since Google/SSO sign-up
// never presents a checkbox of its own for product-specific terms.
export function TermsAddendumGate() {
  const {refreshUser} = useAuth()
  const [checked, setChecked] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function handleAccept() {
    if (!checked) return
    setError('')
    setLoading(true)
    try {
      await apiClient.acceptTermsAddendum()
      await refreshUser()
    } catch {
      setError('Não foi possível confirmar. Tente novamente.')
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md space-y-5 rounded-2xl border border-primary-100 bg-white p-6 shadow-card">
        <div>
          <h1 className="text-lg font-semibold text-gray-900">Só mais um passo</h1>
          <p className="mt-1 text-sm text-gray-600">
            Antes de continuar, confirme que você leu os termos específicos do CTech DFe.
          </p>
        </div>

        {error && <p className="text-sm text-red-600">{error}</p>}

        <label className="flex items-start gap-2 text-sm text-gray-600">
          <input
            type="checkbox"
            checked={checked}
            onChange={(e) => setChecked(e.target.checked)}
            className="mt-0.5 size-4 shrink-0 rounded border-gray-300 accent-primary-600"
          />
          <span>
            Li e concordo com os{' '}
            <a href={DFE_TERMS_URL} target="_blank" rel="noreferrer" className="text-gray-900 underline underline-offset-4">
              Termos Adicionais do CTech DFe
            </a>
            .
          </span>
        </label>

        <button
          type="button"
          onClick={handleAccept}
          disabled={!checked || loading}
          className="w-full rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-primary-700 disabled:opacity-50"
        >
          {loading ? 'Confirmando…' : 'Continuar'}
        </button>
      </div>
    </div>
  )
}
