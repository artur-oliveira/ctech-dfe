'use client'

import {useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Button} from '@/components/ui/button'
import {Badge} from '@/components/ui/badge'
import {OptionsSelect} from '@/components/ui/options-select'
import type {MemberOut} from '@/lib/types/api'
import {ASSIGNABLE_ROLES, ROLE_LABEL, RoleName} from "@/lib/data/roles";


/** Display label for a member: the name snapshot taken at grant time, else the raw id. */
function memberLabel(m: MemberOut): string {
    return m.name?.trim() || m.user_id
}

/** Formats an ISO date, tolerating rows written before created_at existed. */
function formatDate(iso: string | undefined): string | null {
    if (!iso) return null
    const d = new Date(iso)
    return Number.isNaN(d.getTime()) ? null : d.toLocaleDateString('pt-BR')
}

function MembersContent() {
    const {user, selectedOrg} = useAuth()
    const qc = useQueryClient()
    const pk = selectedOrg?.pk ?? ''
    const isOwner = selectedOrg?.role === 'OWNER'
    const [shareOpen, setShareOpen] = useState(false)
    const [actionError, setActionError] = useState<string | null>(null)

    const membersQuery = useQuery({
        queryKey: queryKeys.members(pk),
        queryFn: () => apiClient.listMembers(pk),
        enabled: !!pk,
    })
    const invitationsQuery = useQuery({
        queryKey: queryKeys.invitations(pk),
        queryFn: () => apiClient.listInvitations(pk),
        enabled: !!pk,
    })

    // Removal is optimistic with a 5s undo window (same UX as products/persons/vehicles).
    const {handleDelete, filterVisible} = useEntityDelete<MemberOut>({
        mutationFn: (id) => apiClient.removeMember(pk, id),
        getId: (m) => m.user_id,
        getDeletedMessage: (m) => `${memberLabel(m)} removido`,
        onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.members(pk)}),
    })

    const roleMutation = useMutation({
        mutationFn: ({userId, role}: { userId: string; role: string }) => apiClient.updateMemberRole(pk, userId, role),
        onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.members(pk)}),
        onError: (e) => setActionError(e instanceof ApiError ? e.detail : 'Erro ao alterar função'),
    })
    const revokeMutation = useMutation({
        mutationFn: (id: string) => apiClient.revokeInvitation(pk, id),
        onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.invitations(pk)}),
        onError: (e) => setActionError(e instanceof ApiError ? e.detail : 'Erro ao revogar convite'),
    })

    if (!selectedOrg) {
        return <NoOrgBanner/>
    }

    const visibleMembers = filterVisible(membersQuery.data ?? [])

    return (
        <div className="p-4 md:p-8 max-w-4xl">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
                <div>
                    <h1 className="text-2xl font-semibold text-gray-900">Usuários</h1>
                    <p className="text-sm text-gray-500 mt-1">Quem tem acesso a {selectedOrg.name}</p>
                </div>
                <Button className="h-11 sm:h-10" onClick={() => setShareOpen(true)}>Compartilhar acesso</Button>
            </div>

            {actionError && (
                <div
                    className="mb-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                    {actionError}
                </div>
            )}

            {/* Members */}
            <section className="rounded-xl border border-gray-200 mb-8">
                <h2 className="text-base font-semibold text-gray-900 px-4 md:px-6 py-3 border-b border-gray-100">Membros</h2>
                {membersQuery.isPending ? (
                    <div className="p-6 space-y-3">
                        {[0, 1, 2].map((i) => <div key={i} className="h-10 rounded bg-gray-100 animate-pulse"/>)}
                    </div>
                ) : (
                    <ul className="divide-y divide-gray-100">
                        {visibleMembers.map((m) => (
                            <MemberRow
                                key={m.user_id}
                                member={m}
                                isOwner={isOwner}
                                isSelf={m.user_id === user?.user_id}
                                onChangeRole={(role) => {
                                    setActionError(null);
                                    roleMutation.mutate({userId: m.user_id, role})
                                }}
                                onRemove={() => {
                                    setActionError(null);
                                    handleDelete(m)
                                }}
                                busy={roleMutation.isPending}
                            />
                        ))}
                    </ul>
                )}
            </section>

            {/* Pending invitations */}
            <section className="rounded-xl border border-gray-200">
                <h2 className="text-base font-semibold text-gray-900 px-4 md:px-6 py-3 border-b border-gray-100">Convites
                    pendentes</h2>
                {invitationsQuery.isPending ? (
                    <div className="p-6">
                        <div className="h-10 rounded bg-gray-100 animate-pulse"/>
                    </div>
                ) : (invitationsQuery.data ?? []).length === 0 ? (
                    <p className="px-4 md:px-6 py-6 text-sm text-gray-500">Nenhum convite pendente.</p>
                ) : (
                    <ul className="divide-y divide-gray-100">
                        {(invitationsQuery.data ?? []).map((inv) => (
                            <li key={inv.pk}
                                className="flex flex-col sm:flex-row sm:items-center gap-2 px-4 md:px-6 py-3">
                                <div className="flex-1 min-w-0">
                                    <p className="text-sm font-medium text-gray-900">{ROLE_LABEL[inv.role as RoleName] ?? inv.role}</p>
                                    {formatDate(inv.expires_at) && (
                                        <p className="text-xs text-gray-500">expira em {formatDate(inv.expires_at)}</p>
                                    )}
                                </div>
                                <Button variant="ghost" size="sm" className="text-danger hover:text-red-700 h-11 sm:h-9"
                                        disabled={revokeMutation.isPending}
                                        onClick={() => {
                                            setActionError(null);
                                            revokeMutation.mutate(inv.pk)
                                        }}>
                                    Revogar
                                </Button>
                            </li>
                        ))}
                    </ul>
                )}
            </section>

            {shareOpen && (
                <ShareModal
                    orgPk={pk}
                    onClose={() => {
                        setShareOpen(false);
                        void qc.invalidateQueries({queryKey: queryKeys.invitations(pk)})
                    }}
                />
            )}
        </div>
    )
}

