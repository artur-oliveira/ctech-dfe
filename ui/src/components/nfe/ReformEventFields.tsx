'use client';

import {Button} from '@aoctech/ui';
import {CurrencyInput} from '@/components/ui/currency-input';
import {Input} from '@/components/ui/input';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {PersonDocField} from '@/components/dfe/PersonDocField';
import {UNIT_OPTIONS} from '@/lib/data/unit';
import {
  emptyReformItem, PAYMENT_METHOD_OPTIONS, REFORM_AUTHOR_OPTIONS, REFORM_ITEM_MAX, REFORM_MAX_ITEMS,
  type PresumedDraft, type ReformDraft, type ReformEventDef, type ReformItemDraft,
} from '@/lib/data/nfe_reform_events';

const LABEL = 'text-xs font-medium text-gray-600';

function Field({label, htmlFor, children}: { label: string; htmlFor: string; children: React.ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label htmlFor={htmlFor} className={LABEL}>{label}</label>
      {children}
    </div>
  );
}

function Money({id, label, value, onChange}: { id: string; label: string; value: string; onChange: (v: string) => void }) {
  return (
    <Field label={label} htmlFor={id}>
      <CurrencyInput id={id} value={value} onChange={onChange} decimalPlaces={2}/>
    </Field>
  );
}

function PresumedGroup({idp, title, value, onChange}: { idp: string; title: string; value: PresumedDraft; onChange: (v: PresumedDraft) => void }) {
  return (
    <fieldset className="space-y-2 rounded-lg border border-gray-200 p-3">
      <legend className="px-1 text-xs font-semibold text-gray-600">{title} (opcional)</legend>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <Field label="Percentual" htmlFor={`${idp}-pct`}>
          <NumericInput id={`${idp}-pct`} decimal integerPlaces={3} decimalPlaces={4} suffix="%" value={value.percent}
                        onChange={(percent) => onChange({...value, percent})}/>
        </Field>
        <Money id={`${idp}-val`} label="Valor" value={value.value} onChange={(v) => onChange({...value, value: v})}/>
      </div>
    </fieldset>
  );
}

function ItemRow({def, it, n, onChange, onRemove, removable}: {
  def: ReformEventDef; it: ReformItemDraft; n: number
  onChange: (patch: Partial<ReformItemDraft>) => void; onRemove: () => void; removable: boolean
}) {
  const f = def.itemFields ?? [];
  const id = `reform-item-${it.id}`;
  return (
    <li className="space-y-3 rounded-lg border border-gray-200 p-3" aria-label={`Item ${n}`}>
      <Field label="Número do item na nota" htmlFor={`${id}-n`}>
        <NumericInput id={`${id}-n`} integerPlaces={3} placeholder={`1 a ${REFORM_ITEM_MAX}`} value={it.item_number}
                      onChange={(v) => onChange({item_number: v})}/>
      </Field>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {f.includes('ibs') && <Money id={`${id}-ibs`} label="IBS" value={it.ibs} onChange={(ibs) => onChange({ibs})}/>}
        {f.includes('cbs') && <Money id={`${id}-cbs`} label="CBS" value={it.cbs} onChange={(cbs) => onChange({cbs})}/>}
        {f.includes('stock') && (
          <>
            <Field label="Quantidade" htmlFor={`${id}-qty`}>
              <NumericInput id={`${id}-qty`} decimal integerPlaces={11} decimalPlaces={4} value={it.quantity}
                            onChange={(quantity) => onChange({quantity})}/>
            </Field>
            <Field label="Unidade" htmlFor={`${id}-unit`}>
              <OptionsSelect id={`${id}-unit`} value={it.unit} options={UNIT_OPTIONS} placeholder="Escolha a unidade"
                             onValueChange={(unit) => onChange({unit})}/>
            </Field>
          </>
        )}
        {f.includes('credit') && (
          <>
            <Money id={`${id}-cibs`} label="Crédito de IBS" value={it.credit_ibs} onChange={(credit_ibs) => onChange({credit_ibs})}/>
            <Money id={`${id}-ccbs`} label="Crédito de CBS" value={it.credit_cbs} onChange={(credit_cbs) => onChange({credit_cbs})}/>
          </>
        )}
        {f.includes('presumed') && (
          <>
            <Money id={`${id}-base`} label="Base do crédito presumido" value={it.base_value} onChange={(base_value) => onChange({base_value})}/>
            <Field label="Código do crédito (2 dígitos)" htmlFor={`${id}-pcode`}>
              <NumericInput id={`${id}-pcode`} integerPlaces={2} value={it.presumed_credit_code}
                            onChange={(presumed_credit_code) => onChange({presumed_credit_code})}/>
            </Field>
          </>
        )}
      </div>
      {f.includes('presumed') && (
        <>
          <PresumedGroup idp={`${id}-ibsp`} title="Crédito presumido de IBS" value={it.ibs_presumed} onChange={(ibs_presumed) => onChange({ibs_presumed})}/>
          <PresumedGroup idp={`${id}-cbsp`} title="Crédito presumido de CBS" value={it.cbs_presumed} onChange={(cbs_presumed) => onChange({cbs_presumed})}/>
        </>
      )}
      {removable && (
        <Button type="button" variant="ghost" size="sm" className="min-h-11 text-danger sm:min-h-0"
                aria-label={`Remover item ${n}`} onClick={onRemove}>Remover</Button>
      )}
    </li>
  );
}

