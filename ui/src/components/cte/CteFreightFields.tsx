'use client';

import {ChevronDown} from 'lucide-react';
import {Button, Label} from '@aoctech/ui';
import {Combobox, type ComboboxOption} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {
  CTE_CST_WITH_REDUCTION,
  CTE_CST_WITHOUT_RATE,
  CTE_FREIGHT_COMPONENT_NAMES,
  CTE_ICMS_CST_OPTIONS,
} from '@/lib/constants/cte';
import {
  componentsTotal,
  computeCteIcms,
  fromCents,
  toCents,
  type CteFreightLine,
  type CteIcmsState,
} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CteIcmsCst} from '@/lib/types/api';
import {CteSection} from './CteBlock';

interface Props {
  total: string
  onTotal: (v: string) => void
  components: CteFreightLine[]
  onComponents: (lines: CteFreightLine[]) => void
  icms: CteIcmsState
  onIcms: (icms: CteIcmsState) => void
  cfop: string
  onCfop: (cfop: string) => void
  /** Já filtrados pelo escopo da prestação (mesma UF ou entre UFs). */
  cfopOptions: ComboboxOption[]
  /** Textos do bloco; o padrão é o da emissão a partir de NF-e. */
  title?: string
  hint?: string
  totalLabel?: string
  /** CSTs oferecidos (o CT-e OS não tem o 60). */
  cstOptions?: { value: CteIcmsCst; label: string }[]
}

