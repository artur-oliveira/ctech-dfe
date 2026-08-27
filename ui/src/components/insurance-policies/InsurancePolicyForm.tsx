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
  RESP_SEG_OPTIONS,
  type InsurancePolicyFormData,
  insurancePolicySchema,
} from '@/lib/schemas/insurance-policies'
import {formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'
import type {InsurancePolicyCreate, InsurancePolicyItemOut} from '@/lib/types/api'

const EMPTY: InsurancePolicyFormData = {
  name: '', resp_seg: '1', cnpj: '', cpf: '', x_seg: '', cnpj_seg: '', n_apol: '',
}

export interface InsurancePolicyFormProps {
  initialData?: InsurancePolicyItemOut
  onSubmit: (data: InsurancePolicyCreate) => Promise<void>
  loading?: boolean
}

function toFormData(p: InsurancePolicyItemOut): InsurancePolicyFormData {
  const str = (v: unknown) => (typeof v === 'string' ? v : '')
  return {
    name: p.name,
    resp_seg: (str(p.resp_seg) || '1') as InsurancePolicyFormData['resp_seg'],
    cnpj: str(p.cnpj),
    cpf: str(p.cpf),
    x_seg: str(p.x_seg),
    cnpj_seg: str(p.cnpj_seg),
    n_apol: str(p.n_apol),
  }
}

/** Campo vazio vira null: um "" gravado é um default silenciosamente vazio. */
const nullify = (v: string | undefined) => (v ? v : null)

export function InsurancePolicyForm({initialData, onSubmit, loading}: InsurancePolicyFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<InsurancePolicyFormData>({
    resolver: zodResolver(insurancePolicySchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })

  const handleSubmit = async (data: InsurancePolicyFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        resp_seg: data.resp_seg,
        cnpj: data.cnpj ? unformatCpfCnpj(data.cnpj) : null,
        cpf: data.cpf ? unformatCpfCnpj(data.cpf) : null,
        x_seg: nullify(data.x_seg),
        cnpj_seg: data.cnpj_seg ? unformatCpfCnpj(data.cnpj_seg) : null,
        n_apol: nullify(data.n_apol),
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a apólice.')
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
                                placeholder="Apólice frota 2026"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="resp_seg"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Responsável pelo seguro *</FormLabel>
                         <OptionsSelect id={field.name} value={field.value}
                                        onValueChange={field.onChange}
                                        options={[...RESP_SEG_OPTIONS]}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="cnpj"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>CNPJ do responsável</FormLabel>
                         <Input id={field.name} maxLength={18} className="w-full"
                                placeholder="00.000.000/0000-00"
                                value={formatCpfCnpj(field.value ?? '')}
                                onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                onBlur={field.onBlur} ref={field.ref}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="cpf"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>CPF do responsável</FormLabel>
                         <Input id={field.name} maxLength={14} className="w-full"
                                placeholder="000.000.000-00"
                                value={formatCpfCnpj(field.value ?? '')}
                                onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                onBlur={field.onBlur} ref={field.ref}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="x_seg"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Seguradora</FormLabel>
                         <Input {...field} id={field.name} maxLength={30} className="w-full"
                                value={field.value ?? ''} placeholder="Opcional"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="cnpj_seg"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>CNPJ da seguradora</FormLabel>
                         <Input id={field.name} maxLength={18} className="w-full" placeholder="Opcional"
                                value={formatCpfCnpj(field.value ?? '')}
                                onChange={(e) => field.onChange(unformatCpfCnpj(e.target.value))}
                                onBlur={field.onBlur} ref={field.ref}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="n_apol"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Número da apólice</FormLabel>
                         <Input {...field} id={field.name} maxLength={20} className="w-full"
                                value={field.value ?? ''} placeholder="Opcional"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <p className="sm:col-span-2 text-xs text-gray-500">
            O documento do responsável só é informado quando ele não é o emitente do MDF-e — CNPJ ou
            CPF, nunca os dois; com o contratante como responsável, um dos dois é obrigatório. As
            averbações (nAver) são informadas por viagem, na emissão.
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