/** Campos de um evento da reforma, montados a partir da definição (nada fora do que o evento pede). */
export function ReformEventFields({def, draft, onChange, nextId, onNextId}: {
  def: ReformEventDef; draft: ReformDraft; onChange: (patch: Partial<ReformDraft>) => void
  nextId: number; onNextId: (n: number) => void
}) {
  const setItem = (id: number, patch: Partial<ReformItemDraft>) =>
    onChange({items: draft.items.map((i) => (i.id === id ? {...i, ...patch} : i))});
  const p = draft.payment;

  return (
    <div className="space-y-4">
      {def.chooseAuthor && (
        <Field label="Quem emite o evento" htmlFor="reform-author">
          <OptionsSelect id="reform-author" value={draft.author} options={REFORM_AUTHOR_OPTIONS} placeholder="Escolha o autor"
                         onValueChange={(author) => onChange({author})}/>
        </Field>
      )}

      {def.shape === 'indicator' && (def.indicatorOnlyOn ? (
        <label className="flex min-h-11 items-center gap-2 text-sm text-gray-800">
          <input type="checkbox" className="size-5" checked={draft.indicator === '1'}
                 onChange={(e) => onChange({indicator: e.target.checked ? '1' : ''})}/>
          {def.indicatorLabels?.on}
        </label>
      ) : (
        <Field label="Resposta" htmlFor="reform-indicator">
          <OptionsSelect id="reform-indicator" value={draft.indicator} placeholder="Escolha a resposta"
                         options={[{value: '1', label: def.indicatorLabels?.on ?? 'Sim'}, {value: '0', label: def.indicatorLabels?.off ?? 'Não'}]}
                         onValueChange={(indicator) => onChange({indicator})}/>
        </Field>
      ))}

      {def.shape === 'forecast' && (
        <Field label="Data prevista de entrega" htmlFor="reform-forecast">
          <Input id="reform-forecast" type="date" className="h-11 w-full" value={draft.forecast}
                 onChange={(e) => onChange({forecast: e.target.value})}/>
        </Field>
      )}

      {def.shape === 'items' && (
        <>
          <ul className="space-y-3" aria-label="Itens do evento">
            {draft.items.map((it, i) => (
              <ItemRow key={it.id} def={def} it={it} n={i + 1} removable={draft.items.length > 1}
                       onChange={(patch) => setItem(it.id, patch)}
                       onRemove={() => onChange({items: draft.items.filter((x) => x.id !== it.id)})}/>
            ))}
          </ul>
          <Button type="button" variant="outline" className="min-h-11 sm:min-h-0" disabled={draft.items.length >= REFORM_MAX_ITEMS}
                  onClick={() => { onChange({items: [...draft.items, emptyReformItem(nextId)]}); onNextId(nextId + 1); }}>
            Adicionar item
          </Button>
        </>
      )}

      {def.shape === 'payment' && (
        <div className="space-y-3">
          <Field label="Identificador da transação (2 a 35 caracteres)" htmlFor="reform-tx">
            <Input id="reform-tx" className="h-11 w-full" maxLength={35} value={p.transaction_id}
                   onChange={(e) => onChange({payment: {...p, transaction_id: e.target.value}})}/>
          </Field>
          <Field label="Meio de pagamento" htmlFor="reform-method">
            <OptionsSelect id="reform-method" value={p.payment_method} options={PAYMENT_METHOD_OPTIONS} placeholder="Escolha o meio"
                           onValueChange={(payment_method) => onChange({payment: {...p, payment_method}})}/>
          </Field>
          <PersonDocField id="reform-receiver" label="Recebedor do pagamento" accept="cnpj" value={p.receiver}
                          onChange={(receiver) => onChange({payment: {...p, receiver}})}/>
          <PersonDocField id="reform-psp" label="PSP (usa a raiz de 8 dígitos do CNPJ)" accept="cnpj" value={p.psp}
                          onChange={(psp) => onChange({payment: {...p, psp}})}/>
        </div>
      )}
    </div>
  );
}
