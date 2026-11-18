'use client';

import {useEffect, useState} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Button} from '@aoctech/ui';
import {OptionsSelect} from '@/components/ui/options-select';
import {MunicipalityField} from '@/components/ui/municipality-field';
import {LengthHint} from '@/components/ui/length-hint';
import {PersonPicker} from '@/components/persons/PersonPicker';
import {apiClient, ApiError} from '@/lib/api/client';
import {NFCOM_SERVICE_TYPES, NFCOM_SUBSCRIBER_TYPES} from '@/lib/constants/nfcom';
import {
  emptyNfcomSubscriber,
  type NfcomSubscriberFormData,
  nfcomSubscriberSchema,
  toNfcomSubscriber,
} from '@/lib/schemas/nfcom';
import type {NfcomSubscriberCreate, NfcomSubscriberItemOut, PersonItemOut} from '@/lib/types/api';

interface Props {
  initialData?: NfcomSubscriberItemOut
  onSubmit: (data: NfcomSubscriberCreate) => Promise<void>
  loading?: boolean
}

const str = (v: unknown): string => (typeof v === 'string' ? v : '');

function toFormData(s: NfcomSubscriberItemOut): NfcomSubscriberFormData {
  return {
    name: str(s.name), person_id: str(s.person_id), code: str(s.code), type: str(s.type), service_used: str(s.service_used),
    contract_number: str(s.contract_number), contract_start: str(s.contract_start), contract_end: str(s.contract_end),
    primary_terminal: str(s.primary_terminal), primary_ibge_code: str(s.primary_ibge_code), primary_uf: str(s.primary_uf),
  };
}

/** A pessoa de um assinante já gravado: busca pelo documento da chave (`CNPJ_...`/`CPF_...`) para mostrar o nome. */
function usePersonOf(personId: string): PersonItemOut | null {
  const [person, setPerson] = useState<PersonItemOut | null>(null);
  useEffect(() => {
    const doc = personId.replace(/^(CNPJ|CPF)_/, '');
    let alive = true;
    if (personId && doc !== personId) {
      apiClient.getPerson(doc).then((p) => { if (alive) setPerson(p); }).catch(() => undefined);
    }
    return () => { alive = false; };
  }, [personId]);
  return person;
}

export function NfcomSubscriberForm({initialData, onSubmit, loading}: Props) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const form = useForm<NfcomSubscriberFormData>({
    resolver: zodResolver(nfcomSubscriberSchema),
    defaultValues: initialData ? toFormData(initialData) : emptyNfcomSubscriber(),
  });
  const personId = useWatch({control: form.control, name: 'person_id'});
  const loaded = usePersonOf(initialData ? str(initialData.person_id) : '');
  const [picked, setPicked] = useState<PersonItemOut | null>(null);
  const person = picked ?? (loaded && loaded.sk === personId ? loaded : null);

  const handleSubmit = async (data: NfcomSubscriberFormData) => {
    setSubmitError(null);
    try {
      await onSubmit(toNfcomSubscriber(nfcomSubscriberSchema.parse(data)));
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o assinante.');
    }
  };

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField control={form.control} name="name" render={({field}) => (
            <FormItem>
              <FormLabel>Nome do cadastro *</FormLabel>
              <Input {...field} id={field.name} maxLength={60} className="w-full" placeholder="Contrato Matriz"/>
              <LengthHint value={field.value} max={60}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="code" render={({field}) => (
            <FormItem>
              <FormLabel>Código do assinante *</FormLabel>
              <Input {...field} id={field.name} maxLength={30} className="w-full" placeholder="ASS-001"/>
              <FormMessage/>
            </FormItem>
          )}/>
        </div>

        <FormField control={form.control} name="person_id" render={() => (
          <FormItem>
            <FormLabel>Pessoa (cliente) *</FormLabel>
            <PersonPicker
              value={person}
              onChange={(p) => {
                setPicked(p);
                form.setValue('person_id', p?.sk ?? '', {shouldValidate: true, shouldDirty: true});
              }}
              role="customer"
            />
            <p className="text-xs text-gray-500">Endereço, inscrição estadual e documento vêm do cadastro da pessoa.</p>
            <FormMessage/>
          </FormItem>
        )}/>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField control={form.control} name="type" render={({field}) => (
            <FormItem>
              <FormLabel>Tipo de assinante *</FormLabel>
              <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={[...NFCOM_SUBSCRIBER_TYPES]}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="service_used" render={({field}) => (
            <FormItem>
              <FormLabel>Serviço utilizado *</FormLabel>
              <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={[...NFCOM_SERVICE_TYPES]}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="contract_number" render={({field}) => (
            <FormItem>
              <FormLabel>Número do contrato</FormLabel>
              <Input {...field} id={field.name} maxLength={20} className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="primary_terminal" render={({field}) => (
            <FormItem>
              <FormLabel>Terminal principal</FormLabel>
              <Input {...field} id={field.name} maxLength={12} inputMode="numeric" className="w-full"
                     onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="contract_start" render={({field}) => (
            <FormItem>
              <FormLabel>Início do contrato</FormLabel>
              <Input {...field} id={field.name} type="date" className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="contract_end" render={({field}) => (
            <FormItem>
              <FormLabel>Fim do contrato</FormLabel>
              <Input {...field} id={field.name} type="date" className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <MunicipalityField control={form.control} setValue={form.setValue} name="primary_ibge_code" ufName="primary_uf"
                             label="Município do terminal principal" className="sm:col-span-2"/>
        </div>

        {submitError && <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{submitError}</div>}
        <div className="flex gap-2">
          <Button type="submit" variant="brand" disabled={loading}>{loading ? 'Salvando…' : 'Salvar'}</Button>
        </div>
      </form>
    </Form>
  );
}
