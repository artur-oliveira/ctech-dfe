'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {Combobox} from '@/components/ui/combobox'
import {OptionsSelect} from '@/components/ui/options-select'
import {CITY_OPTIONS} from '@/lib/data/cities'
import {ApiError} from '@/lib/api/client'
import {maskCep} from '@/lib/utils/masks'
import {
  ADDRESS_SCOPES,
  SERVICE_LOCATION_ROLES,
  type ServiceLocationFormData,
  serviceLocationSchema,
} from '@/lib/schemas/service-locations'
import type {ServiceLocationCreate, ServiceLocationItemOut, ServiceLocationRole} from '@/lib/types/api'

const EMPTY: ServiceLocationFormData = {
  name: '', roles: [], address_scope: 'national',
  street: '', number: '', complement: '', neighborhood: '',
  postal_code: '', city_ibge_code: '',
  foreign_postal_code: '', foreign_city: '', foreign_region: '',
  insc_imob_fisc: '', c_obra: '', cib: '', id_atv_evt: '',
}

export interface ServiceLocationFormProps {
  initialData?: ServiceLocationItemOut
  onSubmit: (data: ServiceLocationCreate) => Promise<void>
  loading?: boolean
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function toFormData(l: ServiceLocationItemOut): ServiceLocationFormData {
  const address = (l.address ?? {}) as Record<string, unknown>
  const foreign = !!str(address.foreign_postal_code)
  return {
    name: l.name,
    roles: Array.isArray(l.roles) ? (l.roles as ServiceLocationRole[]) : [],
    address_scope: foreign ? 'foreign' : 'national',
    street: str(address.street),
    number: str(address.number),
    complement: str(address.complement),
    neighborhood: str(address.neighborhood),
    postal_code: str(address.postal_code),
    city_ibge_code: str(address.city_ibge_code),
    foreign_postal_code: str(address.foreign_postal_code),
    foreign_city: str(address.foreign_city),
    foreign_region: str(address.foreign_region),
    insc_imob_fisc: str(l.insc_imob_fisc),
    c_obra: str(l.c_obra),
    cib: str(l.cib),
    id_atv_evt: str(l.id_atv_evt),
  }
}

/** Campo vazio vira null: o backend distingue ausente de string vazia. */
function orNull(value: string | undefined): string | null {
  return value ? value : null
}

export function ServiceLocationForm({initialData, onSubmit, loading}: ServiceLocationFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<ServiceLocationFormData>({
    resolver: zodResolver(serviceLocationSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })
  const scope = useWatch({control: form.control, name: 'address_scope'})
  const isForeign = scope === 'foreign'

  const handleSubmit = async (data: ServiceLocationFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        roles: data.roles,
        address: {
          street: data.street,
          number: data.number,
          complement: orNull(data.complement),
          neighborhood: data.neighborhood,
          postal_code: isForeign ? null : orNull(data.postal_code),
          city_ibge_code: isForeign ? null : orNull(data.city_ibge_code),
          foreign_postal_code: isForeign ? orNull(data.foreign_postal_code) : null,
          foreign_city: isForeign ? orNull(data.foreign_city) : null,
          foreign_region: isForeign ? orNull(data.foreign_region) : null,
        },
        insc_imob_fisc: orNull(data.insc_imob_fisc),
        c_obra: orNull(data.c_obra),
        cib: orNull(data.cib),
        id_atv_evt: orNull(data.id_atv_evt),
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o local.')
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
                                placeholder="Obra Centro"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="address_scope"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Onde fica *</FormLabel>
                         <OptionsSelect id={field.name} value={field.value} className="w-full"
                                        options={[...ADDRESS_SCOPES]}
                                        onValueChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
        </div>

        <FormField control={form.control} name="roles"
                   render={({field}) => (
                     <FormItem>
                       <FormLabel>Papéis do local *</FormLabel>
                       <div className="flex flex-col sm:flex-row gap-2">
                         {SERVICE_LOCATION_ROLES.map((role) => {
                           const checked = field.value.includes(role.value)
                           return (
                             <label key={role.value}
                                    className="flex items-center gap-2 min-h-11 px-3 rounded-md border border-gray-300 text-sm">
                               <input type="checkbox" checked={checked}
                                      onChange={() => field.onChange(checked
                                        ? field.value.filter((r) => r !== role.value)
                                        : [...field.value, role.value])}/>
                               {role.label}
                             </label>
                           )
                         })}
                       </div>
                       <p className="text-xs text-gray-500">
                         Combináveis: o mesmo endereço serve obra, imóvel e local de evento sem
                         virar três cadastros iguais.
                       </p>
                       <FormMessage/>
                     </FormItem>
                   )}
        />

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FormField control={form.control} name="street"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Logradouro *</FormLabel>
                         <Input {...field} id={field.name} maxLength={255} className="w-full"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="number"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Número *</FormLabel>
                         <Input {...field} id={field.name} maxLength={60} className="w-full"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="neighborhood"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Bairro *</FormLabel>
                         <Input {...field} id={field.name} maxLength={60} className="w-full"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="complement"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Complemento</FormLabel>
                         <Input {...field} id={field.name} value={field.value ?? ''} maxLength={156}
                                className="w-full" placeholder="Opcional"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />

          {!isForeign && (
            <>
              <FormField control={form.control} name="postal_code"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>CEP *</FormLabel>
                             <Input id={field.name} value={maskCep(field.value ?? '')} maxLength={9}
                                    className="w-full" inputMode="numeric"
                                    onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
              <FormField control={form.control} name="city_ibge_code"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>Município *</FormLabel>
                             <Combobox id={field.name} value={field.value} options={CITY_OPTIONS}
                                       className="w-full" placeholder="Selecione o município"
                                       searchPlaceholder="Buscar município…" fuzzySearch
                                       onValueChange={field.onChange}/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
            </>
          )}

          {isForeign && (
            <>
              <FormField control={form.control} name="foreign_postal_code"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>Código postal *</FormLabel>
                             <Input {...field} id={field.name} value={field.value ?? ''} maxLength={11}
                                    className="w-full"/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
              <FormField control={form.control} name="foreign_city"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>Cidade *</FormLabel>
                             <Input {...field} id={field.name} value={field.value ?? ''} maxLength={60}
                                    className="w-full"/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
              <FormField control={form.control} name="foreign_region"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>Estado / província / região *</FormLabel>
                             <Input {...field} id={field.name} value={field.value ?? ''} maxLength={60}
                                    className="w-full"/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
            </>
          )}
        </div>

        {!isForeign && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <FormField control={form.control} name="c_obra"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Código da obra (CNO)</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={30}
                                  className="w-full" placeholder="Opcional"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="cib"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>CIB</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={8}
                                  className="w-full" placeholder="8 caracteres"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="insc_imob_fisc"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Inscrição imobiliária</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={30}
                                  className="w-full" placeholder="Opcional"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <p className="sm:col-span-3 text-xs text-gray-500">
              Informe o código da obra <b>ou</b> o CIB, nunca os dois: o leiaute escolhe um dos dois
              — ou o endereço — e guardar ambos deixaria a emissão decidir sozinha.
            </p>
          </div>
        )}

        <FormField control={form.control} name="id_atv_evt"
                   render={({field}) => (
                     <FormItem>
                       <FormLabel>Identificação da atividade de evento</FormLabel>
                       <Input {...field} id={field.name} value={field.value ?? ''} maxLength={30}
                              className="w-full sm:max-w-sm" placeholder="Opcional"/>
                       <p className="text-xs text-gray-500">
                         Nome e período do evento mudam a cada nota e são pedidos na emissão.
                       </p>
                       <FormMessage/>
                     </FormItem>
                   )}
        />

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
