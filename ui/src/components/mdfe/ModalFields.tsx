'use client'

import {useQuery} from '@tanstack/react-query'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {
  CargoUnitItemOut,
  MdfeAirModalIn,
  MdfeRailModalIn,
  MdfeRailWagonIn,
  MdfeWaterModalIn,
  MdfeWaterTerminalIn,
} from '@/lib/types/api'

const EMPTY_WAGON: MdfeRailWagonIn = {
  weight_bc: '', weight_real: '', series: '', number: '', tu: '', wagon_type: '', sequence: '',
}

/** Um voo é válido quando os seis campos do XSD estão preenchidos. */
export function airComplete(a: MdfeAirModalIn): boolean {
  return !!(a.nationality && a.registration && a.flight_number
    && a.origin_airport && a.dest_airport && a.flight_date)
}

/** Um trem precisa de prefixo, origem, destino e ao menos um vagão completo. */
export function railComplete(r: MdfeRailModalIn): boolean {
  return !!(r.train_prefix && r.origin_station && r.dest_station)
    && r.wagons.length > 0
    && r.wagons.every((w) => w.weight_bc && w.weight_real && w.series && w.number && w.tu)
}

function Field({id, label, children}: { id: string; label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={id} className="text-xs font-medium text-gray-600">{label}</Label>
      {children}
    </div>
  )
}

export function AirModalFields({value, onChange}: {
  value: MdfeAirModalIn
  onChange: (v: MdfeAirModalIn) => void
}) {
  const patch = (p: Partial<MdfeAirModalIn>) => onChange({...value, ...p})
  const upper = (s: string) => s.toUpperCase()

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Dados do voo</p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field id="air-nac" label="Nacionalidade da aeronave">
          <Input id="air-nac" maxLength={4} className="w-full" placeholder="PP"
                 value={value.nationality} onChange={(e) => patch({nationality: upper(e.target.value)})}/>
        </Field>
        <Field id="air-matr" label="Marca / matrícula">
          <Input id="air-matr" maxLength={6} className="w-full" placeholder="ABC123"
                 value={value.registration} onChange={(e) => patch({registration: upper(e.target.value)})}/>
        </Field>
        <Field id="air-voo" label="Número do voo">
          <Input id="air-voo" maxLength={9} className="w-full" placeholder="JJ1234"
                 value={value.flight_number} onChange={(e) => patch({flight_number: upper(e.target.value)})}/>
        </Field>
        <Field id="air-data" label="Data do voo">
          <input id="air-data" type="date" value={value.flight_date}
                 onChange={(e) => patch({flight_date: e.target.value})}
                 className="w-full h-11 rounded-md border border-gray-300 px-3 text-sm"/>
        </Field>
        <Field id="air-emb" label="Aeródromo de embarque (IATA)">
          <Input id="air-emb" maxLength={4} className="w-full" placeholder="GRU"
                 value={value.origin_airport} onChange={(e) => patch({origin_airport: upper(e.target.value)})}/>
        </Field>
        <Field id="air-des" label="Aeródromo de destino (IATA)">
          <Input id="air-des" maxLength={4} className="w-full" placeholder="SDU"
                 value={value.dest_airport} onChange={(e) => patch({dest_airport: upper(e.target.value)})}/>
        </Field>
      </div>
    </div>
  )
}

