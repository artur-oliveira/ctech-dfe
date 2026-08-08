'use client'

import {useState} from 'react'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import type {NfeLocalIn, NfeLocalOut} from '@/lib/types/api'

export interface LocationPickerProps {
  label: string
  savedLocations: NfeLocalOut[]
  value: NfeLocalIn | null
  onChange: (loc: NfeLocalIn | null) => void
  save: boolean
  onSaveChange: (save: boolean) => void
}

const EMPTY_LOCAL: NfeLocalIn = {
  x_lgr: '', nro: '', x_cpl: '', x_bairro: '', c_mun: '', x_mun: '', uf: 'SP',
}

function localsMatch(saved: NfeLocalOut, current: NfeLocalIn | null): boolean {
  if (!current) return false
  return saved.x_lgr === current.x_lgr && saved.nro === current.nro
}

/** Free-form entrega/retirada address for NF-e emission, with a picker over
 * locations saved from previous emissions (see delivery_locations /
 * pickup_locations) so the user isn't retyping the same address every time. */
export function LocationPicker({label, savedLocations, value, onChange, save, onSaveChange}: LocationPickerProps) {
  const [open, setOpen] = useState(false)
  const [manual, setManual] = useState(false)

  if (!open) {
    return (
      <Button type="button" variant="ghost" size="xs" onClick={() => setOpen(true)}
              className="gap-1.5 text-brand-600 hover:text-brand-700 px-0">
        + {label}
      </Button>
    )
  }

  const set = (patch: Partial<NfeLocalIn>) => onChange({...(value ?? EMPTY_LOCAL), ...patch})
  const showManualForm = manual || savedLocations.length === 0

  return (
    <div className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-4">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-gray-600">{label}</p>
        <Button type="button" variant="ghost" size="xs"
                onClick={() => {
                  setOpen(false)
                  setManual(false)
                  onChange(null)
                  onSaveChange(false)
                }}
                className="text-gray-500 hover:text-danger hover:bg-red-50">
          Remover
        </Button>
      </div>

      {!manual && savedLocations.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs text-gray-500">Locais salvos</p>
          <div className="flex flex-wrap gap-2">
            {savedLocations.map((loc, i) => (
              <button
                key={i}
                type="button"
                onClick={() => onChange({
                  cnpj: loc.cnpj, cpf: loc.cpf, x_nome: loc.x_nome,
                  x_lgr: loc.x_lgr, nro: loc.nro, x_cpl: loc.x_cpl,
                  x_bairro: loc.x_bairro, c_mun: loc.c_mun, x_mun: loc.x_mun,
                  uf: loc.uf, fone: loc.fone, email: loc.email,
                })}
                className={[
                  'min-h-11 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors sm:min-h-0',
                  localsMatch(loc, value)
                    ? 'border-brand-400 bg-brand-50 text-brand-700'
                    : 'border-gray-200 bg-white text-gray-700 hover:border-brand-300',
                ].join(' ')}
              >
                {loc.x_lgr}, {loc.nro}
              </button>
            ))}
          </div>
          <Button type="button" variant="ghost" size="xs" onClick={() => setManual(true)}
                  className="text-brand-600 hover:text-brand-700 px-0">
            + Endereço diferente
          </Button>
        </div>
      )}

      {showManualForm && (
        <div className="space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <Input placeholder="Logradouro" value={value?.x_lgr ?? ''} className="sm:col-span-2"
                   onChange={(e) => set({x_lgr: e.target.value})}/>
            <Input placeholder="Número" value={value?.nro ?? ''}
                   onChange={(e) => set({nro: e.target.value})}/>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Input placeholder="Complemento" value={value?.x_cpl ?? ''}
                   onChange={(e) => set({x_cpl: e.target.value})}/>
            <Input placeholder="Bairro" value={value?.x_bairro ?? ''}
                   onChange={(e) => set({x_bairro: e.target.value})}/>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <Input placeholder="Cidade" value={value?.x_mun ?? ''} className="sm:col-span-2"
                   onChange={(e) => set({x_mun: e.target.value})}/>
            <OptionsSelect value={value?.uf ?? 'SP'} onValueChange={(uf) => set({uf})} options={UF_OPTIONS}/>
          </div>
          <NumericInput placeholder="Código IBGE do município" value={value?.c_mun ?? ''} maxLength={7}
                        onChange={(c_mun) => set({c_mun})}/>
          <label className="flex min-h-11 items-center gap-2 text-xs text-gray-600 sm:min-h-0">
            <input type="checkbox" checked={save} onChange={(e) => onSaveChange(e.target.checked)}
                   className="rounded border-gray-300"/>
            Salvar este local para reutilizar
          </label>
        </div>
      )}
    </div>
  )
}
