'use client';

import {useQuery} from '@tanstack/react-query';
import {Button, Label} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {Input} from '@/components/ui/input';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {
  CTE_ACCESS_KEY_LENGTH,
  CTE_OS_CHARTER_EVENTUAL,
  CTE_OS_CHARTER_TYPES,
  CTE_OS_DOCS_MAX,
  CTE_OS_GTVE_COMPONENT_NAME_MAX,
  CTE_OS_GTVE_COMPONENT_OTHERS,
  CTE_OS_GTVE_COMPONENT_TYPES,
  CTE_OS_STATE_REGISTRATION_LENGTH,
  CTE_OS_TAF_LENGTH,
} from '@/lib/constants/cte';
import {cn} from '@/lib/utils';
import {
  emptyDocLine,
  emptyGtve,
  emptyGtveComponent,
  gtveComponentsCents,
  gtveTotalCents,
  gtveTotalCloses,
  osHasCharter,
  type CteOsDocLine,
  type CteOsGtveLine,
  type CteOsRegistrationKind,
  type CteOsState,
} from '@/lib/utils/cte-os';
import {fromCents} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';
import type {VehicleOut} from '@/lib/types/api';
import {CteSection} from './CteBlock';

const FIELD = 'space-y-1.5';
const LABEL = 'text-sm font-medium text-gray-700';

interface RoadProps {
  state: CteOsState
  onChange: (patch: Partial<CteOsState>) => void
  /** Imposto pelo trajeto: `null` deixa a escolha com o usuário. */
  forcedKind: CteOsRegistrationKind | null
}

/** Modal rodoviário do CT-e OS: TAF ou registro estadual, veículo do cadastro e, em pessoas, o fretamento. */
export function CteOsRoad({state, onChange, forcedKind}: RoadProps) {
  const {selectedOrg} = useAuth();
  const {data} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk),
    queryFn: () => apiClient.getVehicles({limit: 100}),
    enabled: !!selectedOrg,
  });
  const vehicles: VehicleOut[] = data?.items ?? [];
  const kind = forcedKind ?? state.registrationKind;
  const eventual = state.charterType === CTE_OS_CHARTER_EVENTUAL;

  return (
    <CteSection id="cte-os-road" title="Transporte rodoviário"
                hint={forcedKind
                  ? (forcedKind === 'taf' ? 'Entre UFs, o transporte de pessoas exige o TAF.' : 'Dentro da mesma UF, o transporte de pessoas exige o registro estadual.')
                  : 'Informe o TAF ou o registro estadual.'}>
      <div className="grid grid-cols-1 gap-x-3 gap-y-4 sm:grid-cols-2">
      {!forcedKind && (
        <div className={FIELD}>
          <p className={LABEL}>Qual documento vale</p>
          <OptionsSelect id="cte-os-reg-kind" value={state.registrationKind} placeholder="Escolha"
                         options={[{value: 'taf', label: 'TAF (Termo de Autorização de Fretamento)'}, {value: 'state_registration', label: 'Registro estadual'}]}
                         onValueChange={(v) => onChange({registrationKind: v as CteOsRegistrationKind})} className="w-full sm:max-w-md"/>
        </div>
      )}
      {kind === 'taf' && (
        <div className={FIELD}>
          <Label htmlFor="cte-os-taf" className={LABEL}>TAF ({CTE_OS_TAF_LENGTH} dígitos)</Label>
          <NumericInput id="cte-os-taf" inputMode="numeric" integerPlaces={CTE_OS_TAF_LENGTH} value={state.taf}
                        onChange={(taf) => onChange({taf})} className="w-full sm:max-w-xs"/>
        </div>
      )}
      {kind === 'state_registration' && (
        <div className={FIELD}>
          <Label htmlFor="cte-os-reg" className={LABEL}>Registro estadual ({CTE_OS_STATE_REGISTRATION_LENGTH} dígitos)</Label>
          <NumericInput id="cte-os-reg" inputMode="numeric" integerPlaces={CTE_OS_STATE_REGISTRATION_LENGTH}
                        value={state.stateRegistration} onChange={(stateRegistration) => onChange({stateRegistration})}
                        className="w-full sm:max-w-md"/>
        </div>
      )}

      <div className={FIELD}>
        <Label htmlFor="cte-os-vehicle" className={LABEL}>Veículo (opcional)</Label>
        <Combobox id="cte-os-vehicle" fuzzySearch value={state.vehicle?.sk ?? null} clearable
                  options={vehicles.map((v) => ({value: v.sk, label: `${v.plate} · ${v.plate_uf}`}))}
                  placeholder="Escolha um veículo cadastrado" searchPlaceholder="Placa..."
                  onValueChange={(sk) => onChange({vehicle: vehicles.find((v) => v.sk === sk) ?? null})}/>
      </div>

      {osHasCharter(state.serviceType) && (
        <>
          <div className={FIELD}>
            <Label htmlFor="cte-os-charter" className={LABEL}>Tipo de fretamento</Label>
            <OptionsSelect id="cte-os-charter" value={state.charterType} placeholder="Escolha"
                           options={CTE_OS_CHARTER_TYPES.map((c) => ({value: c.value, label: c.label}))}
                           onValueChange={(v) => onChange({charterType: v as CteOsState['charterType'], tripAt: v === CTE_OS_CHARTER_EVENTUAL ? state.tripAt : ''})}
                           className="w-full sm:max-w-xs"/>
          </div>
          {eventual && (
            <div className={FIELD}>
              <Label htmlFor="cte-os-trip" className={LABEL}>Data e hora da viagem</Label>
              <Input id="cte-os-trip" type="datetime-local" value={state.tripAt} className="w-full sm:max-w-xs"
                     onChange={(e) => onChange({tripAt: e.target.value})}/>
              <p className="text-xs text-gray-500">Precisa ser depois do momento da emissão.</p>
            </div>
          )}
        </>
      )}
      </div>
    </CteSection>
  );
}

