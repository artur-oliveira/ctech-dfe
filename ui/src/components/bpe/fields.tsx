'use client';

import type {Control, FieldValues} from 'react-hook-form';
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {OptionsSelect} from '@/components/ui/options-select';

type Opt = readonly {value: string; label: string}[]

/** Campos de texto e seletor ligados ao react-hook-form, comuns aos formulários do BP-e. */
export function bpeFields<T extends FieldValues>(control: Control<T>) {
  const text = (name: string, label: string, props: {type?: string; className?: string; inputMode?: 'numeric' | 'decimal'; maxLength?: number} = {}) => (
    <FormField control={control} name={name as never} render={({field}) => (
      <FormItem className={props.className}>
        <FormLabel>{label}</FormLabel>
        <Input {...field} value={(field.value as string) ?? ''} id={field.name} type={props.type} inputMode={props.inputMode} maxLength={props.maxLength}/>
        <FormMessage/>
      </FormItem>
    )}/>
  );
  const select = (name: string, label: string, options: Opt, className?: string) => (
    <FormField control={control} name={name as never} render={({field}) => (
      <FormItem className={className}>
        <FormLabel>{label}</FormLabel>
        <OptionsSelect id={field.name} value={field.value as string} onValueChange={field.onChange} options={[...options]}/>
        <FormMessage/>
      </FormItem>
    )}/>
  );
  return {text, select};
}

export const SECTION = 'rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4';
export const GRID = 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4';
export const SUBCARD = 'space-y-4 rounded-lg border border-gray-100 bg-gray-50 p-3 md:p-4';
export const H2 = 'text-base font-semibold text-gray-900';
