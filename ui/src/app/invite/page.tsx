'use client'

import {Suspense, useState} from 'react'
import {useRouter, useSearchParams} from 'next/navigation'
import {useMutation, useQuery} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {Button} from '@/components/ui/button'
import {ROLE_LABEL, RoleName} from "@/lib/data/roles";

function InviteContent({token}: { token: string }) {
    const router = useRouter()
    const {refreshUser} = useAuth()
    const [error, setError] = useState<string | null>(null)

    const {data: preview, isPending, error: fetchError} = useQuery({
        queryKey: queryKeys.invitation(token),
        queryFn: () => apiClient.getInvitation(token),
        retry: false,
        enabled: !!token,
    })

    const acceptMutation = useMutation({
        mutationFn: () => apiClient.acceptInvitation(token),
        onSuccess: async () => {
            await refreshUser()
            router.replace('/dashboard')
        },
        onError: (e) => setError(e instanceof ApiError ? e.detail : 'Não foi possível aceitar o convite'),
    })
    const declineMutation = useMutation({
        mutationFn: () => apiClient.declineInvitation(token),
        onSuccess: () => router.replace('/dashboard'),
        onError: (e) => setError(e instanceof ApiError ? e.detail : 'Não foi possível recusar o convite'),
    })

    const card = (children: React.ReactNode) => (
        <div className="flex items-center justify-center min-h-[60vh] p-4">
            <div
                className="w-full max-w-md rounded-xl border border-gray-200 bg-white p-6 md:p-8 text-center space-y-4">
                {children}
            </div>
        </div>
    )

    if (!token) {
        return card(
            <>
                <h1 className="text-lg font-semibold text-gray-900">Convite inválido</h1>
                <p className="text-sm text-gray-500">Nenhum token de convite fornecido.</p>
                <Button variant="outline" className="w-full h-11" onClick={() => router.replace('/dashboard')}>Ir para o
                    painel</Button>
            </>,
        )
    }

    if (isPending) {
        return card(<div className="h-24 animate-pulse rounded bg-gray-100"/>)
    }
    if (fetchError || !preview) {
        return card(
            <>
                <h1 className="text-lg font-semibold text-gray-900">Convite não encontrado</h1>
                <p className="text-sm text-gray-500">O link é inválido ou já não existe.</p>
                <Button variant="outline" className="w-full h-11" onClick={() => router.replace('/dashboard')}>Ir para o
                    painel</Button>
            </>,
        )
    }

    const invalid =
        preview.already_member ? 'Você já faz parte desta organização.'
            : preview.expired ? 'Este convite expirou.'
                : preview.status !== 'PENDING' ? 'Este convite já foi utilizado ou revogado.'
                    : null

    if (invalid) {
        return card(
            <>
                <h1 className="text-lg font-semibold text-gray-900">{preview.org_name || 'Organização'}</h1>
                <p className="text-sm text-gray-600">{invalid}</p>
                <Button className="w-full h-11" onClick={() => router.replace('/dashboard')}>Ir para o painel</Button>
            </>,
        )
    }

    return card(
        <>
            <h1 className="text-lg font-semibold text-gray-900">Convite
                para {preview.org_name || 'uma organização'}</h1>
            <p className="text-sm text-gray-600 leading-relaxed">
                {preview.invited_by_name || 'Alguém'} convidou você para participar como{' '}
                <strong>{ROLE_LABEL[preview.role as RoleName] ?? preview.role}</strong>.
            </p>
            {error && (
                <div
                    className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                    {error}
                </div>
            )}
            <div className="flex flex-col sm:flex-row gap-2 pt-2">
                <Button className="flex-1 h-11"
                        disabled={acceptMutation.isPending || declineMutation.isPending}
                        onClick={() => {
                            setError(null);
                            acceptMutation.mutate()
                        }}>
                    {acceptMutation.isPending ? 'Entrando…' : 'Aceitar'}
                </Button>
                <Button variant="outline" className="flex-1 h-11"
                        disabled={acceptMutation.isPending || declineMutation.isPending}
                        onClick={() => {
                            setError(null);
                            declineMutation.mutate()
                        }}>
                    Recusar
                </Button>
            </div>
        </>,
    )
}

function InviteParamsWrapper() {
    const searchParams = useSearchParams()
    const token = searchParams.get('token') || ''
    return <InviteContent token={token}/>
}

export default function InvitePage() {
    return (
        <ProtectedRoute>
            <RootLayout>
                <Suspense fallback={
                    <div className="flex items-center justify-center min-h-[60vh] p-4">
                        <div className="w-full max-w-md h-24 animate-pulse rounded bg-gray-100"/>
                    </div>
                }>
                    <InviteParamsWrapper/>
                </Suspense>
            </RootLayout>
        </ProtectedRoute>
    )
}