const patchLine = <T extends { id: string }>(list: T[], id: string, p: Partial<T>) => list.map((x) => (x.id === id ? {...x, ...p} : x));

/** Documentos referenciados do excesso de bagagem: chave do BP-e, ou número, série e data. */
export function CteOsDocRefs({lines, onChange}: { lines: CteOsDocLine[]; onChange: (l: CteOsDocLine[]) => void }) {
  const set = (id: string, p: Partial<CteOsDocLine>) => onChange(patchLine(lines, id, p));
  return (
    <CteSection id="cte-os-docs" title="Qual bilhete tem excesso de bagagem?"
                hint="Um documento por linha: a chave do BP-e, ou o número e a data do outro documento.">
      <ul aria-label="Documentos referenciados" className="space-y-3">
        {lines.map((d, i) => (
          <li key={d.id} className="space-y-2 rounded-lg border border-gray-200 p-3">
            <div className="flex items-center justify-between gap-2">
              <OptionsSelect ariaLabel={`Tipo do documento ${i + 1}`} value={d.mode} className="w-full sm:max-w-xs"
                             options={[{value: 'bpe', label: 'BP-e (chave)'}, {value: 'other', label: 'Outro documento'}]}
                             onValueChange={(mode) => set(d.id, {mode: mode as CteOsDocLine['mode']})}/>
              <Button type="button" variant="ghost" className="text-danger hover:text-red-700"
                      aria-label={`Remover documento ${i + 1}`} onClick={() => onChange(lines.filter((x) => x.id !== d.id))}>
                Remover
              </Button>
            </div>
            {d.mode === 'bpe' ? (
              <NumericInput aria-label={`Chave do BP-e ${i + 1}`} inputMode="numeric" integerPlaces={CTE_ACCESS_KEY_LENGTH}
                            placeholder={`Chave de ${CTE_ACCESS_KEY_LENGTH} dígitos`} value={d.bpe_key}
                            onChange={(bpe_key) => set(d.id, {bpe_key})} className="w-full font-mono"/>
            ) : (
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
                <NumericInput aria-label={`Número ${i + 1}`} placeholder="Número" integerPlaces={20} value={d.number}
                              onChange={(number) => set(d.id, {number})} className="w-full"/>
                <Input aria-label={`Série ${i + 1}`} placeholder="Série" maxLength={3} value={d.serie}
                       onChange={(e) => set(d.id, {serie: e.target.value.replace(/[^A-Za-z0-9]/g, '')})} className="w-full"/>
                <Input aria-label={`Subsérie ${i + 1}`} placeholder="Subsérie" maxLength={3} value={d.subserie}
                       onChange={(e) => set(d.id, {subserie: e.target.value.replace(/[^A-Za-z0-9]/g, '')})} className="w-full"/>
                <Input aria-label={`Data de emissão ${i + 1}`} type="date" value={d.issued_on}
                       onChange={(e) => set(d.id, {issued_on: e.target.value})} className="w-full"/>
                <CurrencyInput aria-label={`Valor ${i + 1}`} placeholder="Valor (opcional)" value={d.amount}
                               onChange={(amount) => set(d.id, {amount})} className="w-full"/>
              </div>
            )}
          </li>
        ))}
      </ul>
      <Button type="button" variant="outline" disabled={lines.length >= CTE_OS_DOCS_MAX}
              onClick={() => onChange([...lines, emptyDocLine()])}>
        Adicionar documento
      </Button>
    </CteSection>
  );
}

