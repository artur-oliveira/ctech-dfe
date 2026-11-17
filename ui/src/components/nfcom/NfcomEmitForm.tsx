'use client';

import {useState} from 'react';
import {useFieldArray, useForm, useWatch, type FieldErrors} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {useRouter} from 'next/navigation';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {EmitError} from '@/components/ui/emit-error';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {useAuth} from '@/lib/hooks/useAuth';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Button, Textarea} from '@aoctech/ui';
import {OptionsSelect} from '@/components/ui/options-select';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import {UF_OPTIONS} from '@/lib/schemas/entity';
import {emptyNfcomForm, emptyNfcomItem, nfcomEmitSchema, toNfcomEmit, type NfcomFormData} from '@/lib/schemas/nfcom';
import {
  NFCOM_ICMS_OPTIONS,
  NFCOM_ICMS_WITH_RATE,
  NFCOM_IE_INDICATORS,
  NFCOM_ITEMS_MAX,
  NFCOM_PIS_COFINS_CST,
  NFCOM_SERVICE_TYPES,
  NFCOM_SUBSCRIBER_TYPES,
  NFCOM_UNIT_OPTIONS,
} from '@/lib/constants/nfcom';

const SEM_CST = 'sem_cst';
const DOCUMENT_KINDS = [
  {value: 'cnpj', label: 'CNPJ'},
  {value: 'cpf', label: 'CPF'},
  {value: 'other', label: 'Outro identificador'},
];
const NO_TAX = {value: '', label: 'Não informar'};

/** Total estimado do item (o servidor recalcula): quantidade x valor - desconto + outras despesas. */
function itemTotal(it: NfcomFormData['items'][number]): number {
  const n = (s: string) => (Number.isFinite(Number(s)) ? Number(s) : 0);
  return n(it.quantity) * n(it.unit_value) - n(it.discount) + n(it.other_charges);
}

