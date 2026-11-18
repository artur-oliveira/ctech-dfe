'use client';

import type {Control, FieldValues} from 'react-hook-form';
import {FormField} from '@/components/ui/form';
import {BPE_CBS_RATE_FROM_YEAR, BPE_GOV_ENTITIES, BPE_GOV_OPERATIONS, BPE_IBS_RATE_FROM_YEAR, BPE_TA_CLASSES, BPE_TA_NO_TAX_CST} from '@/lib/constants/bpe';
import type {TaTaxForm} from '@/lib/schemas/bpe-tax';
import {bpeFields, GRID, SUBCARD} from './fields';

/** IBS e CBS do TA, guardados na linha: classificação (lista da tabela oficial), alíquotas do ano, estorno, compra governamental e Simples. */
export function BpeTaTaxFields<T extends FieldValues>({control, tax, prefix = 'tax'}: { control: Control<T>; tax: TaTaxForm; prefix?: string }) {
  const {text, select} = bpeFields(control);
  const year = new Date().getFullYear();
  const cls = BPE_TA_CLASSES.find((c) => c.value === tax.class_trib);
  const taxed = cls !== undefined && cls.cst !== BPE_TA_NO_TAX_CST;
  const check = (name: string, label: string) => (
    <FormField control={control} name={name as never} render={({field}) => (
      <label className="flex min-h-11 items-center gap-2 text-sm text-gray-700">
        <input type="checkbox" checked={field.value as boolean} onChange={(e) => field.onChange(e.target.checked)}/>
        {label}
      </label>
    )}/>
  );
  const p = (k: string) => `${prefix}.${k}`;
  return (
    <div className="space-y-4">
      <div className={GRID}>
        {select(p('class_trib'), 'Classificação tributária', BPE_TA_CLASSES, 'sm:col-span-2 lg:col-span-3')}
        {taxed && year >= BPE_IBS_RATE_FROM_YEAR && text(p('ibs_uf_rate'), 'Alíquota do IBS da UF (%)', {inputMode: 'decimal'})}
        {taxed && year >= BPE_IBS_RATE_FROM_YEAR && text(p('ibs_mun_rate'), 'Alíquota do IBS do município (%)', {inputMode: 'decimal'})}
        {taxed && year >= BPE_CBS_RATE_FROM_YEAR && text(p('cbs_rate'), 'Alíquota da CBS (%)', {inputMode: 'decimal'})}
        {text(p('total_taxes'), 'Total aproximado de tributos (R$)', {inputMode: 'decimal'})}
      </div>
      {cls && cls.donation !== 0 && check(p('donation'), `Operação de doação${cls.donation === 1 ? ' (obrigatório nesta classificação)' : ''}`)}
      {cls?.reversal && (
        <div className={GRID}>
          {text(p('reversal.ibs'), 'IBS estornado (R$)', {inputMode: 'decimal'})}
          {text(p('reversal.cbs'), 'CBS estornada (R$)', {inputMode: 'decimal'})}
        </div>
      )}
      {taxed && (
        <>
          {check(p('government.enabled'), 'Compra governamental')}
          {tax.government.enabled && (
            <div className={SUBCARD}>
              <div className={GRID}>
                {select(p('government.entity'), 'Ente comprador', BPE_GOV_ENTITIES)}
                {select(p('government.operation'), 'Tipo da operação', BPE_GOV_OPERATIONS, 'sm:col-span-2')}
                {text(p('government.reducer'), 'Redutor da compra governamental (%)', {inputMode: 'decimal'})}
              </div>
              {['2', '3'].includes(tax.government.operation) && text(p('government.references'), 'Chave de acesso do DFe anterior (uma por linha ou separadas por espaço)')}
            </div>
          )}
          <div className={GRID}>
            {text(p('devolution.ibs_uf'), 'Devolução de IBS da UF (R$)', {inputMode: 'decimal'})}
            {text(p('devolution.ibs_mun'), 'Devolução de IBS do município (R$)', {inputMode: 'decimal'})}
            {text(p('devolution.cbs'), 'Devolução de CBS (R$)', {inputMode: 'decimal'})}
          </div>
        </>
      )}
      {check(p('simples.enabled'), 'Emitente optante do Simples Nacional (grupo de composição do IBS e da CBS)')}
      {tax.simples.enabled && (
        <div className={GRID}>
          {text(p('simples.ibs_rate'), 'Alíquota efetiva do IBS (%)', {inputMode: 'decimal'})}
          {text(p('simples.cbs_rate'), 'Alíquota efetiva da CBS (%)', {inputMode: 'decimal'})}
        </div>
      )}
      {text(p('fiscal_info'), 'Informações adicionais de interesse do Fisco')}
      <p className="text-xs text-gray-500">
        A classificação vem da tabela oficial e define o CST. As alíquotas de IBS e CBS fixadas em lei (2026 a 2028 para o IBS e 2026 para a CBS)
        são aplicadas pelo servidor, sobre o total pago, junto com a redução da classe, o redutor e o valor total do documento.
      </p>
    </div>
  );
}
