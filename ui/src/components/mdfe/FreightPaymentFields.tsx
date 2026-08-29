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
import type {MdfePaymentIn} from '@/lib/types/api'

/** Componentes do valor do frete (`infPag/Comp/tpComp`). */
const COMPONENT_OPTIONS = [
  {value: '01', label: '01 – Vale-pedágio'},
  {value: '02', label: '02 – Impostos, taxas e contribuições'},
  {value: '03', label: '03 – Despesas bancárias'},
  {value: '04', label: '04 – Diárias'},
  {value: '99', label: '99 – Outros'},
]

const PAYMENT_TYPE_OPTIONS = [
  {value: '0', label: 'À vista'},
  {value: '1', label: 'A prazo'},
]

const COMPONENT_OTHERS = '99'
const PAYMENT_TERM = '1'

export interface FreightPaymentFieldsProps {
  payments: MdfePaymentIn[]
  onChange: (payments: MdfePaymentIn[]) => void
  /** Contratante declarado obriga o grupo de pagamento (regra do backend). */
  required: boolean
}

/**
 * Pagamento ao transportador autônomo (`infANTT/infPag`). Nome, documento e
 * dados de recebimento vêm do cadastro da pessoa; as parcelas são derivadas do
 * prazo escolhido — a tela nunca pede parcela por parcela.
 */
export function FreightPaymentFields({payments, onChange, required}: FreightPaymentFieldsProps) {
  const {selectedOrg} = useAuth()

  const {data: page} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk, 'driver'),
    queryFn: () => apiClient.getPersons({role: 'driver', limit: 100}),
    enabled: !!selectedOrg,
  })
  const persons = page?.items ?? []

  const patch = (i: number, p: Partial<MdfePaymentIn>) =>
    onChange(payments.map((v, k) => (k === i ? {...v, ...p} : v)))

  const addPayment = () => onChange([...payments, {
    person_doc: '',
    components: [{type: '01', value: ''}],
    contract_value: '',
    payment_type: '0',
  }])

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Pagamento do frete {required ? '(obrigatório)' : '(opcional)'}
        </p>
        <Button type="button" variant="ghost" size="xs" disabled={persons.length === 0} onClick={addPayment}>
          + Pagamento
        </Button>
      </div>

      {persons.length === 0 ? (
        <p className="text-xs text-gray-500">
          Cadastre o condutor/TAC em <span className="font-medium">Cadastros → Pessoas</span> com chave
          PIX, banco ou CNPJ da instituição de pagamento para informar o pagamento aqui.
        </p>
      ) : (
        required && payments.length === 0 && (
          <p className="text-xs text-red-600">
            O MDF-e com contratante do frete exige ao menos um pagamento declarado.
          </p>
        )
      )}

      {payments.map((pay, i) => (
        <div key={i} className="rounded-lg border border-gray-100 p-3 space-y-2">
          <div className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
            <div className="flex flex-col gap-1">
              <Label htmlFor={`pay-person-${i}`} className="text-xs font-medium text-gray-600">Beneficiário</Label>
              <OptionsSelect id={`pay-person-${i}`} value={pay.person_doc} placeholder="Selecione"
                             onValueChange={(doc: string) => patch(i, {person_doc: doc})}
                             options={persons.map((p) => ({
                               value: p.sk.startsWith(SK_PREFIX.FOREIGN) ? p.sk : unformatCpfCnpj(p.sk),
                               label: p.name,
                             }))}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor={`pay-value-${i}`} className="text-xs font-medium text-gray-600">Valor do contrato</Label>
              <CurrencyInput id={`pay-value-${i}`} value={pay.contract_value} className="w-full"
                             onChange={(value) => patch(i, {contract_value: value})}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor={`pay-type-${i}`} className="text-xs font-medium text-gray-600">Pagamento</Label>
              <OptionsSelect id={`pay-type-${i}`} value={pay.payment_type} options={PAYMENT_TYPE_OPTIONS}
                             onValueChange={(v: string) => patch(i, {payment_type: v})}/>
            </div>
            <Button type="button" variant="ghost" size="xs"
                    onClick={() => onChange(payments.filter((_, k) => k !== i))}>
              Remover
            </Button>
          </div>

          {pay.payment_type === PAYMENT_TERM && (
            <div className="grid grid-cols-1 sm:grid-cols-4 gap-2 items-end">
              <div className="flex flex-col gap-1">
                <Label htmlFor={`pay-adiant-${i}`} className="text-xs font-medium text-gray-600">Adiantamento</Label>
                <CurrencyInput id={`pay-adiant-${i}`} value={pay.advance_value ?? ''} className="w-full"
                               onChange={(value) => patch(i, {advance_value: value})}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor={`pay-inst-${i}`} className="text-xs font-medium text-gray-600">Parcelas</Label>
                <Input id={`pay-inst-${i}`} inputMode="numeric" value={pay.installments ?? ''}
                       onChange={(e) => patch(i, {installments: Number(e.target.value.replace(/\D/g, '')) || undefined})}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor={`pay-first-${i}`} className="text-xs font-medium text-gray-600">1º vencimento
                  (dias)</Label>
                <Input id={`pay-first-${i}`} inputMode="numeric" value={pay.first_due_days ?? ''}
                       onChange={(e) => patch(i, {first_due_days: Number(e.target.value.replace(/\D/g, '')) || undefined})}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor={`pay-interval-${i}`} className="text-xs font-medium text-gray-600">Intervalo
                  (dias)</Label>
                <Input id={`pay-interval-${i}`} inputMode="numeric" value={pay.interval_days ?? ''}
                       onChange={(e) => patch(i, {interval_days: Number(e.target.value.replace(/\D/g, '')) || undefined})}/>
              </div>
            </div>
          )}

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <p className="text-xs font-medium text-gray-600">Componentes do frete</p>
              <Button type="button" variant="ghost" size="xs"
                      onClick={() => patch(i, {components: [...pay.components, {type: '01', value: ''}]})}>
                + Componente
              </Button>
            </div>
            {pay.components.map((comp, ci) => (
              <div key={ci}
                   className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`comp-type-${i}-${ci}`} className="text-xs font-medium text-gray-600">Tipo</Label>
                  <OptionsSelect id={`comp-type-${i}-${ci}`} value={comp.type} options={COMPONENT_OPTIONS}
                                 onValueChange={(v: string) => patch(i, {
                                   components: pay.components.map((c, k) => (k === ci ? {...c, type: v} : c)),
                                 })}/>
                </div>
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`comp-value-${i}-${ci}`} className="text-xs font-medium text-gray-600">Valor</Label>
                  <CurrencyInput id={`comp-value-${i}-${ci}`} value={comp.value} className="w-full"
                                 onChange={(value) => patch(i, {
                                   components: pay.components.map((c, k) => (k === ci ? {...c, value} : c)),
                                 })}/>
                </div>
                {comp.type === COMPONENT_OTHERS && (
                  <div className="flex flex-col gap-1">
                    <Label htmlFor={`comp-desc-${i}-${ci}`}
                           className="text-xs font-medium text-gray-600">Descrição</Label>
                    <Input id={`comp-desc-${i}-${ci}`} maxLength={60} value={comp.description ?? ''}
                           onChange={(e) => patch(i, {
                             components: pay.components.map((c, k) => (
                               k === ci ? {...c, description: e.target.value} : c)),
                           })}/>
                  </div>
                )}
                <Button type="button" variant="ghost" size="xs" disabled={pay.components.length === 1}
                        onClick={() => patch(i, {components: pay.components.filter((_, k) => k !== ci)})}>
                  Remover
                </Button>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
