'use client';

import {useEffect, useState} from 'react';
import Link from 'next/link';
import {useFieldArray, useForm, useWatch} from 'react-hook-form';
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
import {OptionsSelect} from '@/components/ui/options-select';
import {bpeTicketSchema, emptyBpeTicket, emptyBpeTrip, PASSENGER_MODES, toBpeTicketBody, type BpeTicketForm} from '@/lib/schemas/bpe';
import {
  BPE_COMPONENT_TYPES, BPE_COMPONENTS_MAX, BPE_DISCOUNT_TYPES, BPE_DOC_TYPES, BPE_LEG_TYPES, BPE_PAYMENT_OTHER, BPE_PAYMENT_TYPES,
  BPE_PAYMENTS_MAX, BPE_PRESENCE, BPE_TA_COMPONENT_TYPES, BPE_TA_DISCOUNT_TYPES, BPE_TRIPS_MAX,
} from '@/lib/constants/bpe';
import {bpeFields, GRID, H2, SECTION, SUBCARD} from './fields';
import {BpeExtraSections, BpePurchaseSection} from './BpeExtraSections';
import {PersonIdField} from './PersonIdField';
import {useBpeLines} from './useBpeLines';

const ADDITIONAL_INFO_MAX = 5000;
const VALIDITY_DAYS = 30;
const pad = (n: number) => String(n).padStart(2, '0');

