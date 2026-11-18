'use client';

import {useState} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Button} from '@aoctech/ui';
import {CurrencyInput} from '@/components/ui/currency-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {LengthHint} from '@/components/ui/length-hint';
import {ApiError} from '@/lib/api/client';
import {
  NFCOM_CFOP_OPTIONS,
  NFCOM_CLASS_CODE_LENGTH,
  NFCOM_ICMS_OPTIONS,
  NFCOM_ICMS_WITH_RATE,
  NFCOM_PIS_COFINS_CST,
  NFCOM_UNIT_OPTIONS,
} from '@/lib/constants/nfcom';
import {emptyNfcomPlan, type NfcomPlanFormData, nfcomPlanSchema, toNfcomPlan} from '@/lib/schemas/nfcom';
import type {NfcomPlanCreate, NfcomPlanItemOut} from '@/lib/types/api';

interface Props {
  initialData?: NfcomPlanItemOut
  onSubmit: (data: NfcomPlanCreate) => Promise<void>
  loading?: boolean
}

const SEM_CST = 'sem_cst';
const NO_TAX = {value: '', label: 'Não informar'};
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const group = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});

function toFormData(p: NfcomPlanItemOut): NfcomPlanFormData {
  return {
    name: str(p.name), code: str(p.code), description: str(p.description), class_code: str(p.class_code), unit: str(p.unit) || '4',
    unit_value: str(p.unit_value), cfop: str(p.cfop),
    icms_cst: str(group(p.icms).cst) || '00', icms_rate: str(group(p.icms).rate),
    pis_cst: str(group(p.pis).cst), pis_rate: str(group(p.pis).rate),
    cofins_cst: str(group(p.cofins).cst), cofins_rate: str(group(p.cofins).rate),
  };
}

export function NfcomPlanForm({initialData, onSubmit, loading}: Props) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const form = useForm<NfcomPlanFormData>({
    resolver: zodResolver(nfcomPlanSchema),
    defaultValues: initialData ? toFormData(initialData) : emptyNfcomPlan(),
  });
  const [icms, pis, cofins] = useWatch({control: form.control, name: ['icms_cst', 'pis_cst', 'cofins_cst']});

  const handleSubmit = async (data: NfcomPlanFormData) => {
    setSubmitError(null);
    try {
      await onSubmit(toNfcomPlan(nfcomPlanSchema.parse(data)));
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o plano.');
    }
  };

  const text = (n: keyof NfcomPlanFormData, label: string, props: {max: number; placeholder?: string; digits?: boolean; className?: string}) => (
    <FormField control={form.control} name={n} render={({field}) => (
      <FormItem className={props.className}>
        <FormLabel>{label}</FormLabel>
        <Input {...field} value={field.value ?? ''} id={field.name} maxLength={props.max} className="w-full" placeholder={props.placeholder}
               inputMode={props.digits ? 'numeric' : undefined}
               onChange={(e) => field.onChange(props.digits ? e.target.value.replace(/\D/g, '') : e.target.value)}/>
        {!props.digits && <LengthHint value={field.value} max={props.max}/>}
        <FormMessage/>
      </FormItem>
    )}/>
  );
  const select = (n: keyof NfcomPlanFormData, label: string, options: readonly { value: string; label: string }[], className?: string) => (
    <FormField control={form.control} name={n} render={({field}) => (
      <FormItem className={className}>
        <FormLabel>{label}</FormLabel>
        <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange} options={[...options]}/>
        <FormMessage/>
      </FormItem>
    )}/>
  );
  const percent = (n: keyof NfcomPlanFormData, label: string) => (
    <FormField control={form.control} name={n} render={({field}) => (
      <FormItem>
        <FormLabel>{label}</FormLabel>
        <CurrencyInput id={field.name} unit="percent" decimalPlaces={2} maxDecimalPlaces={4} value={field.value ?? ''} className="w-full" onChange={field.onChange}/>
        <FormMessage/>
      </FormItem>
    )}/>
  );
  const grid = 'grid grid-cols-1 gap-4 sm:grid-cols-2';

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-6">
        <div className={grid}>
          {text('name', 'Nome do plano *', {max: 60, placeholder: 'Internet 100 Mbps'})}
          {text('code', 'Código *', {max: 60, placeholder: 'PLANO100'})}
          {text('description', 'Descrição na fatura *', {max: 120, className: 'sm:col-span-2'})}
          {text('class_code', `Classificação (cClass, ${NFCOM_CLASS_CODE_LENGTH} dígitos) *`, {max: NFCOM_CLASS_CODE_LENGTH, digits: true, placeholder: '0100301'})}
          {select('unit', 'Unidade *', NFCOM_UNIT_OPTIONS)}
          <FormField control={form.control} name="unit_value" render={({field}) => (
            <FormItem className="sm:col-span-2">
              <FormLabel>Valor padrão (opcional)</FormLabel>
              <CurrencyInput id={field.name} decimalPlaces={2} maxDecimalPlaces={8} value={field.value ?? ''} className="w-full" onChange={field.onChange}/>
              <p className="text-xs text-gray-500">Com valor padrão a fatura só informa a quantidade; sem ele, informa o valor a cada fatura.</p>
              <FormMessage/>
            </FormItem>
          )}/>
        </div>

        <fieldset className="space-y-4 rounded-xl border border-gray-200 bg-white p-4 md:p-6">
          <legend className="px-1 text-base font-semibold text-gray-900">Tributação</legend>
          <div className={grid}>
            {select('icms_cst', 'ICMS *', NFCOM_ICMS_OPTIONS)}
            {icms !== SEM_CST && select('cfop', 'CFOP *', NFCOM_CFOP_OPTIONS)}
            {NFCOM_ICMS_WITH_RATE.includes(icms) && percent('icms_rate', 'Alíquota do ICMS (%)')}
            {select('pis_cst', 'PIS', [NO_TAX, ...NFCOM_PIS_COFINS_CST])}
            {pis && percent('pis_rate', 'Alíquota do PIS (%)')}
            {select('cofins_cst', 'COFINS', [NO_TAX, ...NFCOM_PIS_COFINS_CST])}
            {cofins && percent('cofins_rate', 'Alíquota da COFINS (%)')}
          </div>
        </fieldset>

        {submitError && <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{submitError}</div>}
        <div className="flex gap-2">
          <Button type="submit" variant="brand" disabled={loading}>{loading ? 'Salvando…' : 'Salvar'}</Button>
        </div>
      </form>
    </Form>
  );
}
