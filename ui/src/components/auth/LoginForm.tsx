'use client'

import {useEffect} from 'react'
import {useRouter} from 'next/navigation'
import {useAuth} from '@/lib/hooks/useAuth'

// Legacy component — login page now redirects directly to OAuth.
// This component handles the edge case where LoginForm is rendered while a session already exists.
export function LoginForm() {
  const {login, user, loading} = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!loading && user) {
      router.replace('/dashboard')
    } else if (!loading && !user) {
      login()
    }
  }, [user, loading, router, login])

  return null
}
