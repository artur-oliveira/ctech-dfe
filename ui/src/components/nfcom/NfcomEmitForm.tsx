'use client';

import {useMemo, useState} from 'react';
import Link from 'next/link';
import {useFieldArray, useForm, useWatch, type FieldErrors} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {useQuery} from '@tanstack/react-query';
import {useRouter} from 'next/navigation';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {EmitError} from '@/components/ui/emit-error';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {useAuth} from '@/lib/hooks/useAuth';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Button, Textarea} from '@aoctech/ui';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {LengthHint} from '@/components/ui/length-hint';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import {emptyNfcomForm, emptyNfcomItem, nfcomEmitSchema, toNfcomEmit, type NfcomFormData} from '@/lib/schemas/nfcom';
import {NFCOM_ITEMS_MAX, nfcomCompetenceOptions} from '@/lib/constants/nfcom';
import type {NfcomPlanItemOut} from '@/lib/types/api';

const REGISTRY_PAGE_SIZE = 100;
const ADDITIONAL_INFO_MAX = 3000;
const num = (s: string) => (Number.isFinite(Number(s)) ? Number(s) : 0);

/** Total estimado da linha (o servidor recalcula): quantidade x valor - desconto + outras despesas. */
function itemTotal(it: NfcomFormData['items'][number], plan: NfcomPlanItemOut | undefined): number {
  const unit = plan && typeof plan.unit_value === 'string' && plan.unit_value ? num(plan.unit_value) : num(it.unit_value);
  return num(it.quantity) * unit - num(it.discount) + num(it.other_charges);
}