/** `datetime-local` de `local` mais `days` dias (o validade sugerido do bilhete). */
function plusDays(local: string, days: number): string {
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return '';
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Total estimado (o servidor soma os componentes e abate o desconto). */
function estimate(f: BpeTicketForm): number {
  const n = (s: string) => (Number.isFinite(Number(s)) ? Number(s) : 0);
  return f.components.reduce((s, c) => s + n(c.value), 0) - n(f.discount);
}

export function BpeTicketForm({variant}: { variant: 'normal' | 'ta' }) {
  const router = useRouter();
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const ta = variant === 'ta';
  const {options: lineOptions, isPending: linesPending} = useBpeLines(variant);

  const form = useForm<BpeTicketForm>({resolver: zodResolver(bpeTicketSchema), defaultValues: emptyBpeTicket(variant)});
  const trips = useFieldArray({control: form.control, name: 'trips'});
  const components = useFieldArray({control: form.control, name: 'components'});
  const payments = useFieldArray({control: form.control, name: 'payments'});
  const values = useWatch({control: form.control}) as BpeTicketForm;
  const {text, select} = bpeFields(form.control);

  // Embarque e validade nascem da partida do primeiro trecho (a pessoa ajusta se for diferente).
  const firstDeparture = values.trips[0]?.departure ?? '';
  useEffect(() => {
    if (!firstDeparture) return;
    if (!form.getValues('boarding')) form.setValue('boarding', firstDeparture);
    if (!form.getValues('validity')) form.setValue('validity', plusDays(firstDeparture, VALIDITY_DAYS));
  }, [firstDeparture, form]);

  const onInvalid = () => {
    setSubmitError({message: 'Revise os campos destacados antes de emitir o BP-e.'});
    const first = document.querySelector<HTMLElement>('[aria-invalid="true"]');
    window.setTimeout(() => first?.focus(), 0);
  };

  const submit = form.handleSubmit(async (data) => {
    setSubmitError(null);
    setIsSubmitting(true);
    try {
      const result = await apiClient.emitBpe(toBpeTicketBody(data));
      toast.success('BP-e enviado para processamento', {description: 'Acompanhe a autorização na tela de detalhes.'});
      router.push(`/bpe/detail?key=${encodeURIComponent(result.sk)}`);
    } catch (error) {
      setSubmitError(emitFailure(error, 'Não foi possível enviar o BP-e. Revise os dados e tente novamente.'));
      setIsSubmitting(false);
    }
  }, onInvalid);

  const linked = ta && values.purchase_type === 'linked';
  const multiple = ta && values.purchase_type === 'multiple';
  const hasDiscount = Number(values.discount) > 0;
  const total = estimate(values);
  const noLines = !linesPending && lineOptions.length === 0;
  const passengerModes = PASSENGER_MODES.filter((m) => !(ta && !multiple && m.value === 'none') && !(multiple && m.value !== 'none'));

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="space-y-6 pb-24">
        {noLines && (
          <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            Cadastre antes <Link href="/bpe-lines/new" className="font-semibold underline">uma linha {ta ? 'de voo' : 'do bilhete comum'}</Link>:
            a venda só escolhe a linha e informa data, poltrona e valores.
          </div>
        )}

        <section className={SECTION} aria-labelledby="bpe-trechos">
          <div className="flex items-center justify-between gap-3">
            <h2 id="bpe-trechos" className={H2}>{ta ? 'Voos' : 'Trechos'}</h2>
            <Button type="button" variant="outline" size="sm" disabled={trips.fields.length >= BPE_TRIPS_MAX}
                    onClick={() => trips.append(emptyBpeTrip())}>{ta ? '+ Adicionar voo' : '+ Adicionar trecho'}</Button>
          </div>
          {trips.fields.map((f, i) => (
            <div key={f.id} className={SUBCARD}>
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium text-gray-700">{ta ? 'Voo' : 'Trecho'} {i + 1}</p>
                {trips.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => trips.remove(i)}>Remover</Button>}
              </div>
              <div className={GRID}>
                <FormField control={form.control} name={`trips.${i}.line_id`} render={({field}) => (
                  <FormItem className="sm:col-span-2">
                    <FormLabel>Linha *</FormLabel>
                    <Combobox id={field.name} value={field.value} onValueChange={field.onChange} options={lineOptions}
                              placeholder={linesPending ? 'Carregando…' : 'Selecione a linha…'} searchPlaceholder="Digite o nome da linha…"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                {select(`trips.${i}.leg_type`, 'Tipo de trecho', BPE_LEG_TYPES)}
                {text(`trips.${i}.departure`, 'Partida *', {type: 'datetime-local'})}
                {values.trips[i]?.leg_type === '3' && text(`trips.${i}.connection`, 'Conexão *', {type: 'datetime-local'})}
                {!ta && text(`trips.${i}.seat`, 'Poltrona', {inputMode: 'numeric', maxLength: 3})}
                {!ta && text(`trips.${i}.platform`, 'Plataforma', {maxLength: 20})}
              </div>
            </div>
          ))}
          {form.formState.errors.trips?.message && <p className="text-sm text-danger">{form.formState.errors.trips.message}</p>}
        </section>

        <section className={SECTION} aria-labelledby="bpe-viagem">
          <h2 id="bpe-viagem" className={H2}>Embarque</h2>
          <div className={GRID}>
            {select('presence', 'Presença do comprador', BPE_PRESENCE, 'sm:col-span-2')}
            {text('boarding', 'Embarque *', {type: 'datetime-local'})}
            {text('validity', 'Validade do bilhete *', {type: 'datetime-local'})}
          </div>
        </section>

        {ta && <BpePurchaseSection control={form.control} purchase={values.purchase_type}/>}

        <section className={SECTION} aria-labelledby="bpe-passageiro">
          <h2 id="bpe-passageiro" className={H2}>Passageiro</h2>
          {multiple ? (
            <p className="text-xs text-gray-500">O BP-e múltiplo não identifica passageiro.</p>
          ) : (
            <>
              <FormField control={form.control} name="passenger.mode" render={({field}) => (
                <FormItem className="max-w-md">
                  <FormLabel>Identificação</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={[...passengerModes]}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              {values.passenger.mode === 'avulso' && (
                <div className={GRID}>
                  {text('passenger.name', 'Nome *', {className: 'sm:col-span-2', maxLength: 60})}
                  {select('passenger.doc_type', 'Documento *', BPE_DOC_TYPES)}
                  {text('passenger.doc_number', 'Número do documento *', {maxLength: 20})}
                </div>
              )}
              {values.passenger.mode === 'person' && (
                <PersonIdField control={form.control} setValue={form.setValue} name="passenger.person_id" label="Pessoa *" className="max-w-xl"
                               hint="Pessoa física; o CPF vai como documento do passageiro."/>
              )}
              {values.passenger.mode === 'none' && <p className="text-xs text-gray-500">Obrigatório em viagem interestadual e com desconto.</p>}
            </>
          )}
        </section>

        <BpeExtraSections control={form.control} setValue={form.setValue} values={values} ta={ta}/>

        {!linked && <section className={SECTION} aria-labelledby="bpe-valores">
          <div className="flex items-center justify-between gap-3">
            <h2 id="bpe-valores" className={H2}>Valores</h2>
            <Button type="button" variant="outline" size="sm" disabled={components.fields.length >= BPE_COMPONENTS_MAX}
                    onClick={() => components.append({type: '01', value: ''})}>+ Componente</Button>
          </div>
          {components.fields.map((f, i) => (
            <div key={f.id} className={`${GRID} items-end`}>
              {select(`components.${i}.type`, 'Componente', ta ? BPE_TA_COMPONENT_TYPES : BPE_COMPONENT_TYPES, 'sm:col-span-2')}
              {text(`components.${i}.value`, 'Valor (R$)', {inputMode: 'decimal'})}
              {components.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger justify-self-start" onClick={() => components.remove(i)}>Remover</Button>}
            </div>
          ))}
          <div className={GRID}>
            {text('discount', 'Desconto (R$)', {inputMode: 'decimal'})}
            {hasDiscount && select('discount_type', 'Tipo do desconto', ta ? BPE_TA_DISCOUNT_TYPES : BPE_DISCOUNT_TYPES, 'sm:col-span-2')}
            {hasDiscount && text('discount_desc', 'Descrição do desconto', {maxLength: 100})}
            {hasDiscount && values.discount_type === '99' && text('discount_code', 'Código do desconto', {maxLength: 20})}
          </div>
        </section>}

        {!linked && <section className={SECTION} aria-labelledby="bpe-pagamento">
          <div className="flex items-center justify-between gap-3">
            <h2 id="bpe-pagamento" className={H2}>Pagamento</h2>
            <Button type="button" variant="outline" size="sm" disabled={payments.fields.length >= BPE_PAYMENTS_MAX}
                    onClick={() => payments.append({type: '01', value: '', desc: ''})}>+ Forma de pagamento</Button>
          </div>
          {payments.fields.map((f, i) => (
            <div key={f.id} className={`${GRID} items-end`}>
              {select(`payments.${i}.type`, 'Forma', BPE_PAYMENT_TYPES, 'sm:col-span-2')}
              {text(`payments.${i}.value`, 'Valor (R$)', {inputMode: 'decimal'})}
              {values.payments[i]?.type === BPE_PAYMENT_OTHER && text(`payments.${i}.desc`, 'Descrição', {maxLength: 60})}
              {payments.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger justify-self-start" onClick={() => payments.remove(i)}>Remover</Button>}
            </div>
          ))}
          {form.formState.errors.payments?.message && <p className="text-sm text-danger">{form.formState.errors.payments.message}</p>}
        </section>}

        <section className={SECTION} aria-labelledby="bpe-adicional">
          <h2 id="bpe-adicional" className={H2}>Informações adicionais</h2>
          {ta && !linked && text('iata_number', 'Número do bilhete na IATA', {maxLength: 20})}
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

        <div className="sticky bottom-(--bottomnav-height) -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between md:-mx-8 md:px-8">
          <p className="text-sm text-gray-600">Total estimado R$ {total.toFixed(2).replace('.', ',')}</p>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => router.push('/bpe')}>Cancelar</Button>
            <Button type="submit" variant="brand" disabled={isSubmitting || noLines}>{isSubmitting ? 'Enviando…' : 'Emitir BP-e'}</Button>
          </div>
        </div>
      </form>
    </Form>
  );
}
