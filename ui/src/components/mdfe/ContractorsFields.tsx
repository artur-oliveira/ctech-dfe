'use client'

import {useQuery} from '@tanstack/react-query'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {Input} from '@/components/ui/input'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {SK_PREFIX} from '@/lib/constants/entity-keys'
import {unformatCpfCnpj} from '@/lib/utils/document'
import type {MdfeContractorIn} from '@/lib/types/api'

export interface ContractorsFieldsProps {
  contractors: MdfeContractorIn[]
  onChange: (contractors: MdfeContractorIn[]) => void
}

/**
 * Contratantes do frete (infANTT/infContratante). Nome e documento vêm do
 * cadastro de pessoas com o papel "Contratante de frete"; da viagem sai só o
 * contrato — número e valor global.
 */
export function ContractorsFields({contractors, onChange}: ContractorsFieldsProps) {
  const {selectedOrg} = useAuth()

  const {data: page} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk, 'freight_contractor'),
    queryFn: () => apiClient.getPersons({role: 'freight_contractor', limit: 100}),
    enabled: !!selectedOrg,
  })
  const persons = page?.items ?? []

  const patch = (i: number, p: Partial<MdfeContractorIn>) =>
    onChange(contractors.map((c, k) => (k === i ? {...c, ...p} : c)))

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Contratante do frete
          (opcional)</p>
        <Button type="button" variant="ghost" size="xs" disabled={persons.length === 0}
                onClick={() => onChange([...contractors, {person_doc: '', contract_number: '', contract_value: ''}])}>
          + Contratante
        </Button>
      </div>

      {persons.length === 0 && (
        <p className="text-xs text-gray-500">
          Marque o papel <span className="font-medium">Contratante de frete</span> numa pessoa em
          <span className="font-medium"> Cadastros → Pessoas</span> para informá-la aqui.
        </p>
      )}

      {contractors.map((c, i) => (
        <div key={i}
             className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
          <div className="flex flex-col gap-1">
            <Label htmlFor={`contractor-person-${i}`} className="text-xs font-medium text-gray-600">Pessoa</Label>
            <OptionsSelect id={`contractor-person-${i}`} value={c.person_doc}
                           placeholder="Selecione"
                           onValueChange={(doc: string) => patch(i, {person_doc: doc})}
                           options={persons.map((p) => ({
                             value: p.sk.startsWith(SK_PREFIX.FOREIGN) ? p.sk : unformatCpfCnpj(p.sk),
                             label: p.name,
                           }))}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`contractor-num-${i}`} className="text-xs font-medium text-gray-600">Nº do contrato</Label>
            <Input id={`contractor-num-${i}`} maxLength={20} value={c.contract_number ?? ''} className="w-full"
                   onChange={(e) => patch(i, {contract_number: e.target.value})}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`contractor-value-${i}`} className="text-xs font-medium text-gray-600">Valor global</Label>
            <CurrencyInput id={`contractor-value-${i}`} value={c.contract_value ?? ''} className="w-full"
                           onChange={(value) => patch(i, {contract_value: value})}/>
          </div>
          <Button type="button" variant="ghost" size="xs"
                  onClick={() => onChange(contractors.filter((_, k) => k !== i))}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}
