'use client';

import {useState, type ReactNode} from 'react';
import {ChevronDown} from 'lucide-react';
import {Button} from '@aoctech/ui';
import {PersonPicker} from '@/components/persons/PersonPicker';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {Input} from '@/components/ui/input';
import {NumericInput} from '@/components/ui/numeric-input';
import {CITY_OPTIONS} from '@/lib/data/cities';
import {CTE_PRODUCT_MAX_LENGTH} from '@/lib/constants/cte';
import {cn} from '@/lib/utils';
import {cityPlace, formatWeightKg, placeLabel, type CteOverrideField, type CteOverrides} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CteDeriveOut, CteDeriveWarning, CteParty} from '@/lib/types/api';
import {CteWarnings} from './CteBlock';

interface Props {
  derive: CteDeriveOut | undefined
  overrides: CteOverrides
  onChange: (next: CteOverrides) => void
  warnings: CteDeriveWarning[]
}

const partyLabel = (p: CteParty | null | undefined) => (p ? `${p.name} · ${p.city} / ${p.uf}` : null);

/**
 * Bloco 4: tudo que sai das notas, recolhido. Cada campo mostra o derivado;
 * editar grava um override (o campo ganha "editado") e "Voltar ao derivado"
 * apaga o override. Abre sozinho quando há aviso deste bloco.
 */