export function RailModalFields({value, onChange}: {
  value: MdfeRailModalIn
  onChange: (v: MdfeRailModalIn) => void
}) {
  const patch = (p: Partial<MdfeRailModalIn>) => onChange({...value, ...p})
  const patchWagon = (i: number, p: Partial<MdfeRailWagonIn>) =>
    patch({wagons: value.wagons.map((w, k) => (k === i ? {...w, ...p} : w))})

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Dados do trem</p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field id="rail-pref" label="Prefixo do trem">
          <Input id="rail-pref" maxLength={10} className="w-full"
                 value={value.train_prefix} onChange={(e) => patch({train_prefix: e.target.value})}/>
        </Field>
        <Field id="rail-dh" label="Data/hora do trem (opcional)">
          <input id="rail-dh" type="datetime-local" value={value.train_datetime ?? ''}
                 onChange={(e) => patch({train_datetime: e.target.value})}
                 className="w-full h-11 rounded-md border border-gray-300 px-3 text-sm"/>
        </Field>
        <Field id="rail-ori" label="Estação de origem">
          <Input id="rail-ori" maxLength={100} className="w-full"
                 value={value.origin_station} onChange={(e) => patch({origin_station: e.target.value})}/>
        </Field>
        <Field id="rail-dest" label="Estação de destino">
          <Input id="rail-dest" maxLength={100} className="w-full"
                 value={value.dest_station} onChange={(e) => patch({dest_station: e.target.value})}/>
        </Field>
      </div>

      <div className="flex items-center justify-between pt-1">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Vagões ({value.wagons.length})
        </p>
        <Button type="button" variant="ghost" size="xs"
                onClick={() => patch({wagons: [...value.wagons, {...EMPTY_WAGON}]})}>
          + Vagão
        </Button>
      </div>
      {value.wagons.length === 0 && (
        <p className="text-xs text-gray-500">
          O manifesto ferroviário precisa de ao menos um vagão. A quantidade sai desta lista.
        </p>
      )}
      {value.wagons.map((w, i) => (
        <div key={i} className="rounded-lg border border-gray-200 p-3 space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-xs font-medium text-gray-600">Vagão {i + 1}</p>
            <Button type="button" variant="ghost" size="xs"
                    onClick={() => patch({wagons: value.wagons.filter((_, k) => k !== i)})}>
              Remover
            </Button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <Field id={`rail-serie-${i}`} label="Série">
              <Input id={`rail-serie-${i}`} maxLength={3} className="w-full"
                     value={w.series} onChange={(e) => patchWagon(i, {series: e.target.value})}/>
            </Field>
            <Field id={`rail-num-${i}`} label="Número">
              <Input id={`rail-num-${i}`} maxLength={9} className="w-full"
                     value={w.number} onChange={(e) => patchWagon(i, {number: e.target.value})}/>
            </Field>
            <Field id={`rail-seq-${i}`} label="Sequência (opcional)">
              <Input id={`rail-seq-${i}`} maxLength={2} className="w-full"
                     value={w.sequence ?? ''} onChange={(e) => patchWagon(i, {sequence: e.target.value})}/>
            </Field>
            <Field id={`rail-pbc-${i}`} label="Peso base de cálculo (t)">
              <NumericInput id={`rail-pbc-${i}`} value={w.weight_bc} decimal decimalPlaces={3} className="w-full"
                            onChange={(v) => patchWagon(i, {weight_bc: v})}/>
            </Field>
            <Field id={`rail-pr-${i}`} label="Peso real (t)">
              <NumericInput id={`rail-pr-${i}`} value={w.weight_real} decimal decimalPlaces={3} className="w-full"
                            onChange={(v) => patchWagon(i, {weight_real: v})}/>
            </Field>
            <Field id={`rail-tu-${i}`} label="Tonelada útil">
              <NumericInput id={`rail-tu-${i}`} value={w.tu} decimal decimalPlaces={3} className="w-full"
                            onChange={(v) => patchWagon(i, {tu: v})}/>
            </Field>
            <Field id={`rail-tpvag-${i}`} label="Tipo do vagão (opcional)">
              <Input id={`rail-tpvag-${i}`} maxLength={3} className="w-full"
                     value={w.wagon_type ?? ''} onChange={(e) => patchWagon(i, {wagon_type: e.target.value})}/>
            </Field>
          </div>
        </div>
      ))}
    </div>
  )
}

/** Uma embarcação é válida com os sete campos obrigatórios do XSD. */
export function waterComplete(w: MdfeWaterModalIn): boolean {
  return !!(w.irin && w.vessel_type && w.vessel_code && w.vessel_name
    && w.voyage_number && w.origin_port.length === 5 && w.dest_port.length === 5)
}

/** tpNav — tipo de navegação. */
const TP_NAV_OPTIONS = [
  {value: '0', label: '0 – Interior'},
  {value: '1', label: '1 – Cabotagem'},
]

