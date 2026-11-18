'use client';

import type {Control, UseFormSetValue} from 'react-hook-form';
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Textarea} from '@aoctech/ui';
import {LengthHint} from '@/components/ui/length-hint';
import type {BpeTicketForm} from '@/lib/schemas/bpe';
import {BPE_BOARD_PASS_MAX, BPE_PURCHASE_TYPES, BPE_SUBSTITUTION_TYPES, BPE_TA_SUBSTITUTION_TYPES} from '@/lib/constants/bpe';
import {bpeFields, GRID, H2, SECTION} from './fields';
import {PersonIdField} from './PersonIdField';

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

/** Comprador e agência (pessoas do cadastro), substituição e, no comum, o conteúdo do PDF417: tudo opcional. */
export function BpeExtraSections({control, setValue, values, ta}: {
  control: Control<BpeTicketForm>
  setValue: UseFormSetValue<BpeTicketForm>
  values: BpeTicketForm
  ta: boolean
}) {
  const {text, select} = bpeFields(control);
  return (
    <>
      <section className={SECTION} aria-labelledby="bpe-partes">
        <h2 id="bpe-partes" className={H2}>Comprador e agência</h2>
        <div className={GRID}>
          <PersonIdField control={control} setValue={setValue} name="buyer_person_id" label="Comprador (opcional)" className="sm:col-span-2"
                         hint="Quem pagou o bilhete; o endereço vem do cadastro de pessoas."/>
          <PersonIdField control={control} setValue={setValue} name="agency_person_id" label="Agência (opcional)" className="sm:col-span-2"
                         hint="Pessoa jurídica que vendeu o bilhete, quando não é o emitente."/>
        </div>
      </section>

      <section className={SECTION} aria-labelledby="bpe-substituicao">
        <h2 id="bpe-substituicao" className={H2}>Substituição</h2>
        <FormField control={control} name="substitution.enabled" render={({field}) => (
          <FormItem>
            <label className="flex min-h-11 items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={field.value} onChange={(e) => field.onChange(e.target.checked)}/>
              Este bilhete substitui outro BP-e da empresa
            </label>
            <FormMessage/>
          </FormItem>
        )}/>
        {values.substitution.enabled && (
          <>
            <div className={GRID}>
              {text('substitution.original_key', 'Chave de acesso do BP-e substituído', {className: 'sm:col-span-2', maxLength: 44})}
              {select('substitution.type', 'Tipo da substituição', ta ? BPE_TA_SUBSTITUTION_TYPES : BPE_SUBSTITUTION_TYPES, 'sm:col-span-2')}
            </div>
            <p className="text-xs text-gray-500">O original precisa estar autorizado, repetir origem, destino e comprador e, se o embarque já passou, ter o evento de não embarque.</p>
          </>
        )}
      </section>

      {!ta && (
        <section className={SECTION} aria-labelledby="bpe-pdf417">
          <h2 id="bpe-pdf417" className={H2}>Código de embarque (PDF417)</h2>
          <FormField control={control} name="board_pass" render={({field}) => (
            <FormItem>
              <FormLabel>Conteúdo do código (opcional)</FormLabel>
              <Textarea {...field} id={field.name} rows={2} maxLength={BPE_BOARD_PASS_MAX}/>
              <LengthHint value={field.value} max={BPE_BOARD_PASS_MAX}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </section>
      )}
    </>
  );
}
