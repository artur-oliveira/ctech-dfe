'use client'

import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {OptionsSelect} from '@/components/ui/options-select'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {
  NFE_REF_KEY_KINDS,
  NFE_REF_KIND_LABELS,
  NFE_REF_KINDS,
  type NfeRefKind,
} from '@/lib/schemas/nfe-refs'
import type {NfeRefIn} from '@/lib/types/api'

export interface NfeRefsPickerProps {
  value: NfeRefIn[]
  onChange: (refs: NfeRefIn[]) => void
}

const EMPTY_EXTERNAL: NfeRefIn = {kind: 'nf'}

/** Rótulo curto de uma referência já adicionada. */
function describe(ref: NfeRefIn): string {
  if (ref.nfe_id) return `Nota da base · ${ref.nfe_id}`
  if (ref.access_key) return `${NFE_REF_KIND_LABELS[ref.kind as NfeRefKind]} · ${ref.access_key}`
  if (ref.kind === 'ecf') return `Cupom fiscal · ECF ${ref.n_ecf} COO ${ref.n_coo}`
  return `${NFE_REF_KIND_LABELS[ref.kind as NfeRefKind]} · nº ${ref.n_nf ?? ''}`
}

/**
 * Seleção dos documentos referenciados (ide/NFref). O caminho normal é escolher
 * uma nota da própria base — chave e tipo saem do registro; o formulário de
 * documento externo só existe para o que o sistema nunca emitiu.
 */
