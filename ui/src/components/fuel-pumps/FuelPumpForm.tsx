'use client'

import {useState} from 'react'
import {useForm} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {NumericInput} from '@/components/ui/numeric-input'
import {ApiError} from '@/lib/api/client'
import {type FuelPumpFormData, fuelPumpSchema} from '@/lib/schemas/fuel-pumps'
import type {FuelPumpCreate, FuelPumpItemOut} from '@/lib/types/api'

const EMPTY: FuelPumpFormData = {name: '', n_bico: '', n_bomba: '', n_tanque: ''}

export interface FuelPumpFormProps {
  initialData?: FuelPumpItemOut
  onSubmit: (data: FuelPumpCreate) => Promise<void>
  loading?: boolean
}

function toFormData(p: FuelPumpItemOut): FuelPumpFormData {
  const str = (v: unknown) => (typeof v === 'string' ? v : '')
  return {name: p.name, n_bico: str(p.n_bico), n_bomba: str(p.n_bomba), n_tanque: str(p.n_tanque)}
}

export function FuelPumpForm({initialData, onSubmit, loading}: FuelPumpFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<FuelPumpFormData>({
    resolver: zodResolver(fuelPumpSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })

  const lastReading = typeof initialData?.last_v_enc_fin === 'string' ? initialData.last_v_enc_fin : ''

  const handleSubmit = async (data: FuelPumpFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        n_bico: data.n_bico,
        n_bomba: data.n_bomba ?? '',
        n_tanque: data.n_tanque ?? '',
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a bomba.')
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
                                placeholder="Bico 1 — Gasolina comum"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="n_bico"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Número do bico *</FormLabel>
                         <NumericInput id={field.name} value={field.value} integerPlaces={3}
                                       className="w-full" onChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="n_bomba"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Número da bomba</FormLabel>
                         <NumericInput id={field.name} value={field.value ?? ''} integerPlaces={3}
                                       className="w-full" onChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="n_tanque"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Número do tanque</FormLabel>
                         <NumericInput id={field.name} value={field.value ?? ''} integerPlaces={3}
                                       className="w-full" onChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
        </div>

        <div className="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-xs text-gray-600">
          Última leitura do encerrante:{' '}
          <span className="font-medium text-gray-900">{lastReading || 'nenhuma venda ainda'}</span>.
          Ela é gravada pela emissão, na mesma transação da nota — a próxima venda parte daqui e não
          precisa (nem deve) ser digitada.
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
