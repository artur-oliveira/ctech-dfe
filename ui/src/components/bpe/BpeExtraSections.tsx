'use client';

import type {Control} from 'react-hook-form';
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Textarea} from '@aoctech/ui';
import {UF_OPTIONS} from '@/lib/schemas/entity';
import type {BpeTicketForm} from '@/lib/schemas/bpe';
import {
  BPE_BOARD_PASS_MAX, BPE_PURCHASE_TYPES, BPE_SUBSTITUTION_TYPES, BPE_TA_SUBSTITUTION_TYPES,
} from '@/lib/constants/bpe';
import {bpeFields, GRID, H2, SECTION} from './fields';

function AddressFields({prefix, control}: { prefix: 'agency.address'; control: Control<BpeTicketForm> }) {
  const {text, select} = bpeFields(control);
  return (
    <div className={GRID}>
      {text(`${prefix}.street`, 'Logradouro', {className: 'sm:col-span-2'})}
      {text(`${prefix}.number`, 'Número')}
      {text(`${prefix}.neighborhood`, 'Bairro')}
      {text(`${prefix}.city`, 'Município')}
      {text(`${prefix}.ibge_code`, 'Município (código IBGE)', {inputMode: 'numeric', maxLength: 7})}
      {select(`${prefix}.uf`, 'UF', UF_OPTIONS)}
      {text(`${prefix}.postal_code`, 'CEP', {inputMode: 'numeric', maxLength: 9})}
    </div>
  );
}

function Toggle({control, name, label}: { control: Control<BpeTicketForm>; name: 'agency.enabled' | 'substitution.enabled'; label: string }) {
  return (
    <FormField control={control} name={name} render={({field}) => (
      <FormItem>
        <label className="flex items-center gap-2 text-sm text-gray-700">
          <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
          {label}
        </label>
        <FormMessage/>
      </FormItem>
    )}/>
  );
}

/** Tipo de compra do TA: normal, BP-e múltiplo (fretamento) ou vinculado a um múltiplo. */
export function BpePurchaseSection({control, purchase}: { control: Control<BpeTicketForm>; purchase: BpeTicketForm['purchase_type'] }) {
  const {text, select} = bpeFields(control);
  return (
    <section className={SECTION} aria-labelledby="bpe-compra">
      <h2 id="bpe-compra" className={H2}>Tipo de compra</h2>
      <div className={GRID}>
        {select('purchase_type', 'Tipo de compra', BPE_PURCHASE_TYPES, 'sm:col-span-2')}
        {purchase === 'linked' && text('multiple_key', 'Chave de acesso do BP-e múltiplo', {className: 'sm:col-span-2', maxLength: 44})}
      </div>
      {purchase === 'multiple' && <p className="text-xs text-gray-500">O BP-e múltiplo não identifica passageiro e exige o comprador.</p>}
      {purchase === 'linked' && <p className="text-xs text-gray-500">O vinculado leva só o passageiro e a chave do múltiplo: sem valores, imposto nem pagamento.</p>}
    </section>
  );
}

/** Agência, substituição e (no comum) conteúdo do PDF417, todos opcionais. */
export function BpeExtraSections({control, values, ta}: {
  control: Control<BpeTicketForm>
  values: BpeTicketForm
  ta: boolean
}) {
  const {text, select} = bpeFields(control);
  return (
    <>
      <section className={SECTION} aria-labelledby="bpe-agencia">
        <h2 id="bpe-agencia" className={H2}>Agência</h2>
        <Toggle control={control} name="agency.enabled" label="Bilhete vendido por agência ou ponto de venda de terceiro"/>
        {values.agency.enabled && (
          <>
            <div className={GRID}>
              {text('agency.name', 'Razão social', {className: 'sm:col-span-2'})}
              {text('agency.cnpj', 'CNPJ')}
            </div>
            <AddressFields prefix="agency.address" control={control}/>
          </>
        )}
      </section>

      <section className={SECTION} aria-labelledby="bpe-substituicao">
        <h2 id="bpe-substituicao" className={H2}>Substituição</h2>
        <Toggle control={control} name="substitution.enabled" label="Este bilhete substitui outro BP-e da empresa"/>
        {values.substitution.enabled && (
          <div className={GRID}>
            {text('substitution.original_key', 'Chave de acesso do BP-e substituído', {className: 'sm:col-span-2', maxLength: 44})}
            {select('substitution.type', 'Tipo da substituição', ta ? BPE_TA_SUBSTITUTION_TYPES : BPE_SUBSTITUTION_TYPES, 'sm:col-span-2')}
          </div>
        )}
        {values.substitution.enabled && (
          <p className="text-xs text-gray-500">O original precisa estar autorizado, repetir origem, destino e comprador e, se o embarque já passou, ter o evento de não embarque.</p>
        )}
      </section>

      {!ta && (
        <section className={SECTION} aria-labelledby="bpe-pdf417">
          <h2 id="bpe-pdf417" className={H2}>Código de embarque (PDF417)</h2>
          <FormField control={control} name="board_pass" render={({field}) => (
            <FormItem>
              <FormLabel>Conteúdo do código (opcional)</FormLabel>
              <Textarea {...field} id={field.name} rows={2} maxLength={BPE_BOARD_PASS_MAX}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>
      )}
    </>
  );
}