function MemberRow({
                       member, isOwner, isSelf, onChangeRole, onRemove, busy,
                   }: {
    member: MemberOut
    isOwner: boolean
    isSelf: boolean
    onChangeRole: (role: string) => void
    onRemove: () => void
    busy: boolean
}) {
    const canManage = isOwner && !isSelf && member.role !== 'OWNER'
    const since = formatDate(member.created_at)
    return (
        <li className="flex flex-col sm:flex-row sm:items-center gap-2 px-4 md:px-6 py-3">
            <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900 truncate">{memberLabel(member)}{isSelf && ' (você)'}</p>
                {since && <p className="text-xs text-gray-500">desde {since}</p>}
            </div>
            {canManage ? (
                <div className="flex items-center gap-2">
                    <OptionsSelect
                        value={member.role}
                        onValueChange={onChangeRole}
                        options={ASSIGNABLE_ROLES}
                        disabled={busy}
                        className="h-11 sm:h-9 w-44"
                        ariaLabel={`Alterar função de ${memberLabel(member)}`}
                    />
                    <Button variant="ghost" size="sm" className="text-danger hover:text-red-700 h-11 sm:h-9"
                            disabled={busy} onClick={onRemove}>
                        Remover
                    </Button>
                </div>
            ) : (
                <Badge variant="secondary">{(ROLE_LABEL[member.role as RoleName] ?? member.role).toString()}</Badge>
            )}
        </li>
    )
}

// Where invitations live now.
//
// Not a form here any more. Invitations are ctech-account's (ctech-billing ADR
// 0023), and only the platform's can say WHICH COMPANIES the invitation grants
// — the case this product is for, an accountant inviting a junior to five of
// forty CNPJs. Keeping a second form would be two e-mails, two tokens and two
// ways to be half-invited.
function ShareModal({onClose}: { orgPk: string; onClose: () => void }) {
    const accountUrl = (process.env.NEXT_PUBLIC_CTECH_CLIENT_URL ?? '') + '/account/organizations'

    return (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
            <div className="bg-white rounded-lg shadow-modal w-full sm:max-w-md">
                <div className="border-b border-gray-200 px-6 py-4 flex items-center justify-between">
                    <h2 className="text-lg font-semibold text-gray-900">Compartilhar acesso</h2>
                    <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Fechar">✕</Button>
                </div>
                <div className="p-6 space-y-4">
                    <p className="text-sm text-gray-600">
                        Os convites agora são feitos na sua conta CTech, onde você também escolhe
                        <strong> quais empresas</strong> a pessoa poderá usar.
                    </p>
                    <Button
                        className="w-full h-11"
                        onClick={() => window.open(accountUrl, '_blank', 'noopener,noreferrer')}>
                        Convidar na conta CTech
                    </Button>
                    <Button variant="outline" className="w-full h-11" onClick={onClose}>Fechar</Button>
                </div>
            </div>
        </div>
    )
}

export default function MembersPage() {
    return (
        <ProtectedRoute>
            <RootLayout>
                <MembersContent/>
            </RootLayout>
        </ProtectedRoute>
    )
}
