'use client';

import {useEffect, useState} from 'react';
import type {Control, FieldValues, UseFormSetValue} from 'react-hook-form';
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Combobox, type ComboboxOption} from '@/components/ui/combobox';

type CityTable = { CITIES: { code: string; description: string; uf: string }[]; CITY_OPTIONS: ComboboxOption[] }

// A tabela de municípios do IBGE pesa centenas de kB: carrega só quando um seletor aparece, fora do bundle da rota.
let cached: Promise<CityTable> | null = null;
const loadCities = () => (cached ??= import('@/lib/data/cities'));

interface Props<T extends FieldValues> {
  control: Control<T>
  /** Só precisa quando `ufName` ou `cityName` acompanham a escolha. */
  setValue?: UseFormSetValue<T>
  /** Caminho do campo que guarda o código IBGE de 7 dígitos (o valor escolhido). */
  name: string
  label: string
  /** Campos que acompanham o município: a UF e o nome (preenchidos ao escolher). */
  ufName?: string
  cityName?: string
  className?: string
}

/** Seletor de município pela tabela do IBGE: grava o código e preenche UF e nome, no lugar de digitar o código de 7 dígitos. */
export function MunicipalityField<T extends FieldValues>({control, setValue, name, label, ufName, cityName, className}: Props<T>) {
  const [table, setTable] = useState<CityTable | null>(null);
  useEffect(() => {
    let alive = true;
    void loadCities().then((t) => { if (alive) setTable(t); });
    return () => { alive = false; };
  }, []);
  return (
    <FormField control={control} name={name as never} render={({field}) => (
      <FormItem className={className}>
        <FormLabel>{label}</FormLabel>
        <Combobox
          id={field.name}
          value={(field.value as string) || ''}
          onValueChange={(code) => {
            const city = table?.CITIES.find((c) => c.code === code);
            if (!city) return;
            field.onChange(code);
            if (cityName) setValue?.(cityName as never, city.description as never, {shouldValidate: true});
            if (ufName) setValue?.(ufName as never, city.uf as never, {shouldValidate: true});
          }}
          options={table?.CITY_OPTIONS ?? []}
          placeholder={table ? 'Selecione o município…' : 'Carregando municípios…'}
          searchPlaceholder="Digite município ou UF…"
        />
        <FormMessage/>
      </FormItem>
    )}/>
  );
}
