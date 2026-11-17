'use client';

import {useState} from 'react';
import {useFieldArray, useForm, useWatch, type Control} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {useRouter} from 'next/navigation';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {EmitError} from '@/components/ui/emit-error';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Button, Textarea} from '@aoctech/ui';
import {UF_OPTIONS} from '@/lib/schemas/entity';
import {bpeTmSchema, emptyBpeTm, emptyBpeTmItem, toBpeTmBody, type BpeTmForm} from '@/lib/schemas/bpe-tm';
import {BPE_ICMS_OPTIONS, BPE_ICMS_WITH_RATE, BPE_MODALS, BPE_TM_COMPONENT_NAME_MAX, BPE_TM_COMPONENTS_MAX, BPE_TM_GROUPS_MAX, BPE_TM_ITEMS_MAX} from '@/lib/constants/bpe';
import {MunicipalityField} from '@/components/ui/municipality-field';
import {bpeFields, GRID, H2, SECTION, SUBCARD} from './fields';

function Components({control, name}: { control: Control<BpeTmForm>; name: `groups.${number}.items.${number}.components` }) {
  const {fields, append, remove} = useFieldArray({control, name});
  const {text} = bpeFields(control);
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium text-gray-600">Componentes de tarifa</p>
        <Button type="button" variant="outline" size="sm" disabled={fields.length >= BPE_TM_COMPONENTS_MAX}
                onClick={() => append({name: '', quantity: ''})}>+ Componente</Button>
      </div>
      {fields.map((f, k) => (
        <div key={f.id} className={`${GRID} items-end`}>
          {text(`${name}.${k}.name`, 'Nome', {maxLength: BPE_TM_COMPONENT_NAME_MAX, className: 'sm:col-span-2'})}
          {text(`${name}.${k}.quantity`, 'Passageiros', {inputMode: 'numeric'})}
          {fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger justify-self-start" onClick={() => remove(k)}>Remover</Button>}
        </div>
      ))}
    </div>
  );
}

function Items({control, g}: { control: Control<BpeTmForm>; g: number }) {
  const name = `groups.${g}.items` as const;
  const {fields, append, remove} = useFieldArray({control, name});
  const values = useWatch({control}) as BpeTmForm;
  const {text, select} = bpeFields(control);
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-gray-700">Itens por trecho</p>
        <Button type="button" variant="outline" size="sm" disabled={fields.length >= BPE_TM_ITEMS_MAX} onClick={() => append(emptyBpeTmItem())}>+ Item</Button>
      </div>
      {fields.map((f, i) => (
        <div key={f.id} className={SUBCARD}>
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-gray-700">Item {i + 1}</p>
            {fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => remove(i)}>Remover</Button>}
          </div>
          <div className={GRID}>
            <MunicipalityField control={control} name={`${name}.${i}.origin_ibge_code`} label="Município de origem"/>
            <MunicipalityField control={control} name={`${name}.${i}.dest_ibge_code`} label="Município de destino"/>
            {text(`${name}.${i}.passengers`, 'Total de passageiros', {inputMode: 'numeric'})}
            {text(`${name}.${i}.value`, 'Valor do item (R$)', {inputMode: 'decimal'})}
            {select(`${name}.${i}.icms.cst`, 'Situação do ICMS', BPE_ICMS_OPTIONS, 'sm:col-span-2')}
            {BPE_ICMS_WITH_RATE.includes(values.groups[g]?.items[i]?.icms.cst ?? '') && text(`${name}.${i}.icms.rate`, 'Alíquota do ICMS (%)', {inputMode: 'decimal'})}
          </div>
          <Components control={control} name={`${name}.${i}.components`}/>
        </div>
      ))}
    </div>
  );
}

export function BpeTmForm() {
  const router = useRouter();
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const form = useForm<BpeTmForm>({resolver: zodResolver(bpeTmSchema), defaultValues: emptyBpeTm()});
  const groups = useFieldArray({control: form.control, name: 'groups'});
  const values = useWatch({control: form.control}) as BpeTmForm;
  const {text, select} = bpeFields(form.control);
  const total = values.groups.reduce((s, g) => s + g.items.reduce((a, it) => a + (Number.isFinite(Number(it.value)) ? Number(it.value) : 0), 0), 0);

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
    setSubmitError({message: 'Revise os campos destacados antes de emitir o BP-e.'});
    window.setTimeout(() => document.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus(), 0);
  });

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="space-y-6 pb-24">
        <section className={SECTION} aria-labelledby="bpe-tm-apuracao">
          <h2 id="bpe-tm-apuracao" className={H2}>Apuração</h2>
          <div className={GRID}>
            {select('modal', 'Modal', BPE_MODALS)}
            {text('cfop', 'CFOP', {inputMode: 'numeric', maxLength: 4})}
            {text('competence', 'Competência', {type: 'date'})}
          </div>
          <FormField control={form.control} name="supply_indicator" render={({field}) => (
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
              Fornecimento (indFornecimento)
            </label>
          )}/>
        </section>

        <section className={SECTION} aria-labelledby="bpe-tm-grupos">
          <div className="flex items-center justify-between gap-3">
            <h2 id="bpe-tm-grupos" className={H2}>Grupos de viagem</h2>
            <Button type="button" variant="outline" size="sm" disabled={groups.fields.length >= BPE_TM_GROUPS_MAX}
                    onClick={() => groups.append({origin_uf: '', dest_uf: '', plate: '', prefix: '', items: [emptyBpeTmItem()]})}>+ Grupo</Button>
          </div>
          {groups.fields.map((f, g) => (
            <div key={f.id} className={SUBCARD}>
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium text-gray-700">Grupo {g + 1}</p>
                {groups.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => groups.remove(g)}>Remover</Button>}
              </div>
              <div className={GRID}>
                {select(`groups.${g}.origin_uf`, 'UF de início', UF_OPTIONS)}
                {select(`groups.${g}.dest_uf`, 'UF de fim', [{value: '', label: 'Mesma UF'}, ...UF_OPTIONS, {value: 'EX', label: 'Exterior'}])}
                {text(`groups.${g}.plate`, 'Placa', {maxLength: 7})}
                {text(`groups.${g}.prefix`, 'Prefixo')}
              </div>
              <Items control={form.control} g={g}/>
            </div>
          ))}
          {form.formState.errors.groups?.message && <p className="text-sm text-danger">{form.formState.errors.groups.message}</p>}
        </section>

        <section className={SECTION}>
          <FormField control={form.control} name="additional_info" render={({field}) => (
            <FormItem>
              <FormLabel>Informações adicionais</FormLabel>
              <Textarea {...field} id={field.name} rows={3} maxLength={5000}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>

        <EmitError failure={submitError}/>

        <div className="sticky bottom-(--bottomnav-height) -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between md:-mx-8 md:px-8">
          <p className="text-sm text-gray-600">Total estimado R$ {total.toFixed(2).replace('.', ',')}</p>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => router.push('/bpe')}>Cancelar</Button>
            <Button type="submit" variant="brand" disabled={isSubmitting}>{isSubmitting ? 'Enviando…' : 'Emitir BP-e'}</Button>
          </div>
        </div>
      </form>
    </Form>
  );
}