const newLineId = () => `comp-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

/** Resumo da disclosure de imposto, visível com ela fechada. */
function icmsSummary(total: string, icms: CteIcmsState): string {
  if (!icms.cst) return 'Escolha o CST';
  if (icms.cst === 'SN') return 'Simples Nacional';
  if (CTE_CST_WITHOUT_RATE.has(icms.cst)) return `CST ${icms.cst} · sem ICMS destacado`;
  const calc = computeCteIcms(total, icms);
  return calc ? `ICMS ${formatCurrency(calc.value)}` : `CST ${icms.cst} · informe a alíquota`;
}

/** Bloco 3: quanto custa. Total, componentes opcionais e o ICMS calculado ao vivo. */
export function CteFreightFields({
  total, onTotal, components, onComponents, icms, onIcms, cfop, onCfop, cfopOptions,
  title = 'Quanto custa o frete?', hint = 'O total da prestação. Componentes são opcionais e saem do total.',
  totalLabel = 'Valor total do frete', cstOptions = CTE_ICMS_CST_OPTIONS,
}: Props) {
  const remainder = toCents(total) - componentsTotal(components);
  const usedNames = new Set(components.map((c) => c.name));
  const calc = computeCteIcms(total, icms);
  const needsRate = !!icms.cst && !CTE_CST_WITHOUT_RATE.has(icms.cst);

  const update = (id: string, patch: Partial<CteFreightLine>) =>
    onComponents(components.map((c) => (c.id === id ? {...c, ...patch} : c)));

  return (
    <CteSection id="cte-freight" title={title} hint={hint}>
      <div className="space-y-1.5">
        <Label htmlFor="cte-freight-total" className="text-sm font-medium text-gray-700">{totalLabel}</Label>
        <CurrencyInput id="cte-freight-total" value={total} onChange={onTotal} placeholder="0,00"
                       allowZero={false} className="w-full sm:max-w-xs"/>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="cte-cfop" className="text-sm font-medium text-gray-700">CFOP da prestação</Label>
        <Combobox id="cte-cfop" value={cfop} onValueChange={onCfop} options={cfopOptions} fuzzySearch
                  placeholder="Escolha o CFOP" searchPlaceholder="Código ou descrição..."/>
      </div>

      {components.length > 0 && (
        <ul aria-label="Componentes do frete" className="space-y-2">
          {components.map((c, i) => (
            <li key={c.id} className="grid grid-cols-[1fr_auto] gap-2 sm:grid-cols-[1fr_12rem_auto] sm:items-center">
              <div className="col-span-2 sm:col-span-1">
                <OptionsSelect
                  id={`cte-comp-name-${i}`}
                  ariaLabel={`Componente ${i + 1}`}
                  value={c.name}
                  onValueChange={(name) => update(c.id, {name})}
                  placeholder="Tipo de componente"
                  options={CTE_FREIGHT_COMPONENT_NAMES
                    .filter((n) => n === c.name || !usedNames.has(n))
                    .map((n) => ({value: n, label: n}))}
                />
              </div>
              <CurrencyInput aria-label={`Valor do componente ${i + 1}`} value={c.value}
                             onChange={(value) => update(c.id, {value})} placeholder="0,00"
                             allowZero={false} className="w-full"/>
              <Button type="button" variant="ghost" className="text-danger hover:text-red-700"
                      onClick={() => onComponents(components.filter((x) => x.id !== c.id))}
                      aria-label={`Remover componente ${i + 1}`}>
                Remover
              </Button>
            </li>
          ))}
        </ul>
      )}

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <Button type="button" variant="outline"
                disabled={components.length >= CTE_FREIGHT_COMPONENT_NAMES.length}
                onClick={() => onComponents([...components, {id: newLineId(), name: '', value: ''}])}>
          Adicionar componente
        </Button>
        {components.length > 0 && toCents(total) > 0 && (
          <p className={`text-sm ${remainder > 0 ? 'text-gray-600' : 'text-danger'}`}>
            Frete valor: {remainder > 0 ? formatCurrency(fromCents(remainder)) : 'componentes passam do total'}
          </p>
        )}
      </div>

      <details className="group rounded-lg border border-gray-200">
        <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 px-3 py-2 [&::-webkit-details-marker]:hidden">
          <span className="text-sm font-medium text-gray-900">Imposto</span>
          <span className="flex min-w-0 items-center gap-2">
            <span data-testid="cte-icms-summary" className="truncate text-sm text-gray-600">{icmsSummary(total, icms)}</span>
            <ChevronDown className="size-4 shrink-0 text-gray-400 transition-transform group-open:rotate-180" aria-hidden="true"/>
          </span>
        </summary>
        <div className="space-y-3 border-t border-gray-100 p-3">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div className="space-y-1.5 sm:col-span-3">
              <Label htmlFor="cte-icms-cst" className="text-sm font-medium text-gray-700">Situação tributária (CST)</Label>
              <OptionsSelect id="cte-icms-cst" value={icms.cst} options={cstOptions}
                             onValueChange={(cst) => onIcms({...icms, cst: cst as CteIcmsCst | ''})}
                             placeholder="Escolha o CST" className="w-full"/>
            </div>
            {needsRate && (
              <div className="space-y-1.5">
                <Label htmlFor="cte-icms-rate" className="text-sm font-medium text-gray-700">Alíquota (%)</Label>
                <NumericInput id="cte-icms-rate" decimal decimalPlaces={2} integerPlaces={3} value={icms.p_icms}
                              onChange={(p_icms) => onIcms({...icms, p_icms})} placeholder="0,00" className="w-full"/>
              </div>
            )}
            {needsRate && icms.cst && CTE_CST_WITH_REDUCTION.has(icms.cst) && (
              <div className="space-y-1.5">
                <Label htmlFor="cte-icms-red" className="text-sm font-medium text-gray-700">Redução da base (%)</Label>
                <NumericInput id="cte-icms-red" decimal decimalPlaces={2} integerPlaces={3} value={icms.p_red_bc}
                              onChange={(p_red_bc) => onIcms({...icms, p_red_bc})} placeholder="0,00" className="w-full"/>
              </div>
            )}
          </div>
          {calc && (
            <dl className="grid grid-cols-2 gap-2 text-sm">
              <div><dt className="text-gray-500">Base de cálculo</dt><dd className="font-medium text-gray-900">{formatCurrency(calc.base)}</dd></div>
              <div><dt className="text-gray-500">ICMS</dt><dd data-testid="cte-icms-value" className="font-medium text-gray-900">{formatCurrency(calc.value)}</dd></div>
            </dl>
          )}
          {needsRate && !icms.p_icms && (
            <p className="text-sm text-warning">Informe a alíquota; nenhuma é presumida.</p>
          )}
        </div>
      </details>
    </CteSection>
  );
}