export function CteTransportData({derive, overrides, onChange, warnings}: Props) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const [editing, setEditing] = useState<CteOverrideField | null>(null);
  const open = userOpen ?? warnings.length > 0;

  const set = <K extends CteOverrideField>(field: K, value: CteOverrides[K]) => onChange({...overrides, [field]: value});
  const reset = (field: CteOverrideField) => {
    const next = {...overrides};
    delete next[field];
    onChange(next);
    setEditing(null);
  };
  const editedCount = Object.keys(overrides).length;

  const cargo = derive?.cargo;
  // Sem derivado: as notas divergem (remetente, destinatário e trajeto) ou não trazem o dado (peso).
  const diverge = derive ? 'As notas divergem; escolha aqui' : 'Escolha as notas';
  const absent = derive ? 'Não consta nas notas' : 'Escolha as notas';
  const rows: { field: CteOverrideField; label: string; derived: string | null; current: string | null; empty: string; editor: ReactNode }[] = [
    {
      field: 'sender', empty: diverge, label: 'Remetente',
      derived: partyLabel(derive?.sender), current: overrides.sender?.name ?? null,
      editor: <PersonPicker value={overrides.sender ?? null} placeholder="Buscar remetente cadastrado"
                            onChange={(p) => (p ? set('sender', p) : reset('sender'))}/>,
    },
    {
      field: 'recipient', empty: diverge, label: 'Destinatário',
      derived: partyLabel(derive?.recipient), current: overrides.recipient?.name ?? null,
      editor: <PersonPicker value={overrides.recipient ?? null} placeholder="Buscar destinatário cadastrado"
                            onChange={(p) => (p ? set('recipient', p) : reset('recipient'))}/>,
    },
    {
      field: 'origin', empty: diverge, label: 'Origem',
      derived: placeLabel(derive?.origin ?? null), current: overrides.origin ? placeLabel(cityPlace(overrides.origin)) : null,
      editor: <Combobox id="cte-origin" fuzzySearch options={CITY_OPTIONS}
                        value={overrides.origin ?? derive?.origin?.city_ibge ?? null}
                        onValueChange={(v) => (v ? set('origin', v) : reset('origin'))}
                        placeholder="Município de origem" searchPlaceholder="Buscar município..."/>,
    },
    {
      field: 'destination', empty: diverge, label: 'Destino',
      derived: placeLabel(derive?.destination ?? null),
      current: overrides.destination ? placeLabel(cityPlace(overrides.destination)) : null,
      editor: <Combobox id="cte-destination" fuzzySearch options={CITY_OPTIONS}
                        value={overrides.destination ?? derive?.destination?.city_ibge ?? null}
                        onValueChange={(v) => (v ? set('destination', v) : reset('destination'))}
                        placeholder="Município de destino" searchPlaceholder="Buscar município..."/>,
    },
    {
      field: 'total_weight_kg', empty: absent, label: 'Peso da carga',
      derived: cargo?.total_weight_kg ? formatWeightKg(cargo.total_weight_kg) : null,
      current: overrides.total_weight_kg ? formatWeightKg(overrides.total_weight_kg) : null,
      editor: <NumericInput id="cte-weight" aria-label="Peso da carga em kg" decimal decimalPlaces={3} integerPlaces={12}
                            suffix="kg" className="w-full sm:max-w-xs"
                            value={overrides.total_weight_kg ?? cargo?.total_weight_kg ?? ''}
                            onChange={(v) => set('total_weight_kg', v)}/>,
    },
    {
      field: 'total_value', empty: absent, label: 'Valor da carga',
      derived: cargo ? formatCurrency(cargo.total_value) : null,
      current: overrides.total_value ? formatCurrency(overrides.total_value) : null,
      editor: <CurrencyInput id="cte-cargo-value" aria-label="Valor da carga" className="w-full sm:max-w-xs"
                             value={overrides.total_value ?? cargo?.total_value ?? ''}
                             onChange={(v) => set('total_value', v)}/>,
    },
    {
      field: 'predominant_product', empty: absent, label: 'Produto predominante',
      derived: cargo?.predominant_product || null, current: overrides.predominant_product ?? null,
      editor: <Input id="cte-product" aria-label="Produto predominante" maxLength={CTE_PRODUCT_MAX_LENGTH}
                     className="w-full" value={overrides.predominant_product ?? cargo?.predominant_product ?? ''}
                     onChange={(e) => set('predominant_product', e.target.value)}/>,
    },
  ];

  return (
    <section aria-labelledby="cte-transport" className="rounded-xl border border-gray-200 bg-white">
      <h2 id="cte-transport" className="text-base font-semibold text-gray-900">
      <button type="button" aria-expanded={open} aria-controls="cte-transport-panel"
              onClick={() => setUserOpen(!open)}
              className="flex min-h-11 w-full items-center justify-between gap-3 px-4 py-3 text-left md:px-5 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring">
        <span className="min-w-0">
          <span className="block">Dados do transporte</span>
          <span className="block text-sm font-normal text-gray-500">
            Derivado das notas{editedCount > 0 ? ` · ${editedCount} ${editedCount === 1 ? 'campo editado' : 'campos editados'}` : ''}
          </span>
        </span>
        <ChevronDown className={cn('size-4 shrink-0 text-gray-400 transition-transform', open && 'rotate-180')} aria-hidden="true"/>
      </button>
      </h2>

      {open && (
        <div id="cte-transport-panel" className="space-y-3 border-t border-gray-100 px-4 py-4 md:px-5">
          <CteWarnings warnings={warnings}/>
          <div className="divide-y divide-gray-100">
            {rows.map(({field, label, derived, current, empty, editor}) => {
              const edited = field in overrides;
              const isEditing = editing === field;
              return (
                <div key={field} data-field={field} className="space-y-2 py-3 first:pt-0 last:pb-0">
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <p className="flex items-center gap-2 text-sm text-gray-500">
                        {label}
                        {edited && <span className="rounded-full bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">editado</span>}
                      </p>
                      <p className={cn('text-sm font-medium', (current ?? derived) ? 'text-gray-900' : 'text-warning')}>
                        {current ?? derived ?? empty}
                      </p>
                      {edited && derived && <p className="text-xs text-gray-500">Derivado: {derived}</p>}
                    </div>
                    <div className="flex shrink-0 flex-col-reverse items-end gap-1 sm:flex-row sm:items-center sm:gap-2">
                      {edited && (
                        <Button type="button" variant="ghost" onClick={() => reset(field)}>Voltar ao derivado</Button>
                      )}
                      <Button type="button" variant="outline" onClick={() => setEditing(isEditing ? null : field)}
                              aria-expanded={isEditing} aria-label={`${isEditing ? 'Concluir' : 'Editar'} ${label.toLowerCase()}`}>
                        {isEditing ? 'Pronto' : 'Editar'}
                      </Button>
                    </div>
                  </div>
                  {isEditing && <div>{editor}</div>}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </section>
  );
}
