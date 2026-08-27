'use client'

import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import type {MdfeAirModalIn, MdfeRailModalIn, MdfeRailWagonIn} from '@/lib/types/api'

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
