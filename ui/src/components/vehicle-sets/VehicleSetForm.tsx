'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useQuery} from '@tanstack/react-query'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Combobox} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import {PersonPicker} from '@/components/persons/PersonPicker'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {MAX_TRAILERS, type VehicleSetFormData, vehicleSetSchema} from '@/lib/schemas/vehicle-sets'
import {formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'
import type {PersonItemOut, VehicleSetCreate, VehicleSetItemOut} from '@/lib/types/api'

interface VehicleSetFormProps {
  initialData?: VehicleSetItemOut
  onSubmit: (data: VehicleSetCreate) => Promise<void>
  loading?: boolean
}

const EMPTY: VehicleSetFormData = {
  name: '', tractor_sk: '', trailer_sks: [], driver_docs: [], rntrc: '', ciot: '',
}

/**
 * Cadastro de composição veicular. Reusa os seletores de veículo e o
 * PersonPicker com papel `driver` — nenhum seletor novo é escrito aqui.
 */
export function VehicleSetForm({initialData, onSubmit, loading = false}: VehicleSetFormProps) {
  const {selectedOrg} = useAuth()
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm<VehicleSetFormData>({
    resolver: zodResolver(vehicleSetSchema),
    defaultValues: initialData
      ? {
        name: initialData.name,
        tractor_sk: initialData.tractor_sk,
        trailer_sks: initialData.trailer_sks ?? [],
        driver_docs: initialData.driver_docs ?? [],
        rntrc: initialData.rntrc ?? '',
        ciot: initialData.ciot ?? '',
      }
      : EMPTY,
  })

  const {data: vehiclePage} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk),
    queryFn: () => apiClient.getVehicles({limit: 100}),
    enabled: !!selectedOrg,
  })
  const vehicles = vehiclePage?.items ?? []
  const vehicleLabel = (sk: string) => {
    const v = vehicles.find((it) => it.sk === sk)
    return v ? `${v.plate} · ${v.plate_uf}` : sk
  }
  const optionsFor = (role: string) => vehicles
    .filter((v) => v.role === role)
    .map((v) => ({value: v.sk, label: `${v.plate} · ${v.plate_uf}`}))

  const trailerSks = useWatch({control: form.control, name: 'trailer_sks'}) ?? []
  const driverDocs = useWatch({control: form.control, name: 'driver_docs'}) ?? []

  const addTrailer = (sk: string) => {
    if (!sk || trailerSks.includes(sk) || trailerSks.length >= MAX_TRAILERS) return
    form.setValue('trailer_sks', [...trailerSks, sk], {shouldValidate: true})
  }
  const addDriver = (person: PersonItemOut | null) => {
    if (!person) return
    const doc = unformatCpfCnpj(person.sk)
    if (driverDocs.includes(doc)) return
    form.setValue('driver_docs', [...driverDocs, doc], {shouldValidate: true})
  }

  const handleSubmit = async (data: VehicleSetFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        ...data,
        rntrc: data.rntrc || null,
        ciot: data.ciot || null,
      } as unknown as VehicleSetCreate)
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a composição.')
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
                                  placeholder="Carreta 1 — ABC1D23"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="tractor_sk"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Veículo de tração *</FormLabel>
                           <Combobox value={field.value} onValueChange={field.onChange}
                                     options={optionsFor('tractor')} placeholder="Escolha o cavalo/caminhão"
                                     searchPlaceholder="Placa..."/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField control={form.control} name="rntrc"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>RNTRC</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={12}
                                  inputMode="numeric" className="w-full"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="ciot"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>CIOT</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={20}
                                  className="w-full"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
            Reboques (máx. {MAX_TRAILERS})
          </p>
          {trailerSks.length > 0 && (
            <div className="space-y-1.5">
              {trailerSks.map((sk) => (
                <div key={sk} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                  <span className="font-mono text-gray-700">{vehicleLabel(sk)}</span>
                  <Button type="button" variant="ghost" size="xs"
                          onClick={() => form.setValue('trailer_sks', trailerSks.filter((it) => it !== sk),
                            {shouldValidate: true})}
                          className="text-danger hover:text-red-700">remover</Button>
                </div>
              ))}
            </div>
          )}
          {trailerSks.length < MAX_TRAILERS && (
            <Combobox value="" onValueChange={addTrailer}
                      options={optionsFor('trailer').filter((o) => !trailerSks.includes(o.value))}
                      placeholder="Adicionar reboque" searchPlaceholder="Placa..."/>
          )}
          <FormField control={form.control} name="trailer_sks" render={() => (<FormItem><FormMessage/></FormItem>)}/>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Condutores</p>
          {driverDocs.length > 0 && (
            <div className="space-y-1.5">
              {driverDocs.map((doc) => (
                <div key={doc} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                  <span className="font-mono text-gray-700">{formatCpfCnpj(doc)}</span>
                  <Button type="button" variant="ghost" size="xs"
                          onClick={() => form.setValue('driver_docs', driverDocs.filter((it) => it !== doc),
                            {shouldValidate: true})}
                          className="text-danger hover:text-red-700">remover</Button>
                </div>
              ))}
            </div>
          )}
          <PersonPicker value={null} onChange={addDriver} role="driver"
                        placeholder="Buscar condutor cadastrado"/>
        </div>

        {submitError && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="submit" variant="brand" disabled={loading} className="min-h-11">
            {loading ? 'Salvando…' : 'Salvar composição'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
