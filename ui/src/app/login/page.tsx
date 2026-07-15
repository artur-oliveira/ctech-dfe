'use client'

import {Suspense, useEffect} from 'react'
import {useRouter, useSearchParams} from 'next/navigation'
import {FileText} from 'lucide-react'
import {useAuth} from '@/lib/hooks/useAuth'
import {startOAuthFlow} from '@/lib/auth/oauth'
import {Button} from '@/components/ui/button'

function LoginInner() {
  const {user, loading} = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const returnTo = searchParams.get('returnTo') ?? '/dashboard'

  // Already signed in — go straight to the app.
  useEffect(() => {
    if (!loading && user) router.replace(returnTo)
  }, [loading, user, router, returnTo])

  return (
    <div className="min-h-screen bg-gradient-login flex items-center justify-center p-4">
      <div className="w-full max-w-sm rounded-2xl border border-gray-200 bg-white p-8 text-center shadow-card">
        <div className="mx-auto flex size-12 items-center justify-center rounded-xl bg-primary-600 text-white">
          <FileText size={22}/>
        </div>
        <h1 className="mt-4 text-2xl font-bold text-gray-900">CTech DFe</h1>
        <p className="mt-2 text-sm leading-relaxed text-gray-600">
          Emita e gerencie seus documentos fiscais — NF-e, NFC-e, CT-e e MDF-e.
        </p>
        <Button
          variant="brand"
          size="lg"
          className="mt-6 w-full h-11"
          disabled={loading || !!user}
          onClick={() => void startOAuthFlow(returnTo)}
        >
          Entrar
        </Button>
      </div>
    </div>
  )
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginInner/>
    </Suspense>
  )
}
