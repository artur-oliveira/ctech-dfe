'use client';

import {useState} from 'react';
import Link from 'next/link';
import {useFieldArray, useForm, useWatch, type Control} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {useRouter} from 'next/navigation';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {EmitError} from '@/components/ui/emit-error';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Button, Textarea} from '@aoctech/ui';
import {Combobox} from '@/components/ui/combobox';
import {LengthHint} from '@/components/ui/length-hint';
import {bpeTmSchema, emptyBpeTm, emptyBpeTmItem, toBpeTmBody, type BpeTmForm} from '@/lib/schemas/bpe-tm';
import {BPE_TM_COMPONENTS_MAX, BPE_TM_GROUPS_MAX, BPE_TM_ITEMS_MAX} from '@/lib/constants/bpe';
import type {BpeLineItemOut} from '@/lib/types/api';
import {bpeFields, GRID, H2, SECTION, SUBCARD} from './fields';
import {useBpeLines, useCityName} from './useBpeLines';

const ADDITIONAL_INFO_MAX = 5000;

function Components({control, name, types}: { control: Control<BpeTmForm>; name: `groups.${number}.items.${number}.components`; types: string[] }) {
  const {fields, append, remove} = useFieldArray({control, name});
  const {text, select} = bpeFields(control);
  const options = types.map((t) => ({value: t, label: t}));
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium text-gray-600">Passageiros por tipo</p>
        <Button type="button" variant="outline" size="sm" disabled={fields.length >= BPE_TM_COMPONENTS_MAX}
                onClick={() => append({name: '', quantity: ''})}>+ Tipo</Button>
      </div>
      {fields.map((f, k) => (
        <div key={f.id} className={`${GRID} items-end`}>
          {select(`${name}.${k}.name`, 'Tipo de passageiro', options, 'sm:col-span-2')}
          {text(`${name}.${k}.quantity`, 'Quantidade', {inputMode: 'numeric', maxLength: 5})}
          {fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger justify-self-start" onClick={() => remove(k)}>Remover</Button>}
        </div>
      ))}
    </div>
  );
}

function Items({control, g, line}: { control: Control<BpeTmForm>; g: number; line: BpeLineItemOut | undefined }) {
  const name = `groups.${g}.items` as const;
  const {fields, append, remove} = useFieldArray({control, name});
  const {text, select} = bpeFields(control);
  const cityName = useCityName();
  const legOptions = (line?.legs ?? []).map((l, i) => ({
    value: String(i + 1),
    label: `Trecho ${i + 1}: ${cityName(l.origin_ibge_code)}${l.dest_ibge_code ? ` até ${cityName(l.dest_ibge_code)}` : ''}`,
  }));
  const types = line?.passenger_types ?? [];
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-gray-700">Apuração por trecho</p>
        <Button type="button" variant="outline" size="sm" disabled={fields.length >= BPE_TM_ITEMS_MAX} onClick={() => append(emptyBpeTmItem())}>+ Item</Button>
      </div>
      {fields.map((f, i) => (
        <div key={f.id} className={SUBCARD}>
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-gray-700">Item {i + 1}</p>
            {fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => remove(i)}>Remover</Button>}
          </div>
          <div className={GRID}>
            {select(`${name}.${i}.leg`, 'Trecho *', legOptions, 'sm:col-span-2')}
            {text(`${name}.${i}.passengers`, 'Total de passageiros *', {inputMode: 'numeric', maxLength: 6})}
            {text(`${name}.${i}.value`, 'Valor do item (R$) *', {inputMode: 'decimal'})}
            {text(`${name}.${i}.start_counter`, 'Contador inicial', {inputMode: 'numeric', maxLength: 20})}
            {text(`${name}.${i}.end_counter`, 'Contador final', {inputMode: 'numeric', maxLength: 20})}
            {text(`${name}.${i}.trip_number`, 'Número da viagem', {inputMode: 'numeric', maxLength: 3})}
          </div>
          <Components control={control} name={`${name}.${i}.components`} types={types}/>
        </div>
      ))}
    </div>
  );
}

