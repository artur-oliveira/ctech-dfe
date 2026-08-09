'use client'

import {useState} from 'react'
import {Button} from '@/components/ui/button'
import {
  EMPTY_TAX_GROUPS, TaxFieldsEditor, type TaxGroups,
} from '@/components/tax/TaxFieldsEditor'
import type {CfopConfigFormData} from '@/lib/schemas/products'
import {UF_OPTIONS} from '@/lib/schemas/entity'

export interface UfOverrideFormData {
  ufs: string[]
  overrides: Partial<CfopConfigFormData>
}

interface UfOverridesEditorProps {
  value: UfOverrideFormData[]
  onChange: (next: UfOverrideFormData[]) => void
  simples: boolean
}

/**
 * Lista de cards de override por UF de destino. Cada card tem um picker
 * multi-select de UF e o mesmo TaxFieldsEditor usado no CFOP/perfil base —
 * todos os campos ficam opcionais aqui: só preenche o que diverge para
 * aquelas UFs (design spec 2026-08-09-tax-config-redesign §Modelo de dados 1).
 */
export function UfOverridesEditor({value, onChange, simples}: UfOverridesEditorProps) {
  const [groupsByIndex, setGroupsByIndex] = useState<Record<number, TaxGroups>>({})

  const addCard = () => onChange([...value, {ufs: [], overrides: {}}])
  const removeCard = (i: number) => onChange(value.filter((_, idx) => idx !== i))
  const setUfs = (i: number, ufs: string[]) =>
    onChange(value.map((v, idx) => (idx === i ? {...v, ufs} : v)))
  const setOverrides = (i: number, updater: (r: CfopConfigFormData) => CfopConfigFormData) =>
    onChange(value.map((v, idx) => {
      if (idx !== i) return v
      const next = updater(v.overrides as CfopConfigFormData)
      return {...v, overrides: next}
    }))

  return (
    <div className="space-y-3">
      {value.map((card, i) => (
        <div key={i} className="rounded-lg border border-purple-100 bg-purple-50/20 p-3 space-y-3">
          <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
            <div className="flex-1">
              <label className="text-sm font-medium text-gray-700">UFs de destino</label>
              <div className="flex flex-wrap gap-1.5 pt-1">
                {UF_OPTIONS.map((opt) => {
                  const checked = card.ufs.includes(opt.value)
                  return (
                    <button key={opt.value} type="button"
                            onClick={() => setUfs(i, checked
                              ? card.ufs.filter((u) => u !== opt.value)
                              : [...card.ufs, opt.value])}
                            className={`min-h-8 rounded-full px-2.5 text-xs font-medium ${
                              checked ? 'bg-purple-600 text-white' : 'bg-white text-gray-600 border border-gray-200'
                            }`}>
                      {opt.value}
                    </button>
                  )
                })}
              </div>
            </div>
            <Button type="button" variant="ghost" size="xs" onClick={() => removeCard(i)}
                    className="self-start text-danger hover:text-red-700">remover</Button>
          </div>
          <TaxFieldsEditor value={card.overrides as CfopConfigFormData}
                            onChange={(updater) => setOverrides(i, updater)}
                            simples={simples} hideCfop
                            groups={groupsByIndex[i] ?? EMPTY_TAX_GROUPS}
                            onGroupsChange={(g) => setGroupsByIndex((prev) => ({...prev, [i]: g}))}/>
        </div>
      ))}
      <Button type="button" variant="ghost" size="sm" onClick={addCard}
              className="text-brand-600 hover:text-brand-700 px-0">
        + Adicionar override por UF
      </Button>
    </div>
  )
}
