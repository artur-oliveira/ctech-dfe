'use client';

import {useState} from 'react';
import {useFieldArray, useForm, useWatch} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {useRouter} from 'next/navigation';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {EmitError} from '@/components/ui/emit-error';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Button, Textarea} from '@aoctech/ui';
import {UF_OPTIONS} from '@/lib/schemas/entity';
import {bpeTicketSchema, emptyBpeTicket, emptyBpeTrip, toBpeTicketBody, type BpeTicketForm} from '@/lib/schemas/bpe';
import {
  BPE_ACCOMMODATIONS, BPE_COMPONENT_TYPES, BPE_COMPONENTS_MAX, BPE_DISCOUNT_TYPES, BPE_DOC_TYPES, BPE_ICMS_OPTIONS, BPE_ICMS_WITH_RATE,
  BPE_LEG_TYPES, BPE_MODALS, BPE_PAYMENT_OTHER, BPE_PAYMENT_TYPES, BPE_PAYMENTS_MAX, BPE_PRESENCE, BPE_SERVICE_TYPES,
  BPE_TA_ACCOMMODATIONS, BPE_TA_COMPONENT_TYPES, BPE_TA_DISCOUNT_TYPES, BPE_TA_SERVICE_TYPES, BPE_TA_TRIP_TYPES, BPE_TRIP_TYPES, BPE_TRIPS_MAX,
} from '@/lib/constants/bpe';
import {bpeFields, GRID, H2, SECTION, SUBCARD} from './fields';

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

  const form = useForm<BpeTicketForm>({resolver: zodResolver(bpeTicketSchema), defaultValues: emptyBpeTicket(variant)});
  const trips = useFieldArray({control: form.control, name: 'trips'});
  const components = useFieldArray({control: form.control, name: 'components'});
  const payments = useFieldArray({control: form.control, name: 'payments'});
  const values = useWatch({control: form.control}) as BpeTicketForm;
  const {text, select} = bpeFields(form.control);

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

  const hasDiscount = Number(values.discount) > 0;
  const total = estimate(values);

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="space-y-6 pb-24">
        <section className={SECTION} aria-labelledby="bpe-viagem">
          <h2 id="bpe-viagem" className={H2}>Viagem</h2>
          <div className={GRID}>
            {!ta && select('modal', 'Modal', BPE_MODALS)}
            {select('presence', 'Presença do comprador', BPE_PRESENCE, 'sm:col-span-2')}
          </div>
          <div className={GRID}>
            {select('origin.uf', 'UF de início', UF_OPTIONS)}
            {text('origin.ibge_code', 'Município de início (código IBGE)', {inputMode: 'numeric'})}
            {select('destination.uf', 'UF de fim', UF_OPTIONS)}
            {text('destination.ibge_code', 'Município de fim (código IBGE)', {inputMode: 'numeric'})}
          </div>
          <div className={GRID}>
            {!ta && text('passage.from_code', 'Código do local de origem')}
            {!ta && text('passage.from_name', 'Local de origem', {className: 'sm:col-span-1'})}
            {!ta && text('passage.to_code', 'Código do local de destino')}
            {!ta && text('passage.to_name', 'Local de destino')}
            {text('passage.boarding', 'Embarque', {type: 'datetime-local'})}
            {text('passage.validity', 'Validade do bilhete', {type: 'datetime-local'})}
          </div>
        </section>

        <section className={SECTION} aria-labelledby="bpe-passageiro">
          <h2 id="bpe-passageiro" className={H2}>Passageiro</h2>
          <FormField control={form.control} name="passage.identified" render={({field}) => (
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
              Identificar o passageiro (obrigatório em viagem interestadual e com desconto)
            </label>
          )}/>
          {values.passage.identified && (
            <div className={GRID}>
              {text('passage.passenger.name', 'Nome', {className: 'sm:col-span-2'})}
              {select('passage.passenger.doc_type', 'Documento', BPE_DOC_TYPES)}
              {text('passage.passenger.doc_number', 'Número do documento')}
            </div>
          )}
        </section>

        <section className={SECTION} aria-labelledby="bpe-trechos">
          <div className="flex items-center justify-between gap-3">
            <h2 id="bpe-trechos" className={H2}>{ta ? 'Voos' : 'Trechos'}</h2>
            <Button type="button" variant="outline" size="sm" disabled={trips.fields.length >= BPE_TRIPS_MAX}
                    onClick={() => trips.append(emptyBpeTrip(variant))}>{ta ? '+ Adicionar voo' : '+ Adicionar trecho'}</Button>
          </div>
          {trips.fields.map((f, i) => (
            <div key={f.id} className={SUBCARD}>
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium text-gray-700">{ta ? 'Voo' : 'Trecho'} {i + 1}</p>
                {trips.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => trips.remove(i)}>Remover</Button>}
              </div>
              <div className={GRID}>
                {ta ? (
                  <>
                    {text(`trips.${i}.flight_number`, 'Número do voo')}
                    {text(`trips.${i}.carrier`, 'Companhia (sigla)', {maxLength: 3})}
                    {text(`trips.${i}.from_airport`, 'Aeroporto de origem', {maxLength: 4})}
                    {text(`trips.${i}.to_airport`, 'Aeroporto de destino', {maxLength: 4})}
                  </>
                ) : (
                  <>
                    {text(`trips.${i}.route_code`, 'Código da linha')}
                    {text(`trips.${i}.route_name`, 'Linha (percurso)', {className: 'sm:col-span-2'})}
                    {text(`trips.${i}.prefix`, 'Prefixo')}
                  </>
                )}
                {select(`trips.${i}.trip_type`, 'Tipo de viagem', ta ? BPE_TA_TRIP_TYPES : BPE_TRIP_TYPES)}
                {select(`trips.${i}.service_type`, 'Serviço', ta ? BPE_TA_SERVICE_TYPES : BPE_SERVICE_TYPES)}
                {select(`trips.${i}.accommodation`, 'Acomodação', ta ? BPE_TA_ACCOMMODATIONS : BPE_ACCOMMODATIONS)}
                {select(`trips.${i}.leg_type`, 'Tipo de trecho', BPE_LEG_TYPES)}
                {text(`trips.${i}.departure`, 'Partida', {type: 'datetime-local'})}
                {values.trips[i]?.leg_type === '3' && text(`trips.${i}.connection`, 'Conexão', {type: 'datetime-local'})}
                {!ta && text(`trips.${i}.seat`, 'Poltrona', {inputMode: 'numeric', maxLength: 3})}
                {!ta && text(`trips.${i}.platform`, 'Plataforma')}
              </div>
            </div>
          ))}
          {form.formState.errors.trips?.message && <p className="text-sm text-danger">{form.formState.errors.trips.message}</p>}
        </section>

        <section className={SECTION} aria-labelledby="bpe-valores">
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
            {hasDiscount && text('discount_desc', 'Descrição do desconto')}
            {hasDiscount && values.discount_type === '99' && text('discount_code', 'Código do desconto')}
          </div>
        </section>

        <section className={SECTION} aria-labelledby="bpe-pagamento">
          <div className="flex items-center justify-between gap-3">
            <h2 id="bpe-pagamento" className={H2}>Pagamento</h2>
            <Button type="button" variant="outline" size="sm" disabled={payments.fields.length >= BPE_PAYMENTS_MAX}
                    onClick={() => payments.append({type: '01', value: '', desc: ''})}>+ Forma de pagamento</Button>
          </div>
          {payments.fields.map((f, i) => (
            <div key={f.id} className={`${GRID} items-end`}>
              {select(`payments.${i}.type`, 'Forma', BPE_PAYMENT_TYPES, 'sm:col-span-2')}
              {text(`payments.${i}.value`, 'Valor (R$)', {inputMode: 'decimal'})}
              {values.payments[i]?.type === BPE_PAYMENT_OTHER && text(`payments.${i}.desc`, 'Descrição')}
              {payments.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger justify-self-start" onClick={() => payments.remove(i)}>Remover</Button>}
            </div>
          ))}
          {form.formState.errors.payments?.message && <p className="text-sm text-danger">{form.formState.errors.payments.message}</p>}
        </section>

        <section className={SECTION} aria-labelledby="bpe-imposto">
          <h2 id="bpe-imposto" className={H2}>{ta ? 'Imposto (IBS e CBS)' : 'Imposto (ICMS)'}</h2>
          {ta ? (
            <>
              <div className={GRID}>
                {text('tax.cst', 'CST do IBS e CBS (3 dígitos)', {inputMode: 'numeric', maxLength: 3})}
                {text('tax.class_trib', 'Classificação tributária (6 dígitos)', {inputMode: 'numeric', maxLength: 6})}
                {text('tax.ibs_uf_rate', 'Alíquota do IBS da UF (%)', {inputMode: 'decimal'})}
                {text('tax.ibs_mun_rate', 'Alíquota do IBS do município (%)', {inputMode: 'decimal'})}
                {text('tax.cbs_rate', 'Alíquota da CBS (%)', {inputMode: 'decimal'})}
                {text('iata_number', 'Número do bilhete na IATA')}
              </div>
              <FormField control={form.control} name="tax.donation" render={({field}) => (
                <label className="flex items-center gap-2 text-sm text-gray-700">
                  <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
                  Operação de doação
                </label>
              )}/>
              <p className="text-xs text-gray-500">Sem alíquotas, só o CST e a classificação seguem no documento (situações sem tributo). O servidor calcula os valores sobre o total pago.</p>
            </>
          ) : (
            <div className={GRID}>
              {select('icms.cst', 'Situação do ICMS', BPE_ICMS_OPTIONS, 'sm:col-span-2')}
              {BPE_ICMS_WITH_RATE.includes(values.icms.cst) && text('icms.rate', 'Alíquota do ICMS (%)', {inputMode: 'decimal'})}
            </div>
          )}
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