export function NfcomEmitForm() {
  const router = useRouter();
  const {selectedOrg} = useAuth();
  const {config} = useFiscalConfig('nfcom', selectedOrg?.pk);
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const subscribers = useQuery({
    queryKey: queryKeys.nfcomSubscribers.list(selectedOrg?.pk),
    queryFn: () => apiClient.getNfcomSubscribers({limit: REGISTRY_PAGE_SIZE}),
    enabled: !!selectedOrg,
  });
  const plans = useQuery({
    queryKey: queryKeys.nfcomPlans.list(selectedOrg?.pk),
    queryFn: () => apiClient.getNfcomPlans({limit: REGISTRY_PAGE_SIZE}),
    enabled: !!selectedOrg,
  });
  const subscriberOptions = useMemo(
    () => (subscribers.data?.items ?? []).map((s) => ({value: s.sk, label: `${s.name} (${String(s.code ?? '')})`})),
    [subscribers.data],
  );
  const planBySk = useMemo(() => new Map((plans.data?.items ?? []).map((p) => [p.sk, p])), [plans.data]);
  const planOptions = useMemo(
    () => (plans.data?.items ?? []).map((p) => ({value: p.sk, label: `${p.name} (${String(p.code ?? '')})`})),
    [plans.data],
  );
  const competenceOptions = useMemo(() => nfcomCompetenceOptions(), []);

  const form = useForm<NfcomFormData>({resolver: zodResolver(nfcomEmitSchema), defaultValues: emptyNfcomForm()});
  const {fields, append, remove} = useFieldArray({control: form.control, name: 'items'});
  const values = useWatch({control: form.control}) as NfcomFormData;
  const total = values.items.reduce((sum, it) => sum + itemTotal(it, planBySk.get(it.plan_id)), 0);
  const additionalInfo = values.additional_info;

  const onInvalid = (errors: FieldErrors<NfcomFormData>) => {
    setSubmitError({message: 'Revise os campos destacados antes de emitir a NFCom.'});
    const first = document.querySelector<HTMLElement>('[aria-invalid="true"]');
    window.setTimeout(() => first?.focus(), 0);
    void errors;
  };

  const submit = form.handleSubmit(async (data) => {
    setSubmitError(null);
    setIsSubmitting(true);
    try {
      const result = await apiClient.emitNfcom(toNfcomEmit(data));
      toast.success('NFCom enviada para processamento', {description: 'Acompanhe a autorização na tela de detalhes.'});
      router.push(`/nfcom/detail?key=${encodeURIComponent(result.sk)}`);
    } catch (error) {
      setSubmitError(emitFailure(error, 'Não foi possível enviar a NFCom. Revise os dados e tente novamente.'));
      setIsSubmitting(false);
    }
  }, onInvalid);

  const section = 'rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4';
  const grid = 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4';
  const registriesLoaded = !subscribers.isPending && !plans.isPending;
  const missing = registriesLoaded && (subscriberOptions.length === 0 || planOptions.length === 0);

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="space-y-6 pb-24">
        <HomologationBanner environment={config?.environment}/>

        {missing && (
          <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            Cadastre antes {subscriberOptions.length === 0 && <Link href="/nfcom-subscribers/new" className="font-semibold underline">um assinante</Link>}
            {subscriberOptions.length === 0 && planOptions.length === 0 && ' e '}
            {planOptions.length === 0 && <Link href="/nfcom-plans/new" className="font-semibold underline">um plano</Link>}
            : a fatura só escolhe e informa as quantidades.
          </div>
        )}

        <section className={section} aria-labelledby="nfcom-assinante">
          <h2 id="nfcom-assinante" className="text-base font-semibold text-gray-900">Assinante</h2>
          <FormField control={form.control} name="subscriber_id" render={({field}) => (
            <FormItem className="max-w-xl">
              <FormLabel>Assinante *</FormLabel>
              <Combobox id={field.name} value={field.value} onValueChange={field.onChange} options={subscriberOptions}
                        placeholder={subscribers.isPending ? 'Carregando…' : 'Selecione o assinante…'} searchPlaceholder="Digite o nome ou o código…"/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>

        <section className={section} aria-labelledby="nfcom-itens">
          <div className="flex items-center justify-between gap-3">
            <h2 id="nfcom-itens" className="text-base font-semibold text-gray-900">Itens</h2>
            <Button type="button" variant="outline" size="sm" disabled={fields.length >= NFCOM_ITEMS_MAX}
                    onClick={() => append(emptyNfcomItem())}>+ Adicionar item</Button>
          </div>
          {fields.map((f, i) => {
            const plan = planBySk.get(values.items[i]?.plan_id ?? '');
            const fixedValue = !!plan && typeof plan.unit_value === 'string' && plan.unit_value !== '';
            return (
              <div key={f.id} className="space-y-4 rounded-lg border border-gray-100 bg-gray-50 p-3 md:p-4">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium text-gray-700">Item {i + 1}</p>
                  {fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => remove(i)}>Remover</Button>}
                </div>
                <div className={grid}>
                  <FormField control={form.control} name={`items.${i}.plan_id`} render={({field}) => (
                    <FormItem className="sm:col-span-2">
                      <FormLabel>Plano *</FormLabel>
                      <Combobox id={field.name} value={field.value} onValueChange={field.onChange} options={planOptions}
                                placeholder={plans.isPending ? 'Carregando…' : 'Selecione o plano…'} searchPlaceholder="Digite o nome ou o código…"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name={`items.${i}.quantity`} render={({field}) => (
                    <FormItem>
                      <FormLabel>Quantidade *</FormLabel>
                      <Input {...field} id={field.name} inputMode="decimal" className="w-full"
                             onChange={(e) => field.onChange(e.target.value.replace(',', '.').replace(/[^\d.]/g, ''))}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  {plan && !fixedValue && (
                    <FormField control={form.control} name={`items.${i}.unit_value`} render={({field}) => (
                      <FormItem>
                        <FormLabel>Valor unitário *</FormLabel>
                        <CurrencyInput id={field.name} decimalPlaces={2} maxDecimalPlaces={8} value={field.value} className="w-full" onChange={field.onChange}/>
                        <FormMessage/>
                      </FormItem>
                    )}/>
                  )}
                  {fixedValue && <p className="self-end pb-2 text-sm text-gray-600">Valor do plano: R$ {num(String(plan?.unit_value)).toFixed(2).replace('.', ',')}</p>}
                  <FormField control={form.control} name={`items.${i}.discount`} render={({field}) => (
                    <FormItem>
                      <FormLabel>Desconto</FormLabel>
                      <CurrencyInput id={field.name} value={field.value} className="w-full" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name={`items.${i}.other_charges`} render={({field}) => (
                    <FormItem>
                      <FormLabel>Outras despesas</FormLabel>
                      <CurrencyInput id={field.name} value={field.value} className="w-full" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                </div>
              </div>
            );
          })}
          {form.formState.errors.items?.message && <p className="text-sm text-danger">{form.formState.errors.items.message}</p>}
        </section>

        <section className={section} aria-labelledby="nfcom-fatura">
          <h2 id="nfcom-fatura" className="text-base font-semibold text-gray-900">Fatura</h2>
          <FormField control={form.control} name="prepaid" render={({field}) => (
            <label className="flex min-h-11 items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
              Serviço pré-pago (dispensa a fatura)
            </label>
          )}/>
          {!values.prepaid && (
            <div className={grid}>
              <FormField control={form.control} name="invoice.competence" render={({field}) => (
                <FormItem>
                  <FormLabel>Competência *</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={competenceOptions}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="invoice.due_date" render={({field}) => (
                <FormItem>
                  <FormLabel>Vencimento *</FormLabel>
                  <Input {...field} id={field.name} type="date" className="w-full"/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="invoice.bar_code" render={({field}) => (
                <FormItem className="sm:col-span-2">
                  <FormLabel>Código de barras *</FormLabel>
                  <Input {...field} id={field.name} inputMode="numeric" maxLength={48} className="w-full"
                         onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>
          )}
          <FormField control={form.control} name="additional_info" render={({field}) => (
            <FormItem>
              <FormLabel>Informações adicionais</FormLabel>
              <Textarea {...field} id={field.name} rows={3} maxLength={ADDITIONAL_INFO_MAX}/>
              <LengthHint value={additionalInfo} max={ADDITIONAL_INFO_MAX}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>

        <EmitError failure={submitError}/>

        <div className="sticky bottom-(--bottomnav-height) -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between md:-mx-8 md:px-8">
          <p className="text-sm text-gray-600">Total estimado R$ {total.toFixed(2).replace('.', ',')}</p>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => router.push('/nfcom')}>Cancelar</Button>
            <Button type="submit" variant="brand" disabled={isSubmitting || missing}>{isSubmitting ? 'Enviando…' : 'Emitir NFCom'}</Button>
          </div>
        </div>
      </form>
    </Form>
  );
}