export function NfcomEmitForm() {
  const router = useRouter();
  const {selectedOrg} = useAuth();
  const {config} = useFiscalConfig('nfcom', selectedOrg?.pk);
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const form = useForm<NfcomFormData>({resolver: zodResolver(nfcomEmitSchema), defaultValues: emptyNfcomForm()});
  const {fields, append, remove} = useFieldArray({control: form.control, name: 'items'});
  const values = useWatch({control: form.control}) as NfcomFormData;
  const total = values.items.reduce((sum, it) => sum + itemTotal(it), 0);

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

  const text = (name: string, label: string, props: {type?: string; className?: string; inputMode?: 'numeric' | 'decimal'} = {}) => (
    <FormField control={form.control} name={name as never} render={({field}) => (
      <FormItem className={props.className}>
        <FormLabel>{label}</FormLabel>
        <Input {...field} value={(field.value as string) ?? ''} id={field.name} type={props.type} inputMode={props.inputMode}/>
        <FormMessage/>
      </FormItem>
    )}/>
  );
  const select = (name: string, label: string, options: readonly {value: string; label: string}[], className?: string) => (
    <FormField control={form.control} name={name as never} render={({field}) => (
      <FormItem className={className}>
        <FormLabel>{label}</FormLabel>
        <OptionsSelect id={field.name} value={field.value as string} onValueChange={field.onChange} options={[...options]}/>
        <FormMessage/>
      </FormItem>
    )}/>
  );
  const section = 'rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4';
  const grid = 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4';

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="space-y-6 pb-24">
        <HomologationBanner environment={config?.environment}/>

        <section className={section} aria-labelledby="nfcom-cliente">
          <h2 id="nfcom-cliente" className="text-base font-semibold text-gray-900">Cliente (destinatário)</h2>
          <div className={grid}>
            {text('customer.name', 'Nome', {className: 'sm:col-span-2'})}
            {select('customer.document_kind', 'Documento', DOCUMENT_KINDS)}
            {text('customer.document', values.customer.document_kind === 'other' ? 'Identificador' : values.customer.document_kind.toUpperCase())}
            {select('customer.ie_indicator', 'Situação na inscrição estadual', NFCOM_IE_INDICATORS, 'sm:col-span-2')}
            {values.customer.ie_indicator !== '9' && text('customer.ie', values.customer.ie_indicator === '2' ? 'IE (ISENTO)' : 'Inscrição estadual')}
          </div>
          <div className={grid}>
            {text('customer.address.street', 'Logradouro', {className: 'sm:col-span-2'})}
            {text('customer.address.number', 'Número')}
            {text('customer.address.complement', 'Complemento')}
            {text('customer.address.neighborhood', 'Bairro')}
            {text('customer.address.city', 'Município')}
            {text('customer.address.ibge_code', 'Código IBGE do município', {inputMode: 'numeric'})}
            {select('customer.address.uf', 'UF', UF_OPTIONS)}
            {text('customer.address.postal_code', 'CEP', {inputMode: 'numeric'})}
            {text('customer.address.phone', 'Telefone', {inputMode: 'numeric'})}
            {text('customer.address.email', 'E-mail', {type: 'email'})}
          </div>
        </section>

        <section className={section} aria-labelledby="nfcom-assinante">
          <h2 id="nfcom-assinante" className="text-base font-semibold text-gray-900">Assinante</h2>
          <div className={grid}>
            {text('subscriber.code', 'Código do assinante')}
            {select('subscriber.type', 'Tipo de assinante', NFCOM_SUBSCRIBER_TYPES)}
            {select('subscriber.service_used', 'Serviço utilizado', NFCOM_SERVICE_TYPES, 'sm:col-span-2')}
            {text('subscriber.contract_number', 'Número do contrato')}
            {text('subscriber.contract_start', 'Início do contrato', {type: 'date'})}
            {text('subscriber.contract_end', 'Fim do contrato', {type: 'date'})}
            {text('subscriber.primary_terminal', 'Terminal principal', {inputMode: 'numeric'})}
          </div>
        </section>

        <section className={section} aria-labelledby="nfcom-itens">
          <div className="flex items-center justify-between gap-3">
            <h2 id="nfcom-itens" className="text-base font-semibold text-gray-900">Itens</h2>
            <Button type="button" variant="outline" size="sm" disabled={fields.length >= NFCOM_ITEMS_MAX}
                    onClick={() => append(emptyNfcomItem())}>+ Adicionar item</Button>
          </div>
          {fields.map((f, i) => {
            const it = values.items[i];
            const semCst = it?.icms_cst === SEM_CST;
            return (
              <div key={f.id} className="space-y-4 rounded-lg border border-gray-100 bg-gray-50 p-3 md:p-4">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium text-gray-700">Item {i + 1}</p>
                  {fields.length > 1 && <Button type="button" variant="ghost" size="sm" className="text-danger" onClick={() => remove(i)}>Remover</Button>}
                </div>
                <div className={grid}>
                  {text(`items.${i}.code`, 'Código')}
                  {text(`items.${i}.description`, 'Descrição', {className: 'sm:col-span-2'})}
                  {text(`items.${i}.class_code`, 'Classificação (cClass)', {inputMode: 'numeric'})}
                  {select(`items.${i}.unit`, 'Unidade', NFCOM_UNIT_OPTIONS)}
                  {text(`items.${i}.quantity`, 'Quantidade', {inputMode: 'decimal'})}
                  {text(`items.${i}.unit_value`, 'Valor unitário (R$)', {inputMode: 'decimal'})}
                  {text(`items.${i}.discount`, 'Desconto (R$)', {inputMode: 'decimal'})}
                  {text(`items.${i}.other_charges`, 'Outras despesas (R$)', {inputMode: 'decimal'})}
                  {select(`items.${i}.icms_cst`, 'ICMS', NFCOM_ICMS_OPTIONS, 'sm:col-span-2')}
                  {!semCst && text(`items.${i}.cfop`, 'CFOP', {inputMode: 'numeric'})}
                  {NFCOM_ICMS_WITH_RATE.includes(it?.icms_cst ?? '') && text(`items.${i}.icms_rate`, 'Alíquota do ICMS (%)', {inputMode: 'decimal'})}
                  {select(`items.${i}.pis_cst`, 'PIS', [NO_TAX, ...NFCOM_PIS_COFINS_CST])}
                  {it?.pis_cst && text(`items.${i}.pis_rate`, 'Alíquota do PIS (%)', {inputMode: 'decimal'})}
                  {select(`items.${i}.cofins_cst`, 'COFINS', [NO_TAX, ...NFCOM_PIS_COFINS_CST])}
                  {it?.cofins_cst && text(`items.${i}.cofins_rate`, 'Alíquota da COFINS (%)', {inputMode: 'decimal'})}
                </div>
              </div>
            );
          })}
          {form.formState.errors.items?.message && <p className="text-sm text-danger">{form.formState.errors.items.message}</p>}
        </section>

        <section className={section} aria-labelledby="nfcom-fatura">
          <h2 id="nfcom-fatura" className="text-base font-semibold text-gray-900">Fatura</h2>
          <FormField control={form.control} name="prepaid" render={({field}) => (
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
              Serviço pré-pago (dispensa a fatura)
            </label>
          )}/>
          {!values.prepaid && (
            <div className={grid}>
              {text('invoice.competence', 'Competência (AAAAMM)', {inputMode: 'numeric'})}
              {text('invoice.due_date', 'Vencimento', {type: 'date'})}
              {text('invoice.bar_code', 'Código de barras', {inputMode: 'numeric', className: 'sm:col-span-2'})}
            </div>
          )}
          <FormField control={form.control} name="additional_info" render={({field}) => (
            <FormItem>
              <FormLabel>Informações adicionais</FormLabel>
              <Textarea {...field} id={field.name} rows={3} maxLength={3000}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>

        <EmitError failure={submitError}/>

        <div className="sticky bottom-(--bottomnav-height) -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between md:-mx-8 md:px-8">
          <p className="text-sm text-gray-600">Total estimado R$ {total.toFixed(2).replace('.', ',')}</p>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => router.push('/nfcom')}>Cancelar</Button>
            <Button type="submit" variant="brand" disabled={isSubmitting}>{isSubmitting ? 'Enviando…' : 'Emitir NFCom'}</Button>
          </div>
        </div>
      </form>
    </Form>
  );
}
