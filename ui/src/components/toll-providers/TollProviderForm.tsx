'use client'

import {useState} from 'react'
import {useForm} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {OptionsSelect} from '@/components/ui/options-select'
import {ApiError} from '@/lib/api/client'
import {
  TP_VALE_PED_OPTIONS,
  type TollProviderFormData,
  tollProviderSchema,
} from '@/lib/schemas/toll-providers'
import {formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'
import type {TollProviderCreate, TollProviderItemOut} from '@/lib/types/api'

const EMPTY: TollProviderFormData = {
  name: '', cnpj_forn: '', cnpj_pg: '', cpf_pg: '', tp_vale_ped: '',
}

export interface TollProviderFormProps {
  initialData?: TollProviderItemOut
  onSubmit: (data: TollProviderCreate) => Promise<void>
  loading?: boolean
}

function toFormData(t: TollProviderItemOut): TollProviderFormData {
  const str = (v: unknown) => (typeof v === 'string' ? v : '')
  return {
    name: t.name,
    cnpj_forn: str(t.cnpj_forn),
    cnpj_pg: str(t.cnpj_pg),
    cpf_pg: str(t.cpf_pg),
    tp_vale_ped: str(t.tp_vale_ped) as TollProviderFormData['tp_vale_ped'],
  }
}

/** Campo vazio vira null: um "" gravado é um default silenciosamente vazio. */
const nullify = (v: string | undefined) => (v ? v : null)

export function TollProviderForm({initialData, onSubmit, loading}: TollProviderFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<TollProviderFormData>({
    resolver: zodResolver(tollProviderSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })

  const handleSubmit = async (data: TollProviderFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        cnpj_forn: unformatCpfCnpj(data.cnpj_forn),
        cnpj_pg: data.cnpj_pg ? unformatCpfCnpj(data.cnpj_pg) : null,
        cpf_pg: data.cpf_pg ? unformatCpfCnpj(data.cpf_pg) : null,
        tp_vale_ped: nullify(data.tp_vale_ped),
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a fornecedora.')
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
                                placeholder="Sem Parar"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="cnpj_forn"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>CNPJ da fornecedora *</FormLabel>
                         <Input id={field.name} maxLength={18} className="w-full"
                                placeholder="00.000.000/0000-00"
                                value={formatCpfCnpj(field.value)}
                                onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                onBlur={field.onBlur} ref={field.ref}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="tp_vale_ped"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Tipo do vale</FormLabel>
                         <OptionsSelect id={field.name} value={field.value ?? ''}
                                        onValueChange={field.onChange}
                                        placeholder="Não informado"
                                        options={[...TP_VALE_PED_OPTIONS]}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <div className="sm:col-span-2 grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField control={form.control} name="cnpj_pg"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>CNPJ do pagador</FormLabel>
                           <Input id={field.name} maxLength={18} className="w-full" placeholder="Opcional"
                                  value={formatCpfCnpj(field.value ?? '')}
                                  onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                  onBlur={field.onBlur} ref={field.ref}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="cpf_pg"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>CPF do pagador</FormLabel>
                           <Input id={field.name} maxLength={14} className="w-full" placeholder="Opcional"
                                  value={formatCpfCnpj(field.value ?? '')}
                                  onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                  onBlur={field.onBlur} ref={field.ref}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
          <p className="sm:col-span-2 text-xs text-gray-500">
            Preencha o pagador só quando o vale não é pago pelo próprio emitente — CNPJ ou CPF,
            nunca os dois.
          </p>
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
