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
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {MdfeTollIn} from '@/lib/types/api'

export interface TollVouchersFieldsProps {
  vouchers: MdfeTollIn[]
  onChange: (vouchers: MdfeTollIn[]) => void
}

/**
 * Vales-pedágio da viagem (infANTT/valePed). O CNPJ da fornecedora e do pagador
 * vivem no cadastro; aqui entra só o que muda a cada viagem — qual fornecedora,
 * o número da compra e o valor. A categoria da combinação veicular é derivada
 * do número de reboques pelo backend, nunca perguntada.
 */
export function TollVouchersFields({vouchers, onChange}: TollVouchersFieldsProps) {
  const {selectedOrg} = useAuth()

  const {data: page} = useQuery({
    queryKey: queryKeys.tollProviders.list(selectedOrg?.pk),
    queryFn: () => apiClient.getTollProviders({limit: 100}),
    enabled: !!selectedOrg,
  })
  const providers = page?.items ?? []

  const patch = (i: number, p: Partial<MdfeTollIn>) =>
    onChange(vouchers.map((v, k) => (k === i ? {...v, ...p} : v)))

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Vale-pedágio (opcional)</p>
        <Button type="button" variant="ghost" size="xs" disabled={providers.length === 0}
                onClick={() => onChange([...vouchers, {toll_provider_id: '', n_compra: '', v_vale_ped: ''}])}>
          + Vale
        </Button>
      </div>

      {providers.length === 0 ? (
        <p className="text-xs text-gray-500">
          Cadastre uma fornecedora em <span className="font-medium">Cadastros → Vale-pedágio</span> para
          informar vales aqui.
        </p>
      ) : (
        vouchers.length === 0 && (
          <p className="text-xs text-gray-500">
            Obrigatório no transporte rodoviário de carga por conta de terceiros (Lei 10.209).
          </p>
        )
      )}

      {vouchers.map((v, i) => (
        <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
          <div className="flex flex-col gap-1">
            <Label htmlFor={`toll-provider-${i}`} className="text-xs font-medium text-gray-600">Fornecedora</Label>
            <OptionsSelect id={`toll-provider-${i}`} value={v.toll_provider_id}
                           placeholder="Selecione"
                           onValueChange={(id: string) => patch(i, {toll_provider_id: id})}
                           options={providers.map((p) => ({
                             value: extractId(p.sk, SK_PREFIX.TOLL_PROVIDER),
                             label: p.name,
                           }))}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`toll-ncompra-${i}`} className="text-xs font-medium text-gray-600">Nº da compra</Label>
            <Input id={`toll-ncompra-${i}`} maxLength={20} value={v.n_compra} className="w-full"
                   onChange={(e) => patch(i, {n_compra: e.target.value})}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`toll-valor-${i}`} className="text-xs font-medium text-gray-600">Valor</Label>
            <CurrencyInput id={`toll-valor-${i}`} value={v.v_vale_ped} className="w-full"
                           onChange={(value) => patch(i, {v_vale_ped: value})}/>
          </div>
          <Button type="button" variant="ghost" size="xs"
                  onClick={() => onChange(vouchers.filter((_, k) => k !== i))}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}
