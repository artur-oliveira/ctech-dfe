'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {OptionsSelect} from '@/components/ui/options-select'
import {ApiError} from '@/lib/api/client'
import {
  CARGO_UNIT_KIND_OPTIONS,
  TP_UNID_CARGA_OPTIONS,
  TP_UNID_TRANSP_OPTIONS,
  type CargoUnitFormData,
  cargoUnitSchema,
} from '@/lib/schemas/cargo-units'
import type {CargoUnitCreate, CargoUnitItemOut} from '@/lib/types/api'

const EMPTY: CargoUnitFormData = {name: '', kind: 'transport', tp_unid: '1', id_unid: '', seals: ''}

export interface CargoUnitFormProps {
  initialData?: CargoUnitItemOut
  onSubmit: (data: CargoUnitCreate) => Promise<void>
  loading?: boolean
}

function toFormData(u: CargoUnitItemOut): CargoUnitFormData {
  const seals = Array.isArray(u.seals) ? (u.seals as string[]) : []
  return {
    name: u.name,
    kind: u.kind,
    tp_unid: u.tp_unid as CargoUnitFormData['tp_unid'],
    id_unid: u.id_unid,
    seals: seals.join(', '),
  }
}

export function CargoUnitForm({initialData, onSubmit, loading}: CargoUnitFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<CargoUnitFormData>({
    resolver: zodResolver(cargoUnitSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })
  const kind = useWatch({control: form.control, name: 'kind'})

  const handleSubmit = async (data: CargoUnitFormData) => {
    setSubmitError(null)
    const seals = (data.seals ?? '').split(',').map((s) => s.trim()).filter(Boolean)
    try {
      await onSubmit({
        name: data.name,
        kind: data.kind,
        tp_unid: data.tp_unid,
        id_unid: data.id_unid,
        seals: seals.length ? seals : null,
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a unidade.')
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
                                placeholder="Carreta 1"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="kind"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Tipo de unidade *</FormLabel>
                         <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                        options={CARGO_UNIT_KIND_OPTIONS}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="tp_unid"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Classificação *</FormLabel>
                         <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                        options={kind === 'cargo' ? TP_UNID_CARGA_OPTIONS : TP_UNID_TRANSP_OPTIONS}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="id_unid"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Identificação *</FormLabel>
                         <Input {...field} id={field.name} maxLength={20} className="w-full"
                                placeholder="Placa, nº do contêiner ou do vagão"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="seals"
                     render={({field}) => (
                       <FormItem className="sm:col-span-2">
                         <FormLabel>Lacres fixos</FormLabel>
                         <Input {...field} id={field.name} value={field.value ?? ''} className="w-full"
                                placeholder="Separados por vírgula"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <p className="sm:col-span-2 text-xs text-gray-500">
            O rateio da carga entre os documentos é calculado na emissão a partir dos pesos — não se
            informa percentual aqui.
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
