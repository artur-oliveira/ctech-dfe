'use client'

import {useState} from 'react'
import {useForm} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {OptionsSelect} from '@/components/ui/options-select'
import {ApiError} from '@/lib/api/client'
import {CARD_BAND_OPTIONS} from '@/components/nfe/PaymentCardFields'
import {
  type PaymentTerminalFormData,
  paymentTerminalSchema,
} from '@/lib/schemas/payment-terminals'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import {formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'
import type {PaymentTerminalCreate, PaymentTerminalItemOut} from '@/lib/types/api'

const EMPTY: PaymentTerminalFormData = {
  name: '', cnpj_receb: '', id_term_pag: '', cnpj_pag: '', uf_pag: '', t_band: '',
}

export interface PaymentTerminalFormProps {
  initialData?: PaymentTerminalItemOut
  onSubmit: (data: PaymentTerminalCreate) => Promise<void>
  loading?: boolean
}

function toFormData(t: PaymentTerminalItemOut): PaymentTerminalFormData {
  const str = (v: unknown) => (typeof v === 'string' ? v : '')
  return {
    name: t.name,
    cnpj_receb: str(t.cnpj_receb),
    id_term_pag: str(t.id_term_pag),
    cnpj_pag: str(t.cnpj_pag),
    uf_pag: str(t.uf_pag),
    t_band: str(t.t_band),
  }
}

/** Campo vazio vira null: um "" gravado é um default silenciosamente vazio. */
const nullify = (v: string | undefined) => (v ? v : null)

export function PaymentTerminalForm({initialData, onSubmit, loading}: PaymentTerminalFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<PaymentTerminalFormData>({
    resolver: zodResolver(paymentTerminalSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })

  const handleSubmit = async (data: PaymentTerminalFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        cnpj_receb: unformatCpfCnpj(data.cnpj_receb),
        id_term_pag: data.id_term_pag,
        cnpj_pag: data.cnpj_pag ? unformatCpfCnpj(data.cnpj_pag) : null,
        uf_pag: nullify(data.uf_pag),
        t_band: nullify(data.t_band),
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o terminal.')
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FormField control={form.control} name="name"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Nome *</FormLabel>
                         <Input {...field} id={field.name} maxLength={120} className="w-full"
                                placeholder="POS Caixa 1"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="id_term_pag"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Identificador do terminal *</FormLabel>
                         <Input {...field} id={field.name} maxLength={40} className="w-full"
                                placeholder="Atribuído pela adquirente"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="cnpj_receb"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>CNPJ recebedor *</FormLabel>
                         <Input id={field.name} maxLength={18} className="w-full"
                                placeholder="00.000.000/0000-00"
                                value={formatCpfCnpj(field.value)}
                                onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                onBlur={field.onBlur} ref={field.ref}/>
                         <p className="text-xs text-gray-500">
                           Estabelecimento credenciado que recebe o pagamento (<code>card/CNPJReceb</code>).
                         </p>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="t_band"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Bandeira padrão</FormLabel>
                         <OptionsSelect id={field.name} value={field.value ?? ''}
                                        onValueChange={field.onChange}
                                        placeholder="Nenhuma"
                                        options={[...CARD_BAND_OPTIONS]}/>
                         <p className="text-xs text-gray-500">A bandeira informada na emissão sempre vence.</p>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="cnpj_pag"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>CNPJ do pagador</FormLabel>
                         <Input id={field.name} maxLength={18} className="w-full"
                                placeholder="Opcional"
                                value={formatCpfCnpj(field.value ?? '')}
                                onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                onBlur={field.onBlur} ref={field.ref}/>
                         <p className="text-xs text-gray-500">
                           Só quando o pagamento ocorre fora do estabelecimento emitente.
                         </p>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="uf_pag"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>UF do pagador</FormLabel>
                         <OptionsSelect id={field.name} value={field.value ?? ''}
                                        onValueChange={field.onChange}
                                        placeholder="Nenhuma"
                                        options={[...UF_OPTIONS]}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
        </div>

        {submitError && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="submit" variant="brand" disabled={loading} className="min-h-11">
            {loading ? 'Salvando…' : 'Salvar'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
