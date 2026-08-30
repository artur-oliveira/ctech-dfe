'use client'

import {useState} from 'react'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {authorizedViewerSchema, hasDuplicateViewer, MAX_AUTHORIZED_VIEWERS} from '@/lib/schemas/authorized-viewers'
import {maskCpfCnpj} from '@/lib/utils/masks'
import type {AuthorizedViewerOut} from '@/lib/types/api'

interface AuthorizedViewersSectionProps {
  orgPk: string
  viewers: AuthorizedViewerOut[]
}

/**
 * SEFAZ allows up to 10 CPF/CNPJ+name pairs authorized to view an
 * organization's NF-e XML (autXML) — this is org-level config, applied
 * automatically to every NF-e that org issues (not a per-emission choice).
 *
 * Rendered as a subsection inside the company form's "dados complementares",
 * not as a card of its own. It used to sit below the form as a peer of it,
 * which read as a second thing to do on the screen — and it is not: it is one
 * more optional fiscal detail of the same company, and almost nobody fills it.
 * Its own writes are immediate, so it is not part of the form's submit.
 */
export function AuthorizedViewersSection({orgPk, viewers}: AuthorizedViewersSectionProps) {
  const qc = useQueryClient()
  const [cpfCnpj, setCpfCnpj] = useState('')
  const [name, setName] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const invalidate = () => qc.invalidateQueries({queryKey: queryKeys.organizations.detail(orgPk)})

  const addMutation = useMutation({
    mutationFn: (data: { cpf_or_cnpj: string; name: string }) => apiClient.addAuthorizedViewer(orgPk, data),
    onSuccess: () => {
      invalidate()
      setCpfCnpj('')
      setName('')
      setFormError(null)
    },
    onError: (err) => setFormError(err instanceof Error ? err.message : 'Erro ao adicionar'),
  })

  const removeMutation = useMutation({
    mutationFn: (viewerCpfCnpj: string) => apiClient.removeAuthorizedViewer(orgPk, viewerCpfCnpj),
    onSuccess: invalidate,
  })

  const atLimit = viewers.length >= MAX_AUTHORIZED_VIEWERS

  const handleAdd = () => {
    setFormError(null)
    const raw = cpfCnpj.replace(/\D/g, '')
    const parsed = authorizedViewerSchema.safeParse({cpf_or_cnpj: raw, name})
    if (!parsed.success) {
      setFormError(parsed.error.issues[0]?.message ?? 'Dados inválidos')
      return
    }
    if (hasDuplicateViewer(viewers, raw)) {
      setFormError('CPF/CNPJ já autorizado')
      return
    }
    addMutation.mutate(parsed.data)
  }

  return (
    <div className="border-t border-gray-200 pt-4">
      <h3 className="mb-1 text-sm font-semibold text-gray-900">Pessoas autorizadas a ver o XML (autXML)</h3>
      <p className="text-[0.8rem] text-gray-500 text-pretty">
        A SEFAZ permite autorizar até {MAX_AUTHORIZED_VIEWERS} pessoas (CPF ou CNPJ) a visualizar o XML
        das NF-e emitidas por esta organização.
      </p>

      {viewers.length > 0 && (
        <ul className="mt-3 space-y-2">
          {viewers.map((v) => (
            <li key={v.cpf_cnpj}
                className="flex items-center justify-between gap-3 rounded-lg bg-gray-50 border border-gray-100 px-3 py-2">
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-900 truncate">{v.name}</p>
                <p className="text-xs text-gray-500">{maskCpfCnpj(v.cpf_cnpj)}</p>
              </div>
              <Button type="button" variant="ghost" size="sm"
                      disabled={removeMutation.isPending && removeMutation.variables === v.cpf_cnpj}
                      onClick={() => removeMutation.mutate(v.cpf_cnpj)}
                      className="shrink-0 min-h-11 sm:min-h-0 text-gray-400 hover:text-red-500 hover:bg-red-50">
                {removeMutation.isPending && removeMutation.variables === v.cpf_cnpj ? 'Removendo...' : 'Remover'}
              </Button>
            </li>
          ))}
        </ul>
      )}

      <div className="mt-4 flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Adicionar ({viewers.length}/{MAX_AUTHORIZED_VIEWERS})
        </p>
      </div>

      {formError && (
        <div className="mt-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
          {formError}
        </div>
      )}

      <div className="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Input placeholder="CPF ou CNPJ" value={maskCpfCnpj(cpfCnpj)} maxLength={18} disabled={atLimit}
               onChange={(e) => setCpfCnpj(e.target.value.replace(/\D/g, '').slice(0, 14))}/>
        <Input placeholder="Nome" value={name} maxLength={60} disabled={atLimit}
               onChange={(e) => setName(e.target.value)}/>
      </div>
      <Button type="button" variant="outline" size="sm" onClick={handleAdd}
              disabled={atLimit || addMutation.isPending || !cpfCnpj || !name}
              className="mt-2 min-h-11 sm:min-h-0">
        {addMutation.isPending ? 'Adicionando...' : atLimit ? 'Limite atingido' : 'Adicionar'}
      </Button>
    </div>
  )
}
