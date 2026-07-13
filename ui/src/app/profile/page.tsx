'use client'

import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {SectionCard} from '@/components/ui/section-card'
import {Button} from '@/components/ui/button'

const CTECH_URL = process.env.NEXT_PUBLIC_CTECH_CLIENT_URL ?? ''

const UserIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
    <circle cx="12" cy="7" r="4"/>
  </svg>
)

function ProfileContent() {
  const {user} = useAuth()
  
  const initials = user
    ? `${user.first_name[0] ?? ''}${user.last_name[0] ?? ''}`.toUpperCase()
    : ''
  
  return (
    <div className="max-w-2xl mx-auto px-4 py-8 space-y-6">
      <div className="flex items-center gap-4">
        <div
          className="w-14 h-14 rounded-full flex items-center justify-center text-white text-lg font-semibold shrink-0"
          style={{backgroundColor: 'var(--brand-600)'}}
        >
          {initials}
        </div>
        <div>
          <h1 className="text-xl font-semibold text-gray-900">
            {user?.first_name} {user?.last_name}
          </h1>
          <p className="text-sm text-gray-500">{user?.email}</p>
        </div>
      </div>
      
      <SectionCard icon={<UserIcon/>} title="Conta">
        <div className="space-y-3">
          <p className="text-sm text-gray-500 mb-3">
            <span className="font-medium text-gray-700">Usuário: </span>{user?.username}
          </p>
          <p className="text-sm text-gray-500 mb-4">
            <span className="font-medium text-gray-700">E-mail: </span>{user?.email}
          </p>
          <p className="text-sm text-gray-500">
            Para alterar seus dados pessoais ou senha, acesse sua conta ctech.
          </p>
          <div className="pt-2">
            <Button
              type="button"
              onClick={() => window.open(CTECH_URL + '/account/profile', '_blank', 'noopener,noreferrer')}
            >
              Gerenciar conta
            </Button>
          </div>
        </div>
      </SectionCard>
    </div>
  )
}

export default function ProfilePage() {
  return (
    <ProtectedRoute>
      <RootLayout>
        <ProfileContent/>
      </RootLayout>
    </ProtectedRoute>
  )
}
