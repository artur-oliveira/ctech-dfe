'use client';

import {useState} from 'react';
import dynamic from 'next/dynamic';
import type {Control, FieldValues, UseFormSetValue} from 'react-hook-form';
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
// O seletor traz o formulário de pessoa inteiro: carrega só quando o campo aparece, fora do bundle da rota de emissão.
const PersonPicker = dynamic(() => import('@/components/persons/PersonPicker').then((m) => m.PersonPicker), {ssr: false});
import type {PersonRole} from '@/lib/schemas/entity';
import type {PersonItemOut} from '@/lib/types/api';

/** Pessoa do cadastro escolhida no formulário: o campo guarda só a chave (`CNPJ_...`, `CPF_...`), o servidor resolve o resto. */
export function PersonIdField<T extends FieldValues>({control, setValue, name, label, hint, role, className}: {
  control: Control<T>
  setValue: UseFormSetValue<T>
  name: string
  label: string
  hint?: string
  role?: PersonRole
  className?: string
}) {
  const [person, setPerson] = useState<PersonItemOut | null>(null);
  return (
    <FormField control={control} name={name as never} render={() => (
      <FormItem className={className}>
        <FormLabel>{label}</FormLabel>
        <PersonPicker
          value={person}
          role={role}
          onChange={(p) => {
            setPerson(p);
            setValue(name as never, (p?.sk ?? '') as never, {shouldValidate: true, shouldDirty: true});
          }}
        />
        {hint && <p className="text-xs text-gray-500">{hint}</p>}
        <FormMessage/>
      </FormItem>
    )}/>
  );
}
