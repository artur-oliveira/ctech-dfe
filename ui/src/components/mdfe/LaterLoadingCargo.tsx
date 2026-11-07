'use client';

import {Label} from '@aoctech/ui';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {NumericInput} from '@/components/ui/numeric-input';
import {CITIES, CITY_OPTIONS} from '@/lib/data/cities';
import type {MdfeMunIn} from '@/lib/types/api';

/** Município do carregamento posterior com a UF, que vira início e fim da viagem (F21, F22). */
export interface LaterLoadingMun extends MdfeMunIn {
  uf: string
}

export interface LaterLoadingCargoProps {
  mun: LaterLoadingMun | null
  onMunChange: (mun: LaterLoadingMun | null) => void
  cargoValue: string
  onCargoValueChange: (v: string) => void
  cargoWeight: string
  onCargoWeightChange: (v: string) => void
  /** Recusas da api para o carregamento posterior (F21 a F24, F27), mostradas junto do campo. */
  errors?: string[]
}

/**
 * Carga do carregamento posterior (`ide/indCarregaPosterior`): sem documentos de onde derivar, o manifesto leva um
 * único município, que é o de carregamento e o de descarregamento (F21), e o valor e o peso da carga, que valem 0
 * quando ficam vazios. As notas entram depois, por inclusão de DF-e.
 */
export function LaterLoadingCargo({
                                    mun, onMunChange, cargoValue, onCargoValueChange, cargoWeight, onCargoWeightChange,
                                    errors = [],
                                  }: LaterLoadingCargoProps) {
  const pick = (code: string) => {
    const city = CITIES.find((c) => c.code === code);
    onMunChange(city ? {ibge_code: city.code, city: city.description, uf: city.uf} : null);
  };

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
        <p className="text-sm text-gray-700">
          Carregamento posterior: o manifesto sai sem notas. Depois da autorização, inclua as NF-e pelo botão
          {' '}<b>Incluir NF-e</b> no detalhe do MDF-e.
        </p>
        <div className="flex flex-col gap-1">
          <Label htmlFor="mdfe-later-mun" className="text-xs font-medium text-gray-600">
            Município de carregamento e descarregamento
          </Label>
          <Combobox id="mdfe-later-mun" value={mun?.ibge_code ?? null} onValueChange={pick} options={CITY_OPTIONS}
                    placeholder="Selecione o município" searchPlaceholder="Buscar município…"/>
          <p className="text-xs text-gray-500">
            No carregamento posterior a viagem começa e termina no mesmo município, dentro de um estado.
          </p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <Label htmlFor="mdfe-later-value" className="text-xs font-medium text-gray-600">Valor da carga (opcional)</Label>
            <CurrencyInput id="mdfe-later-value" value={cargoValue} onChange={onCargoValueChange} allowZero
                           placeholder="R$ 0,00" className="w-full"/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="mdfe-later-weight" className="text-xs font-medium text-gray-600">Peso da carga em kg (opcional)</Label>
            <NumericInput id="mdfe-later-weight" value={cargoWeight} onChange={onCargoWeightChange} decimal decimalPlaces={4}
                          placeholder="0,0000" className="w-full"/>
          </div>
        </div>
        <p className="text-xs text-gray-500">Vazios, valor e peso saem como zero.</p>
        {errors.map((e) => <p key={e} role="alert" className="text-xs text-red-600">{e}</p>)}
      </div>
    </div>
  );
}