/** GTV-e do transporte de valores, cada uma com os componentes que somam o frete (H053). */
export function CteOsGtve({lines, onChange, freightTotal}: {
  lines: CteOsGtveLine[]; onChange: (l: CteOsGtveLine[]) => void; freightTotal: string
}) {
  const set = (id: string, p: Partial<CteOsGtveLine>) => onChange(patchLine(lines, id, p));
  const total = gtveTotalCents(lines);
  const closes = gtveTotalCloses(lines, freightTotal);
  return (
    <CteSection id="cte-os-gtve" title="Quais GTV-e estão neste transporte?"
                hint="Cada GTV-e leva a chave e os componentes do valor; a soma de todos precisa fechar com o frete.">
      <ul aria-label="GTV-e" className="space-y-3">
        {lines.map((g, i) => (
          <li key={g.id} className="space-y-2 rounded-lg border border-gray-200 p-3">
            <div className="flex items-center justify-between gap-2">
              <NumericInput aria-label={`Chave da GTV-e ${i + 1}`} inputMode="numeric" integerPlaces={CTE_ACCESS_KEY_LENGTH}
                            placeholder={`Chave de ${CTE_ACCESS_KEY_LENGTH} dígitos`} value={g.key}
                            onChange={(key) => set(g.id, {key})} className="w-full font-mono"/>
              <Button type="button" variant="ghost" className="text-danger hover:text-red-700"
                      aria-label={`Remover GTV-e ${i + 1}`} onClick={() => onChange(lines.filter((x) => x.id !== g.id))}>
                Remover
              </Button>
            </div>
            <ul aria-label={`Componentes da GTV-e ${i + 1}`} className="space-y-2">
              {g.components.map((c, j) => (
                <li key={c.id} className={cn('grid grid-cols-[1fr_auto] gap-2', c.type === CTE_OS_GTVE_COMPONENT_OTHERS ? 'sm:grid-cols-[1fr_1fr_10rem_auto]' : 'sm:grid-cols-[1fr_10rem_auto]')}>
                  <OptionsSelect ariaLabel={`Tipo do componente ${j + 1} da GTV-e ${i + 1}`} value={c.type} placeholder="Tipo"
                                 options={CTE_OS_GTVE_COMPONENT_TYPES.map((t) => ({value: t.value, label: t.label}))}
                                 onValueChange={(type) => set(g.id, {components: patchLine(g.components, c.id, {type})})}/>
                  {c.type === CTE_OS_GTVE_COMPONENT_OTHERS && (
                    <Input aria-label={`Nome do componente ${j + 1} da GTV-e ${i + 1}`} placeholder="Nome" maxLength={CTE_OS_GTVE_COMPONENT_NAME_MAX}
                           value={c.name} className="w-full"
                           onChange={(e) => set(g.id, {components: patchLine(g.components, c.id, {name: e.target.value.replace(/[\r\n]/g, '')})})}/>
                  )}
                  <CurrencyInput aria-label={`Valor do componente ${j + 1} da GTV-e ${i + 1}`} placeholder="0,00" allowZero={false}
                                 value={c.value} className="w-full"
                                 onChange={(value) => set(g.id, {components: patchLine(g.components, c.id, {value})})}/>
                  <Button type="button" variant="ghost" className="text-danger hover:text-red-700"
                          aria-label={`Remover componente ${j + 1} da GTV-e ${i + 1}`}
                          disabled={g.components.length === 1}
                          onClick={() => set(g.id, {components: g.components.filter((x) => x.id !== c.id)})}>
                    Remover
                  </Button>
                </li>
              ))}
            </ul>
            <div className="flex items-center justify-between gap-2">
              <Button type="button" variant="outline" disabled={g.components.length >= CTE_OS_GTVE_COMPONENT_TYPES.length}
                      onClick={() => set(g.id, {components: [...g.components, emptyGtveComponent()]})}>
                Adicionar componente
              </Button>
              <p className="text-sm text-gray-600">GTV-e: {formatCurrency(fromCents(gtveComponentsCents(g)))}</p>
            </div>
          </li>
        ))}
      </ul>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <Button type="button" variant="outline" disabled={lines.length >= CTE_OS_DOCS_MAX}
                onClick={() => onChange([...lines, emptyGtve()])}>
          Adicionar GTV-e
        </Button>
        {lines.length > 0 && (
          <p data-testid="cte-os-gtve-total" className={cn('text-sm', closes ? 'text-gray-600' : 'text-danger')}>
            Soma das GTV-e: {formatCurrency(fromCents(total))}{!closes && ' (precisa fechar com o frete)'}
          </p>
        )}
      </div>
    </CteSection>
  );
}
