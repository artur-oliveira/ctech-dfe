'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {Combobox} from '@/components/ui/combobox'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {ApiError} from '@/lib/api/client'
import {PAYMENT_OPTIONS} from '@/lib/data/payment-options'
import {
  type PaymentTermFormData,
  paymentTermSchema,
  previewInstallments,
} from '@/lib/schemas/payment-terms'
import type {PaymentTermCreate, PaymentTermItemOut} from '@/lib/types/api'

interface PaymentTermFormProps {
  initialData?: PaymentTermItemOut
  onSubmit: (data: PaymentTermCreate) => Promise<void>
  loading?: boolean
}

/** Total de exemplo da pré-visualização — só para o usuário ver a divisão. */
const PREVIEW_TOTAL = 1000

const IND_PAG_OPTIONS = [
  {value: '', label: 'Derivar das parcelas'},
  {value: '0', label: '0 – À vista'},
  {value: '1', label: '1 – A prazo'},
]

const EMPTY: PaymentTermFormData = {
  name: '', payment_type: '', ind_pag: '', installments: 1, interval_days: 30, first_due_days: 30,
}

export function PaymentTermForm({initialData, onSubmit, loading = false}: PaymentTermFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm<PaymentTermFormData>({
    resolver: zodResolver(paymentTermSchema),
    defaultValues: initialData
      ? {
        name: initialData.name,
        payment_type: initialData.payment_type,
        ind_pag: (initialData.ind_pag ?? '') as PaymentTermFormData['ind_pag'],
        installments: initialData.installments,
        interval_days: initialData.interval_days ?? 30,
        first_due_days: initialData.first_due_days ?? 30,
      }
      : EMPTY,
  })

  const values = useWatch({control: form.control}) as PaymentTermFormData
  const preview = previewInstallments(
    {
      installments: values.installments ?? 1,
      interval_days: values.interval_days ?? 0,
      first_due_days: values.first_due_days ?? 0,
    },
    PREVIEW_TOTAL,
    new Date(),
  )

  const handleSubmit = async (data: PaymentTermFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({...data, ind_pag: data.ind_pag || null} as unknown as PaymentTermCreate)
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a condição.')
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField control={form.control} name="name"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Nome *</FormLabel>
                           <Input {...field} id={field.name} maxLength={120} className="w-full"
                                  placeholder="30/60/90"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="payment_type"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Forma de pagamento *</FormLabel>
                           <Combobox value={field.value} onValueChange={field.onChange}
                                     options={PAYMENT_OPTIONS} placeholder="Forma de pagamento"
                                     searchPlaceholder="Código ou descrição..."/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <FormField control={form.control} name="installments"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Parcelas *</FormLabel>
                           <NumericInput value={String(field.value ?? 1)}
                                         onChange={(v) => field.onChange(parseInt(v || '1', 10))}
                                         placeholder="3"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="first_due_days"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Dias até a 1ª</FormLabel>
                           <NumericInput value={String(field.value ?? 0)}
                                         onChange={(v) => field.onChange(parseInt(v || '0', 10))}
                                         placeholder="30"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="interval_days"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Intervalo (dias)</FormLabel>
                           <NumericInput value={String(field.value ?? 0)}
                                         onChange={(v) => field.onChange(parseInt(v || '0', 10))}
                                         placeholder="30"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="ind_pag"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Indicador</FormLabel>
                           <OptionsSelect id={field.name} value={field.value ?? ''}
                                          onValueChange={field.onChange} options={IND_PAG_OPTIONS}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        </div>

        {/* Pré-visualização: sem ela, "30/60/90 com 3 parcelas" é uma aposta. */}
        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
            Parcelas geradas para R$ {PREVIEW_TOTAL.toFixed(2).replace('.', ',')}
          </p>
          <div className="space-y-1">
            {preview.map((inst) => (
              <div key={inst.number}
                   className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                <span className="font-mono text-xs text-gray-500">{inst.number}</span>
                <span className="text-gray-700">{inst.dueDate}</span>
                <span className="font-medium text-gray-900">R$ {inst.value.replace('.', ',')}</span>
              </div>
            ))}
          </div>
          <p className="text-xs text-gray-500">
            A última parcela absorve o resíduo do arredondamento — a soma sempre fecha com o total da nota.
          </p>
        </div>

        {submitError && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="submit" variant="brand" disabled={loading} className="min-h-11">
            {loading ? 'Salvando…' : 'Salvar condição'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
