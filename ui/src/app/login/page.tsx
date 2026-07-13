'use client'

import {Suspense, useEffect} from 'react'
import {useSearchParams} from 'next/navigation'
import {startOAuthFlow} from '@/lib/auth/oauth'

function LoginInner() {
  const searchParams = useSearchParams()
  
  useEffect(() => {
    const returnTo = searchParams.get('returnTo') ?? '/dashboard'
    void startOAuthFlow(returnTo)
  }, [searchParams])
  
  return (
    <div className="min-h-screen bg-gradient-login flex items-center justify-center px-4">
      <div className="text-center space-y-3">
        <div
          className="w-10 h-10 border-4 border-primary-200 border-t-primary-600 rounded-full animate-spin mx-auto"/>
        <p className="text-gray-600 text-sm">Redirecionando para autenticação...</p>
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
