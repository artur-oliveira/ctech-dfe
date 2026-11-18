'use client';

import {useState} from 'react';
import {useFieldArray, useForm, useWatch} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {Form, FormField, FormItem, FormLabel} from '@/components/ui/form';
import {Button} from '@aoctech/ui';
import {OptionsSelect} from '@/components/ui/options-select';
import {MunicipalityField} from '@/components/ui/municipality-field';
import {ApiError} from '@/lib/api/client';
import {UF_OPTIONS} from '@/lib/schemas/entity';
import {
  BPE_ACCOMMODATIONS, BPE_ICMS_OPTIONS, BPE_ICMS_WITH_RATE, BPE_MODALS, BPE_SERVICE_TYPES, BPE_TA_ACCOMMODATIONS, BPE_TA_SERVICE_TYPES,
  BPE_TA_TRIP_TYPES, BPE_TM_COMPONENTS_MAX, BPE_TM_COMPONENT_NAME_MAX, BPE_TM_ITEMS_MAX, BPE_TRIP_TYPES, BPE_VARIANTS,
} from '@/lib/constants/bpe';
import {bpeLineSchema, bpeLineToForm, emptyBpeLine, toBpeLine, type BpeLineFormData} from '@/lib/schemas/bpe-line';
import type {BpeLineCreate, BpeLineItemOut} from '@/lib/types/api';
import {BpeTaTaxFields} from './BpeTaTaxFields';
import {bpeFields, GRID, H2, SECTION, SUBCARD} from './fields';

interface Props {
  initialData?: BpeLineItemOut
  onSubmit: (data: BpeLineCreate) => Promise<void>
  loading?: boolean
}

const UF_OR_EXTERIOR = [...UF_OPTIONS, {value: 'EX', label: 'EX - Exterior'}];

