'use client'

import {useState} from 'react'
import type {Control, FieldPath, UseFormSetValue} from 'react-hook-form'
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Combobox} from '@/components/ui/combobox'
import {CITIES, CITY_OPTIONS} from '@/lib/data/cities'
import {maskCep} from '@/lib/utils/masks'
import type {EntityFormData} from "@/lib/schemas/entity";

interface AddressFieldsProps {
  control: Control<EntityFormData>
  setValue: UseFormSetValue<EntityFormData>
  /** Base path for the address object, e.g. "person.addresses.0" */
  basePath: FieldPath<EntityFormData>
}

export function AddressFields({control, setValue, basePath}: AddressFieldsProps) {
  const [cepLoading, setCepLoading] = useState(false)

  const p = (field: string) => `${basePath}.${field}` as FieldPath<EntityFormData>

  const lookupCep = async (raw: string) => {
    if (raw.length !== 8) return
    setCepLoading(true)
    try {
      const res = await fetch(`https://viacep.com.br/ws/${raw}/json/`)
      const data = await res.json() as {
        erro?: boolean; logradouro?: string; bairro?: string
        localidade?: string; uf?: string; ibge?: string
      }
      if (data.erro) return
      setValue(p('street'), data.logradouro ?? '', {shouldValidate: true})
      setValue(p('neighborhood'), data.bairro ?? '', {shouldValidate: true})
      setValue(p('city'), data.localidade ?? '', {shouldValidate: true})
      setValue(p('state_federation'), data.uf ?? 'SP', {shouldValidate: true})
      setValue(p('city_ibge_code'), data.ibge ?? '', {shouldValidate: true})
    } catch {
      // silently ignore network errors
    } finally {
      setCepLoading(false)
    }
  }

  return (
    <div className="space-y-3">
      {/* CEP */}
      <FormField
        control={control}
        name={p('postal_code')}
        render={({field}) => (
          <FormItem>
            <FormLabel>
              CEP
              {cepLoading && <span className="ml-2 text-xs font-normal text-gray-400">Buscando…</span>}
            </FormLabel>
            <Input
              id={field.name}
              placeholder="12345-678"
              maxLength={9}
              value={maskCep(String(field.value ?? ''))}
              onChange={(e) => {
                const raw = e.target.value.replace(/\D/g, '').slice(0, 8)
                field.onChange(raw)
                if (raw.length === 8) void lookupCep(raw)
              }}
              onBlur={field.onBlur}
              ref={field.ref}
            />
            <FormMessage/>
          </FormItem>
        )}
      />

      {/* Logradouro */}
      <FormField
        control={control}
        name={p('street')}
        render={({field}) => (
          <FormItem>
            <FormLabel>Logradouro</FormLabel>
            <Input {...field} id={field.name} placeholder="Av. Paulista" maxLength={255}
                   value={String(field.value ?? '')}/>
            <FormMessage/>
          </FormItem>
        )}
      />

      {/* Número + Bairro + Complemento */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <FormField
          control={control}
          name={p('number')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Número</FormLabel>
              <Input {...field} id={field.name} placeholder="1000 ou S/N" maxLength={20}
                     value={String(field.value ?? '')}/>
              <FormMessage/>
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={p('neighborhood')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Bairro</FormLabel>
              <Input {...field} id={field.name} placeholder="Centro" maxLength={120}
                     value={String(field.value ?? '')}/>
              <FormMessage/>
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={p('complement')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Complemento</FormLabel>
              <Input {...field} id={field.name} placeholder="Apto 101" maxLength={120}
                     value={String(field.value ?? '')}/>
              <FormMessage/>
            </FormItem>
          )}
        />
      </div>

      {/* Cidade / UF / Cód. IBGE — combobox unificado */}
      <FormField
        control={control}
        name={p('city_ibge_code')}
        render={({field}) => (
          <FormItem>
            <FormLabel>Cidade / UF</FormLabel>
            <Combobox
              id={field.name}
              value={field.value?.toString() || "SP"}
              onValueChange={(code) => {
                const city = CITIES.find((c) => c.code === code)
                if (!city) return
                field.onChange(code)
                setValue(p('city'), city.description, {shouldValidate: true})
                setValue(p('state_federation'), city.uf, {shouldValidate: true})
              }}
              options={CITY_OPTIONS}
              placeholder="Selecione a cidade…"
              searchPlaceholder="Digite cidade ou UF…"
            />
            <FormMessage/>
          </FormItem>
        )}
      />
    </div>
  )
}
