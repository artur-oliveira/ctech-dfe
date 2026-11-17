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
  BPE_ACCOMMODATIONS, BPE_BUYER_DOC_KINDS, BPE_COMPONENT_TYPES, BPE_COMPONENTS_MAX, BPE_DISCOUNT_TYPES, BPE_DOC_TYPES, BPE_ICMS_OPTIONS, BPE_ICMS_WITH_RATE,
  BPE_LEG_TYPES, BPE_MODALS, BPE_PAYMENT_OTHER, BPE_PAYMENT_TYPES, BPE_PAYMENTS_MAX, BPE_PRESENCE, BPE_SERVICE_TYPES,
  BPE_CBS_RATE_FROM_YEAR, BPE_GOV_ENTITIES, BPE_GOV_OPERATIONS, BPE_IBS_RATE_FROM_YEAR, BPE_TA_CLASSES, BPE_TA_NO_TAX_CST,
  BPE_TA_ACCOMMODATIONS, BPE_TA_COMPONENT_TYPES, BPE_TA_DISCOUNT_TYPES, BPE_TA_SERVICE_TYPES, BPE_TA_TRIP_TYPES, BPE_TRIP_TYPES, BPE_TRIPS_MAX,
} from '@/lib/constants/bpe';
import {MunicipalityField} from '@/components/ui/municipality-field';
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
  const year = new Date().getFullYear();
  const taClass = BPE_TA_CLASSES.find((c) => c.value === values.tax.class_trib);
  const taTaxed = taClass !== undefined && taClass.cst !== BPE_TA_NO_TAX_CST;
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
            <MunicipalityField control={form.control} setValue={form.setValue} name="origin.ibge_code" label="Município de início" ufName="origin.uf"/>
            {select('destination.uf', 'UF de fim', UF_OPTIONS)}
            <MunicipalityField control={form.control} setValue={form.setValue} name="destination.ibge_code" label="Município de fim" ufName="destination.uf"/>
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

        <section className={SECTION} aria-labelledby="bpe-comprador">
          <h2 id="bpe-comprador" className={H2}>Comprador</h2>
          <FormField control={form.control} name="buyer.enabled" render={({field}) => (
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
              Informar o comprador (opcional; agência ou quem pagou o bilhete)
            </label>
          )}/>
          {values.buyer.enabled && (
            <div className={GRID}>
              {text('buyer.name', 'Nome ou razão social', {className: 'sm:col-span-2'})}
              {select('buyer.doc_kind', 'Documento', BPE_BUYER_DOC_KINDS)}
              {text('buyer.document', 'Número do documento')}
              {text('buyer.ie', 'Inscrição estadual')}
              {text('buyer.street', 'Logradouro', {className: 'sm:col-span-2'})}
              {text('buyer.number', 'Número')}
              {text('buyer.complement', 'Complemento')}
              {text('buyer.neighborhood', 'Bairro')}
              <MunicipalityField control={form.control} setValue={form.setValue} name="buyer.ibge_code" label="Município"
                                 ufName="buyer.uf" cityName="buyer.city"/>
              {select('buyer.uf', 'UF', UF_OPTIONS)}
              {text('buyer.postal_code', 'CEP', {inputMode: 'numeric'})}
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
                {select('tax.class_trib', 'Classificação tributária', BPE_TA_CLASSES, 'sm:col-span-2 lg:col-span-3')}
                {text('iata_number', 'Número do bilhete na IATA')}
                {taTaxed && year >= BPE_IBS_RATE_FROM_YEAR && text('tax.ibs_uf_rate', 'Alíquota do IBS da UF (%)', {inputMode: 'decimal'})}
                {taTaxed && year >= BPE_IBS_RATE_FROM_YEAR && text('tax.ibs_mun_rate', 'Alíquota do IBS do município (%)', {inputMode: 'decimal'})}
                {taTaxed && year >= BPE_CBS_RATE_FROM_YEAR && text('tax.cbs_rate', 'Alíquota da CBS (%)', {inputMode: 'decimal'})}
                {text('tax.total_taxes', 'Total aproximado de tributos (R$)', {inputMode: 'decimal'})}
              </div>
              {taClass && taClass.donation !== 0 && (
                <FormField control={form.control} name="tax.donation" render={({field}) => (
                  <label className="flex items-center gap-2 text-sm text-gray-700">
                    <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
                    Operação de doação{taClass.donation === 1 ? ' (obrigatório nesta classificação)' : ''}
                  </label>
                )}/>
              )}
              {taClass?.reversal && (
                <div className={GRID}>
                  {text('tax.reversal.ibs', 'IBS estornado (R$)', {inputMode: 'decimal'})}
                  {text('tax.reversal.cbs', 'CBS estornada (R$)', {inputMode: 'decimal'})}
                </div>
              )}
              {taTaxed && (
                <>
                  <FormField control={form.control} name="tax.government.enabled" render={({field}) => (
                    <label className="flex items-center gap-2 text-sm text-gray-700">
                      <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
                      Compra governamental
                    </label>
                  )}/>
                  {values.tax.government.enabled && (
                    <div className={SUBCARD}>
                      <div className={GRID}>
                        {select('tax.government.entity', 'Ente comprador', BPE_GOV_ENTITIES)}
                        {select('tax.government.operation', 'Tipo da operação', BPE_GOV_OPERATIONS, 'sm:col-span-2')}
                        {text('tax.government.reducer', 'Redutor da compra governamental (%)', {inputMode: 'decimal'})}
                      </div>
                      {['2', '3'].includes(values.tax.government.operation) && text('tax.government.references', 'Chave de acesso do DFe anterior (uma por linha ou separadas por espaço)')}
                    </div>
                  )}
                  <div className={GRID}>
                    {text('tax.devolution.ibs_uf', 'Devolução de IBS da UF (R$)', {inputMode: 'decimal'})}
                    {text('tax.devolution.ibs_mun', 'Devolução de IBS do município (R$)', {inputMode: 'decimal'})}
                    {text('tax.devolution.cbs', 'Devolução de CBS (R$)', {inputMode: 'decimal'})}
                  </div>
                </>
              )}
              <FormField control={form.control} name="tax.simples.enabled" render={({field}) => (
                <label className="flex items-center gap-2 text-sm text-gray-700">
                  <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
                  Emitente optante do Simples Nacional (grupo de composição do IBS e da CBS)
                </label>
              )}/>
              {values.tax.simples.enabled && (
                <div className={GRID}>
                  {text('tax.simples.ibs_rate', 'Alíquota efetiva do IBS (%)', {inputMode: 'decimal'})}
                  {text('tax.simples.cbs_rate', 'Alíquota efetiva da CBS (%)', {inputMode: 'decimal'})}
                </div>
              )}
              {text('tax.fiscal_info', 'Informações adicionais de interesse do Fisco')}
              <p className="text-xs text-gray-500">
                A classificação vem da tabela oficial e define o CST. As alíquotas de IBS e CBS fixadas em lei (2026 a 2028 para o IBS e 2026 para a CBS)
                são aplicadas pelo servidor, sobre o total pago, junto com a redução da classe, o redutor e o valor total do documento.
              </p>
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