/** Lista de pares código/nome: terminais e balsas têm a mesma forma. */
function PairList({title, addLabel, codeLabel, nameLabel, idPrefix, items, onChange, max}: {
  title: string
  addLabel: string
  codeLabel: string
  nameLabel: string
  idPrefix: string
  items: MdfeWaterTerminalIn[]
  onChange: (v: MdfeWaterTerminalIn[]) => void
  max: number
}) {
  const patch = (i: number, p: Partial<MdfeWaterTerminalIn>) =>
    onChange(items.map((t, k) => (k === i ? {...t, ...p} : t)))

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium text-gray-600">{title}</p>
        <Button type="button" variant="ghost" size="xs" disabled={items.length >= max}
                onClick={() => onChange([...items, {code: '', name: ''}])}>
          {addLabel}
        </Button>
      </div>
      {items.map((t, i) => (
        <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto] gap-2 items-end">
          <Field id={`${idPrefix}-code-${i}`} label={codeLabel}>
            <Input id={`${idPrefix}-code-${i}`} className="w-full" value={t.code}
                   onChange={(e) => patch(i, {code: e.target.value})}/>
          </Field>
          <Field id={`${idPrefix}-name-${i}`} label={nameLabel}>
            <Input id={`${idPrefix}-name-${i}`} maxLength={60} className="w-full" value={t.name}
                   onChange={(e) => patch(i, {name: e.target.value})}/>
          </Field>
          <Button type="button" variant="ghost" size="xs"
                  onClick={() => onChange(items.filter((_, k) => k !== i))}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}

/** Unidades do cadastro que viajam vazias — marcadas, não redigitadas. */
function EmptyUnitPicker({label, hint, idPrefix, options, selected, onChange}: {
  label: string
  hint: string
  idPrefix: string
  options: CargoUnitItemOut[]
  selected: string[]
  onChange: (ids: string[]) => void
}) {
  const toggle = (id: string) =>
    onChange(selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id])

  return (
    <div className="space-y-2">
      <p className="text-xs font-medium text-gray-600">{label}</p>
      {options.length === 0 ? (
        <p className="text-xs text-gray-500">{hint}</p>
      ) : (
        options.map((u) => {
          const id = extractId(u.sk, SK_PREFIX.CARGO_UNIT)
          return (
            <label key={u.sk} htmlFor={`${idPrefix}-${id}`}
                   className="flex items-center gap-2 min-h-11 py-1 cursor-pointer text-sm text-gray-700">
              <input id={`${idPrefix}-${id}`} type="checkbox" checked={selected.includes(id)}
                     onChange={() => toggle(id)}
                     className="size-4 cursor-pointer rounded border-gray-300 text-brand-600"/>
              <span>{u.name} <span className="text-gray-400">· {u.id_unid}</span></span>
            </label>
          )
        })
      )}
    </div>
  )
}

export function WaterModalFields({value, onChange}: {
  value: MdfeWaterModalIn
  onChange: (v: MdfeWaterModalIn) => void
}) {
  const {selectedOrg} = useAuth()
  const patch = (p: Partial<MdfeWaterModalIn>) => onChange({...value, ...p})
  const upper = (s: string) => s.toUpperCase()

  const {data: page} = useQuery({
    queryKey: queryKeys.cargoUnits.list(selectedOrg?.pk),
    queryFn: () => apiClient.getCargoUnits({limit: 100}),
    enabled: !!selectedOrg,
  })
  const units = page?.items ?? []
  // O XSD só aceita unidades rodoviárias (tração/reboque) como transporte vazio.
  const emptyTransportOptions = units.filter((u) => u.kind === 'transport' && (u.tp_unid === '1' || u.tp_unid === '2'))
  const emptyCargoOptions = units.filter((u) => u.kind === 'cargo')

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Dados da embarcação</p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field id="water-irin" label="IRIN">
          <Input id="water-irin" maxLength={10} className="w-full" value={value.irin}
                 onChange={(e) => patch({irin: upper(e.target.value)})}/>
        </Field>
        <Field id="water-tpemb" label="Tipo da embarcação">
          <Input id="water-tpemb" maxLength={2} className="w-full" placeholder="01" value={value.vessel_type}
                 onChange={(e) => patch({vessel_type: e.target.value.replace(/\D/g, '')})}/>
        </Field>
        <Field id="water-cembar" label="Código da embarcação">
          <Input id="water-cembar" maxLength={10} className="w-full" value={value.vessel_code}
                 onChange={(e) => patch({vessel_code: upper(e.target.value)})}/>
        </Field>
        <Field id="water-xembar" label="Nome da embarcação">
          <Input id="water-xembar" maxLength={60} className="w-full" value={value.vessel_name}
                 onChange={(e) => patch({vessel_name: e.target.value})}/>
        </Field>
        <Field id="water-nviag" label="Número da viagem">
          <Input id="water-nviag" maxLength={10} className="w-full" value={value.voyage_number}
                 onChange={(e) => patch({voyage_number: upper(e.target.value)})}/>
        </Field>
        <Field id="water-mmsi" label="MMSI (opcional)">
          <Input id="water-mmsi" maxLength={9} className="w-full" value={value.mmsi ?? ''}
                 onChange={(e) => patch({mmsi: e.target.value.replace(/\D/g, '')})}/>
        </Field>
        <Field id="water-prtemb" label="Porto de embarque (UN/LOCODE)">
          <Input id="water-prtemb" maxLength={5} className="w-full" placeholder="BRSSZ" value={value.origin_port}
                 onChange={(e) => patch({origin_port: upper(e.target.value)})}/>
        </Field>
        <Field id="water-prtdest" label="Porto de destino (UN/LOCODE)">
          <Input id="water-prtdest" maxLength={5} className="w-full" placeholder="BRRIO" value={value.dest_port}
                 onChange={(e) => patch({dest_port: upper(e.target.value)})}/>
        </Field>
        <Field id="water-prttrans" label="Porto de transbordo (opcional)">
          <Input id="water-prttrans" maxLength={60} className="w-full" value={value.transit_port ?? ''}
                 onChange={(e) => patch({transit_port: e.target.value})}/>
        </Field>
        <Field id="water-tpnav" label="Tipo de navegação (opcional)">
          <OptionsSelect id="water-tpnav" value={value.navigation_type ?? ''}
                         placeholder="Não informado"
                         onValueChange={(v: string) => patch({navigation_type: v})}
                         options={[...TP_NAV_OPTIONS]}/>
        </Field>
      </div>

      <PairList title="Terminais de carregamento (até 5)" addLabel="+ Terminal"
                codeLabel="Código" nameLabel="Nome" idPrefix="water-tcar" max={5}
                items={value.loading_terminals ?? []}
                onChange={(v) => patch({loading_terminals: v})}/>

      <PairList title="Terminais de descarregamento (até 5)" addLabel="+ Terminal"
                codeLabel="Código" nameLabel="Nome" idPrefix="water-tdes" max={5}
                items={value.unloading_terminals ?? []}
                onChange={(v) => patch({unloading_terminals: v})}/>

      <PairList title="Balsas do comboio (até 30)" addLabel="+ Balsa"
                codeLabel="Código" nameLabel="Nome da balsa" idPrefix="water-balsa" max={30}
                items={value.barges ?? []}
                onChange={(v) => patch({barges: v})}/>

      <EmptyUnitPicker label="Unidades de carga vazias" idPrefix="water-uc"
                       hint="Cadastre contêineres ou pallets em Cadastros → Unidades de carga para marcá-los aqui."
                       options={emptyCargoOptions} selected={value.empty_cargo_unit_ids ?? []}
                       onChange={(ids) => patch({empty_cargo_unit_ids: ids})}/>

      <EmptyUnitPicker label="Unidades de transporte vazias (rodoviárias)" idPrefix="water-ut"
                       hint="Só carretas e reboques (tração ou reboque) entram aqui, e vêm do cadastro de unidades."
                       options={emptyTransportOptions} selected={value.empty_transport_unit_ids ?? []}
                       onChange={(ids) => patch({empty_transport_unit_ids: ids})}/>
    </div>
  )
}
