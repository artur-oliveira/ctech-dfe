'use client';

import {Label} from '@aoctech/ui';
import {CollapsibleSection} from '@/components/ui/collapsible-section';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {NumericInput} from '@/components/ui/numeric-input';
import {IBS_CBS_CLASS_BY_CST, IBS_CBS_CST} from '@/lib/data/ibs_cbs_cst';
import {CTE_TOTAL_WITH_TAXES_FROM_YEAR} from '@/lib/constants/cte';
import {computeCteIbsCbs, emptyIbsCbs, ibsCbsComplete, type CteIbsCbsState} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';

/** Só o CST que leva o grupo com base e alíquotas (`ind_gIBSCBS`): é o único que o CT-e emite hoje. */
const CST_OPTIONS = IBS_CBS_CST.filter((e) => e.requiresTax).map((e) => ({value: e.cst, label: `${e.cst} · ${e.desc}`}));

interface Props {
  value: CteIbsCbsState | null
  onChange: (v: CteIbsCbsState | null) => void
  freightTotal: string
  /** Ano da emissão; separado só para o teste fixar o `vTotDFe`. */
  year?: number
}

const RATES: { key: 'ibs_uf_rate' | 'ibs_mun_rate' | 'cbs_rate'; label: string }[] = [
  {key: 'ibs_uf_rate', label: 'IBS UF (%)'},
  {key: 'ibs_mun_rate', label: 'IBS município (%)'},
  {key: 'cbs_rate', label: 'CBS (%)'},
];

/**
 * IBS/CBS do CT-e (regime normal, CRT 3): bloco recolhido, ligado por escolha. Nada é presumido: CST,
 * classificação, base e alíquotas são do operador; os valores abaixo são a prévia do que o servidor calcula
 * (vBC x alíquota, 2 casas, arredondamento bancário). O Simples Nacional e o MEI nem veem o bloco.
 */
export function CteIbsCbsFields({value, onChange, freightTotal, year = new Date().getFullYear()}: Props) {
  const calc = value ? computeCteIbsCbs(value, freightTotal, year) : null;
  const patch = (p: Partial<CteIbsCbsState>) => value && onChange({...value, ...p});

  return (
    <CollapsibleSection keepMounted title="IBS e CBS (opcional)" summaryTestId="cte-ibscbs-summary"
                        summary={!value ? 'Não informado' : ibsCbsComplete(value) ? 'Informado' : 'Incompleto'}>
      <>
        <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-800">
          <input id="cte-ibscbs-enable" type="checkbox" className="size-5 accent-brand-600" checked={!!value}
                 onChange={(e) => onChange(e.target.checked ? emptyIbsCbs() : null)}/>
          Informar IBS e CBS neste CT-e
        </label>

        {value && (
          <>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="cte-ibscbs-cst" className="text-sm font-medium text-gray-700">Situação tributária (CST)</Label>
                <Combobox id="cte-ibscbs-cst" value={value.cst} options={CST_OPTIONS} fuzzySearch
                          placeholder="Escolha o CST" searchPlaceholder="Código ou descrição..."
                          onValueChange={(cst) => patch({cst, class_trib: IBS_CBS_CLASS_BY_CST[cst]?.[0]?.value ?? ''})}/>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="cte-ibscbs-class" className="text-sm font-medium text-gray-700">Classificação (cClassTrib)</Label>
                <Combobox id="cte-ibscbs-class" value={value.class_trib} options={IBS_CBS_CLASS_BY_CST[value.cst] ?? []}
                          fuzzySearch placeholder={value.cst ? 'Escolha a classificação' : 'Escolha o CST antes'}
                          searchPlaceholder="Código ou descrição..." disabled={!value.cst}
                          onValueChange={(class_trib) => patch({class_trib})}/>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-4">
              <div className="space-y-1.5">
                <Label htmlFor="cte-ibscbs-base" className="text-sm font-medium text-gray-700">Base de cálculo</Label>
                <CurrencyInput id="cte-ibscbs-base" value={value.base_value} onChange={(base_value) => patch({base_value})}
                               placeholder="0,00" allowZero={false} className="w-full"/>
              </div>
              {RATES.map((r) => (
                <div key={r.key} className="space-y-1.5">
                  <Label htmlFor={`cte-ibscbs-${r.key}`} className="text-sm font-medium text-gray-700">{r.label}</Label>
                  <NumericInput id={`cte-ibscbs-${r.key}`} decimal decimalPlaces={4} integerPlaces={3} value={value[r.key]}
                                onChange={(v) => patch({[r.key]: v})} placeholder="0.0000" className="w-full"/>
                </div>
              ))}
            </div>

            <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-800">
              <input type="checkbox" className="size-5 accent-brand-600" checked={value.donation}
                     onChange={(e) => patch({donation: e.target.checked})}/>
              Doação (indDoacao)
            </label>

            {calc ? (
              <dl data-testid="cte-ibscbs-preview" className="grid grid-cols-2 gap-2 text-sm sm:grid-cols-4">
                <div><dt className="text-gray-500">IBS UF</dt><dd className="font-medium text-gray-900">{formatCurrency(calc.ibsUf)}</dd></div>
                <div><dt className="text-gray-500">IBS município</dt><dd className="font-medium text-gray-900">{formatCurrency(calc.ibsMun)}</dd></div>
                <div><dt className="text-gray-500">IBS total</dt><dd data-testid="cte-ibs-total" className="font-medium text-gray-900">{formatCurrency(calc.ibs)}</dd></div>
                <div><dt className="text-gray-500">CBS</dt><dd data-testid="cte-cbs-total" className="font-medium text-gray-900">{formatCurrency(calc.cbs)}</dd></div>
                <div className="col-span-2 sm:col-span-4">
                  <dt className="text-gray-500">Valor total do documento (vTotDFe)</dt>
                  <dd data-testid="cte-vtotdfe" className="font-medium text-gray-900">{formatCurrency(calc.total)}</dd>
                  {year < CTE_TOTAL_WITH_TAXES_FROM_YEAR && (
                    <p className="mt-0.5 text-xs text-gray-500">Até 2026 o total do documento é só o frete; IBS e CBS entram na soma a partir de 2027.</p>
                  )}
                </div>
              </dl>
            ) : (
              <p className="text-sm text-gray-500">Informe a base de cálculo para ver os valores. Nenhuma alíquota é presumida.</p>
            )}
          </>
        )}
      </>
    </CollapsibleSection>
  );
}
