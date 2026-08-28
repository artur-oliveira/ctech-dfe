'use client'

import {useQuery} from '@tanstack/react-query'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {OptionsSelect} from '@/components/ui/options-select'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'

export interface AccessKeyPickerProps {
  id: string
  label: string
  /** Chaves de 44 dígitos já escolhidas. */
  value: string[]
  onChange: (keys: string[]) => void
  /** Texto de apoio: normalmente a regra do leiaute que limita a escolha. */
  hint?: string
  /** Limite de chaves. Alcançado, o seletor fica desabilitado. */
  max?: number
}

/** Quantas notas recentes o seletor oferece. */
const RECENT_LIMIT = 50

/**
 * Escolha de chaves de acesso entre as notas da própria organização.
 *
 * Digitar 44 dígitos à mão é a forma mais fácil de errar uma chave, e os dois
 * campos que usam este componente (`refDFeAnt` da compra governamental e
 * `gPagAntecipado/refNFe`) exigem documentos do **mesmo CNPJ base** — ou seja,
 * notas que este sistema emitiu. Então o campo é um seletor, não um input.
 */
export function AccessKeyPicker({id, label, value, onChange, hint, max}: AccessKeyPickerProps) {
  const {selectedOrg} = useAuth()

  const {data: nfePage, isLoading} = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, {limit: RECENT_LIMIT}),
    queryFn: () => apiClient.getNfes({limit: RECENT_LIMIT}),
    enabled: !!selectedOrg,
  })
  const nfes = nfePage?.items ?? []

  const atMax = max !== undefined && value.length >= max
  // Uma chave já escolhida não aparece de novo na lista.
  const options = nfes
    .filter((n) => !value.includes(n.sk))
    .map((n) => ({
      value: n.sk,
      label: `${n.number ?? ''} · ${n.dest_name ?? ''} · ${n.sk}`,
    }))

  return (
    <div className="space-y-2">
      <Label htmlFor={id} className="text-sm font-medium text-gray-700">{label}</Label>
      {hint && <p className="text-xs text-gray-500">{hint}</p>}

      {value.length > 0 && (
        <ul className="space-y-1">
          {value.map((key) => (
            <li key={key}
                className="flex items-center justify-between gap-2 rounded border border-gray-200 px-2 py-1 text-sm">
              <span className="truncate font-mono text-xs">{key}</span>
              <Button type="button" variant="ghost" size="xs" className="min-h-11 sm:min-h-0"
                      onClick={() => onChange(value.filter((k) => k !== key))}>
                Remover
              </Button>
            </li>
          ))}
        </ul>
      )}

      <OptionsSelect
        id={id}
        value=""
        disabled={atMax || isLoading}
        placeholder={atMax ? 'Limite alcançado' : isLoading ? 'Carregando notas…' : 'Selecione uma nota emitida…'}
        options={options}
        onValueChange={(v: string) => {
          if (v) onChange([...value, v])
        }}
      />
    </div>
  )
}