export function BpeLineForm({initialData, onSubmit, loading}: Props) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const form = useForm<BpeLineFormData>({
    resolver: zodResolver(bpeLineSchema),
    defaultValues: initialData ? bpeLineToForm(initialData) : emptyBpeLine('normal'),
  });
  const legs = useFieldArray({control: form.control, name: 'legs'});
  const types = useFieldArray({control: form.control, name: 'passenger_types'});
  const values = useWatch({control: form.control}) as BpeLineFormData;
  const {text, select} = bpeFields(form.control);
  const variant = values.variant;
  const ta = variant === 'ta';
  const tm = variant === 'tm';

  const handleSubmit = async (data: BpeLineFormData) => {
    setSubmitError(null);
    try {
      await onSubmit(toBpeLine(bpeLineSchema.parse(data)));
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a linha.');
    }
  };

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} noValidate className="space-y-6">
        <section className={SECTION} aria-labelledby="line-id">
          <h2 id="line-id" className={H2}>Identificação</h2>
          <div className={GRID}>
            {text('name', 'Nome da linha *', {className: 'sm:col-span-2', maxLength: 60})}
            <FormField control={form.control} name="variant" render={({field}) => (
              <FormItem className="sm:col-span-2">
                <FormLabel>Tipo de bilhete *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} disabled={!!initialData}
                               onValueChange={(v) => form.reset({...emptyBpeLine(v as BpeLineFormData['variant']), name: values.name})}
                               options={[...BPE_VARIANTS]}/>
                {initialData && <p className="text-xs text-gray-500">O tipo não muda depois de salvo.</p>}
              </FormItem>
            )}/>
          </div>
        </section>

        {!tm && (
          <section className={SECTION} aria-labelledby="line-percurso">
            <h2 id="line-percurso" className={H2}>{ta ? 'Voo' : 'Percurso'}</h2>
            <div className={GRID}>
              {!ta && select('modal', 'Modal *', BPE_MODALS)}
              <MunicipalityField control={form.control} setValue={form.setValue} name="origin.ibge_code" label="Município de início *"
                                 ufName="origin.uf" cityName="origin.city"/>
              <MunicipalityField control={form.control} setValue={form.setValue} name="destination.ibge_code" label="Município de fim *"
                                 ufName="destination.uf" cityName="destination.city"/>
              {ta ? (
                <>
                  {text('carrier', 'Companhia (sigla) *', {maxLength: 3})}
                  {text('flight_number', 'Número do voo *', {maxLength: 20})}
                  {text('from_airport', 'Aeroporto de origem *', {maxLength: 4})}
                  {text('to_airport', 'Aeroporto de destino *', {maxLength: 4})}
                </>
              ) : (
                <>
                  {text('route_code', 'Código da linha *', {maxLength: 20})}
                  {text('route_name', 'Percurso *', {className: 'sm:col-span-2', maxLength: 100})}
                  {text('prefix', 'Prefixo', {maxLength: 20})}
                </>
              )}
              {select('trip_type', 'Tipo de viagem *', ta ? BPE_TA_TRIP_TYPES : BPE_TRIP_TYPES)}
              {select('service_type', 'Serviço *', ta ? BPE_TA_SERVICE_TYPES : BPE_SERVICE_TYPES)}
              {select('accommodation', 'Acomodação *', ta ? BPE_TA_ACCOMMODATIONS : BPE_ACCOMMODATIONS)}
            </div>
          </section>
        )}

        {variant === 'normal' && (
          <section className={SECTION} aria-labelledby="line-icms">
            <h2 id="line-icms" className={H2}>Imposto (ICMS)</h2>
            <div className={GRID}>
              {select('icms_cst', 'Situação do ICMS *', BPE_ICMS_OPTIONS, 'sm:col-span-2')}
              {BPE_ICMS_WITH_RATE.includes(values.icms_cst) && text('icms_rate', 'Alíquota do ICMS (%)', {inputMode: 'decimal'})}
            </div>
          </section>
        )}

        {ta && (
          <section className={SECTION} aria-labelledby="line-ibs">
            <h2 id="line-ibs" className={H2}>Imposto (IBS e CBS)</h2>
            <BpeTaTaxFields control={form.control} tax={values.tax}/>
          </section>
        )}

        {tm && (
          <>
            <section className={SECTION} aria-labelledby="line-tm">
              <h2 id="line-tm" className={H2}>Grupo de viagem</h2>
              <div className={GRID}>
                {select('modal', 'Modal *', BPE_MODALS)}
                {text('cfop', 'CFOP *', {inputMode: 'numeric', maxLength: 4})}
                {select('group_origin_uf', 'UF de início *', UF_OPTIONS)}
                {select('group_dest_uf', 'UF de fim', UF_OR_EXTERIOR)}
                {text('plate', 'Placa do veículo', {maxLength: 7})}
                {text('prefix', 'Prefixo', {maxLength: 20})}
              </div>
            </section>

            <section className={SECTION} aria-labelledby="line-trechos">
              <div className="flex items-center justify-between gap-3">
                <h2 id="line-trechos" className={H2}>Trechos</h2>
                <Button type="button" variant="outline" size="sm" disabled={legs.fields.length >= BPE_TM_ITEMS_MAX}
                        onClick={() => legs.append({origin_ibge_code: '', dest_ibge_code: '', icms_cst: '40', icms_rate: ''})}>+ Trecho</Button>
              </div>
              {legs.fields.map((f, i) => (
                <div key={f.id} className={SUBCARD}>
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium text-gray-700">Trecho {i + 1}</p>
                    {legs.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => legs.remove(i)}>Remover</Button>}
                  </div>
                  <div className={GRID}>
                    <MunicipalityField control={form.control} name={`legs.${i}.origin_ibge_code`} label="Município de origem *"/>
                    <MunicipalityField control={form.control} name={`legs.${i}.dest_ibge_code`} label="Município de destino"/>
                    {select(`legs.${i}.icms_cst`, 'Situação do ICMS *', BPE_ICMS_OPTIONS)}
                    {BPE_ICMS_WITH_RATE.includes(values.legs[i]?.icms_cst ?? '') && text(`legs.${i}.icms_rate`, 'Alíquota do ICMS (%)', {inputMode: 'decimal'})}
                  </div>
                </div>
              ))}
            </section>

            <section className={SECTION} aria-labelledby="line-pax">
              <div className="flex items-center justify-between gap-3">
                <h2 id="line-pax" className={H2}>Tipos de passageiro</h2>
                <Button type="button" variant="outline" size="sm" disabled={types.fields.length >= BPE_TM_COMPONENTS_MAX}
                        onClick={() => types.append({name: ''})}>+ Tipo</Button>
              </div>
              <p className="text-xs text-gray-500">Os nomes dos componentes de tarifa (inteira, meia, gratuito...); a apuração informa a quantidade de cada um.</p>
              {types.fields.map((f, i) => (
                <div key={f.id} className={`${GRID} items-end`}>
                  {text(`passenger_types.${i}.name`, 'Nome *', {maxLength: BPE_TM_COMPONENT_NAME_MAX, className: 'sm:col-span-2'})}
                  {types.fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger justify-self-start" onClick={() => types.remove(i)}>Remover</Button>}
                </div>
              ))}
            </section>
          </>
        )}

        {submitError && <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{submitError}</div>}
        <div className="flex gap-2">
          <Button type="submit" variant="brand" disabled={loading}>{loading ? 'Salvando…' : 'Salvar'}</Button>
        </div>
      </form>
    </Form>
  );
}