export function BpeTmForm() {
  const router = useRouter();
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const {options: lineOptions, bySk, isPending: linesPending} = useBpeLines('tm');
  const form = useForm<BpeTmForm>({resolver: zodResolver(bpeTmSchema), defaultValues: emptyBpeTm()});
  const groups = useFieldArray({control: form.control, name: 'groups'});
  const values = useWatch({control: form.control}) as BpeTmForm;
  const {text} = bpeFields(form.control);
  const noLines = !linesPending && lineOptions.length === 0;

  const submit = form.handleSubmit(async (data) => {
    setSubmitError(null);
    setIsSubmitting(true);
    try {
      const result = await apiClient.emitBpe(toBpeTmBody(data));
      toast.success('BP-e TM enviado para processamento', {description: 'Acompanhe a autorização na tela de detalhes.'});
      router.push(`/bpe/detail?key=${encodeURIComponent(result.sk)}`);
    } catch (error) {
      setSubmitError(emitFailure(error, 'Não foi possível enviar o BP-e TM. Revise os dados e tente novamente.'));
      setIsSubmitting(false);
    }
  }, () => {
    setSubmitError({message: 'Revise os campos destacados antes de emitir o BP-e TM.'});
    const first = document.querySelector<HTMLElement>('[aria-invalid="true"]');
    window.setTimeout(() => first?.focus(), 0);
  });

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="space-y-6 pb-24">
        {noLines && (
          <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            Cadastre antes <Link href="/bpe-lines/new" className="font-semibold underline">uma linha do TM</Link>: a apuração só escolhe a linha, o trecho e informa os números.
          </div>
        )}

        <section className={SECTION} aria-labelledby="tm-competencia">
          <h2 id="tm-competencia" className={H2}>Competência</h2>
          <div className={GRID}>
            {text('competence', 'Competência *', {type: 'date'})}
            <FormField control={form.control} name="supply_indicator" render={({field}) => (
              <label className="flex min-h-11 items-center gap-2 self-end text-sm text-gray-700 sm:col-span-2">
                <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
                Fornecimento (indFornecimento)
              </label>
            )}/>
          </div>
        </section>

        <section className={SECTION} aria-labelledby="tm-grupos">
          <div className="flex items-center justify-between gap-3">
            <h2 id="tm-grupos" className={H2}>Grupos de viagem</h2>
            <Button type="button" variant="outline" size="sm" disabled={groups.fields.length >= BPE_TM_GROUPS_MAX}
                    onClick={() => groups.append({line_id: '', counter_id: '', items: [emptyBpeTmItem()]})}>+ Grupo</Button>
          </div>
          {groups.fields.map((f, g) => (
            <div key={f.id} className={SUBCARD}>
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium text-gray-700">Grupo {g + 1}</p>
                {groups.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => groups.remove(g)}>Remover</Button>}
              </div>
              <div className={GRID}>
                <FormField control={form.control} name={`groups.${g}.line_id`} render={({field}) => (
                  <FormItem className="sm:col-span-2">
                    <FormLabel>Linha *</FormLabel>
                    <Combobox id={field.name} value={field.value} onValueChange={field.onChange} options={lineOptions}
                              placeholder={linesPending ? 'Carregando…' : 'Selecione a linha…'} searchPlaceholder="Digite o nome da linha…"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                {text(`groups.${g}.counter_id`, 'Equipamento contador', {inputMode: 'numeric', maxLength: 3})}
              </div>
              <Items control={form.control} g={g} line={bySk.get(values.groups[g]?.line_id ?? '')}/>
            </div>
          ))}
        </section>

        <section className={SECTION} aria-labelledby="tm-adicional">
          <h2 id="tm-adicional" className={H2}>Informações adicionais</h2>
          <FormField control={form.control} name="additional_info" render={({field}) => (
            <FormItem>
              <FormLabel>Texto livre</FormLabel>
              <Textarea {...field} id={field.name} rows={3} maxLength={ADDITIONAL_INFO_MAX}/>
              <LengthHint value={field.value} max={ADDITIONAL_INFO_MAX}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>

        <EmitError failure={submitError}/>

        <div className="sticky bottom-(--bottomnav-height) -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-end md:-mx-8 md:px-8">
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => router.push('/bpe')}>Cancelar</Button>
            <Button type="submit" variant="brand" disabled={isSubmitting || noLines}>{isSubmitting ? 'Enviando…' : 'Emitir BP-e TM'}</Button>
          </div>
        </div>
      </form>
    </Form>
  );
}