export function NfeRefsPicker({value, onChange}: NfeRefsPickerProps) {
  const {selectedOrg} = useAuth()
  const [external, setExternal] = useState<NfeRefIn | null>(null)

  const {data: nfePage} = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, {limit: 50}),
    queryFn: () => apiClient.getNfes({limit: 50}),
    enabled: !!selectedOrg,
  })
  const nfes = nfePage?.items ?? []

  const add = (ref: NfeRefIn) => onChange([...value, ref])
  const removeAt = (i: number) => onChange(value.filter((_, k) => k !== i))
  const patchExternal = (patch: Partial<NfeRefIn>) =>
    setExternal((cur) => ({...(cur ?? EMPTY_EXTERNAL), ...patch}))

  const externalKind = (external?.kind ?? 'nf') as NfeRefKind
  const externalIsKeyOnly = NFE_REF_KEY_KINDS.includes(externalKind)

  return (
    <div className="space-y-3">
      {value.length > 0 && (
        <ul className="space-y-1">
          {value.map((ref, i) => (
            <li key={`${ref.nfe_id ?? ref.access_key ?? ref.n_nf ?? ''}-${i}`}
                className="flex items-center justify-between rounded border border-gray-200 px-2 py-1 text-sm">
              <span className="truncate">{describe(ref)}</span>
              <Button type="button" variant="ghost" size="xs" onClick={() => removeAt(i)}>
                Remover
              </Button>
            </li>
          ))}
        </ul>
      )}

      <div>
        <Label htmlFor="nfe-ref-from-base">Nota desta empresa</Label>
        <OptionsSelect
          id="nfe-ref-from-base"
          value=""
          placeholder="Selecione uma nota emitida…"
          onValueChange={(v: string) => {
            if (v) add({nfe_id: v})
          }}
          options={[
            ...nfes.map((n) => ({
              value: n.sk,
              label: `${n.number ?? ''} · ${n.dest_name ?? ''} · ${n.sk}`,
            })),
          ]}
        />
      </div>

      {external === null ? (
        <Button type="button" variant="ghost" size="xs" className="px-0 text-brand-600 hover:text-brand-700"
                onClick={() => setExternal(EMPTY_EXTERNAL)}>
          + Documento de fora do sistema
        </Button>
      ) : (
        <div className="space-y-2 rounded border border-gray-200 p-2">
          <div>
            <Label htmlFor="nfe-ref-kind">Tipo do documento</Label>
            <OptionsSelect
              id="nfe-ref-kind"
              value={externalKind}
              onValueChange={(v: string) => setExternal({kind: v as NfeRefKind})}
              options={NFE_REF_KINDS.map((k) => ({value: k, label: NFE_REF_KIND_LABELS[k]}))}
            />
          </div>

          {externalIsKeyOnly && (
            <div>
              <Label htmlFor="nfe-ref-key">Chave de acesso</Label>
              <Input id="nfe-ref-key" inputMode="numeric" maxLength={44}
                     value={external.access_key ?? ''}
                     onChange={(e) => patchExternal({access_key: e.target.value})}/>
            </div>
          )}

          {(externalKind === 'nf' || externalKind === 'nfp') && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div>
                <Label htmlFor="nfe-ref-cuf">cUF</Label>
                <Input id="nfe-ref-cuf" inputMode="numeric" maxLength={2}
                       value={external.c_uf ?? ''} onChange={(e) => patchExternal({c_uf: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor="nfe-ref-aamm">AAMM</Label>
                <Input id="nfe-ref-aamm" inputMode="numeric" maxLength={4}
                       value={external.aamm ?? ''} onChange={(e) => patchExternal({aamm: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor="nfe-ref-cnpj">CNPJ</Label>
                <Input id="nfe-ref-cnpj" inputMode="numeric" maxLength={14}
                       value={external.cnpj ?? ''} onChange={(e) => patchExternal({cnpj: e.target.value})}/>
              </div>
              {externalKind === 'nfp' && (
                <>
                  <div>
                    <Label htmlFor="nfe-ref-cpf">CPF (se produtor pessoa física)</Label>
                    <Input id="nfe-ref-cpf" inputMode="numeric" maxLength={11}
                           value={external.cpf ?? ''} onChange={(e) => patchExternal({cpf: e.target.value})}/>
                  </div>
                  <div>
                    <Label htmlFor="nfe-ref-ie">Inscrição estadual</Label>
                    <Input id="nfe-ref-ie" maxLength={14}
                           value={external.ie ?? ''} onChange={(e) => patchExternal({ie: e.target.value})}/>
                  </div>
                </>
              )}
              <div>
                <Label htmlFor="nfe-ref-mod">Modelo</Label>
                <Input id="nfe-ref-mod" maxLength={2}
                       value={external.mod ?? ''} onChange={(e) => patchExternal({mod: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor="nfe-ref-serie">Série</Label>
                <Input id="nfe-ref-serie" inputMode="numeric" maxLength={3}
                       value={external.serie ?? ''} onChange={(e) => patchExternal({serie: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor="nfe-ref-nnf">Número</Label>
                <Input id="nfe-ref-nnf" inputMode="numeric" maxLength={9}
                       value={external.n_nf ?? ''} onChange={(e) => patchExternal({n_nf: e.target.value})}/>
              </div>
            </div>
          )}

          {externalKind === 'ecf' && (
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <div>
                <Label htmlFor="nfe-ref-ecf-mod">Modelo</Label>
                <Input id="nfe-ref-ecf-mod" maxLength={2}
                       value={external.mod ?? ''} onChange={(e) => patchExternal({mod: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor="nfe-ref-necf">nECF</Label>
                <Input id="nfe-ref-necf" inputMode="numeric" maxLength={3}
                       value={external.n_ecf ?? ''} onChange={(e) => patchExternal({n_ecf: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor="nfe-ref-ncoo">nCOO</Label>
                <Input id="nfe-ref-ncoo" inputMode="numeric" maxLength={6}
                       value={external.n_coo ?? ''} onChange={(e) => patchExternal({n_coo: e.target.value})}/>
              </div>
            </div>
          )}

          <div className="flex gap-2">
            <Button type="button" size="xs" onClick={() => {
              add(external)
              setExternal(null)
            }}>
              Adicionar
            </Button>
            <Button type="button" variant="ghost" size="xs" onClick={() => setExternal(null)}>
              Cancelar
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
