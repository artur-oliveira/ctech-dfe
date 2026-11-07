'use client';

import {useState} from 'react';
import {Input} from '@/components/ui/input';
import {Combobox} from '@/components/ui/combobox';
import {Button} from '@aoctech/ui';
import {CITIES, CITY_OPTIONS} from '@/lib/data/cities';
import {BACEN_FOREIGN_COUNTRY_OPTIONS, bacenCountryName} from '@/lib/data/bacen_countries';
import {C_MUN_EXTERIOR, IE_EXEMPT, LAYOUT_LIMITS, UF_EXTERIOR} from '@/lib/constants/layout-limits';
import {maskCep} from '@/lib/utils/masks';
import type {NfeLocalIn, NfeLocalOut} from '@/lib/types/api';

export interface LocationPickerProps {
  label: string
  savedLocations: NfeLocalOut[]
  value: NfeLocalIn | null
  onChange: (loc: NfeLocalIn | null) => void
  save: boolean
  onSaveChange: (save: boolean) => void
  /** Recusa da emissão por campo do NfeLocalBody (`x_lgr`, `c_pais`...), mostrada no próprio campo. */
  errors?: Record<string, string>
}

const L = LAYOUT_LIMITS;

const EMPTY_LOCAL: NfeLocalIn = {
  x_lgr: '', nro: '', x_cpl: '', x_bairro: '', c_mun: '', x_mun: '', uf: 'SP',
};

function localsMatch(saved: NfeLocalOut, current: NfeLocalIn | null): boolean {
  if (!current) return false;
  return saved.x_lgr === current.x_lgr && saved.nro === current.nro;
}

function FieldError({message}: { message?: string }) {
  return message ? <p className="text-[0.8rem] text-danger">{message}</p> : null;
}

/** Free-form entrega/retirada address for NF-e emission, with a picker over
 * locations saved from previous emissions (see delivery_locations /
 * pickup_locations) so the user isn't retyping the same address every time.
 * A place abroad (UF EX) takes the country from the BACEN table, no CEP and the
 * municipality code 9999999, as TLocal requires. */
export function LocationPicker({label, savedLocations, value, onChange, save, onSaveChange, errors = {}}: LocationPickerProps) {
  // Aberto quando já há local (rascunho, volta de outro passo, recusa da emissão apontando para cá).
  const [open, setOpen] = useState(value !== null);
  const [manual, setManual] = useState(false);

  if (!open) {
    return (
      <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(true)}
              className="gap-1.5 text-brand-600 hover:text-brand-700 px-0">
        + {label}
      </Button>
    );
  }

  const set = (patch: Partial<NfeLocalIn>) => onChange({...(value ?? EMPTY_LOCAL), ...patch});
  const showManualForm = manual || savedLocations.length === 0 || Object.keys(errors).length > 0;
  const abroad = value?.uf === UF_EXTERIOR;
  const invalid = (field: string) => (errors[field] ? {'aria-invalid': true} : {});

  return (
    <div className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-4">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-gray-600">{label}</p>
        <Button type="button" variant="ghost" size="sm"
                onClick={() => {
                  setOpen(false);
                  setManual(false);
                  onChange(null);
                  onSaveChange(false);
                }}
                className="text-red-700 hover:text-red-800 hover:bg-red-50">
          Remover
        </Button>
      </div>

      {!showManualForm && (
        <div className="space-y-2">
          <p className="text-xs text-gray-500">Locais salvos</p>
          <div className="flex flex-wrap gap-2">
            {savedLocations.map((loc, i) => (
              <button
                key={i}
                type="button"
                onClick={() => onChange({
                  cnpj: loc.cnpj, cpf: loc.cpf, x_nome: loc.x_nome,
                  x_lgr: loc.x_lgr, nro: loc.nro, x_cpl: loc.x_cpl,
                  x_bairro: loc.x_bairro, c_mun: loc.c_mun, x_mun: loc.x_mun,
                  uf: loc.uf, cep: loc.cep, ie: loc.ie, c_pais: loc.c_pais, x_pais: loc.x_pais,
                  fone: loc.fone, email: loc.email,
                })}
                className={[
                  'min-h-11 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors sm:min-h-0',
                  localsMatch(loc, value)
                    ? 'border-brand-400 bg-brand-50 text-brand-700'
                    : 'border-gray-200 bg-white text-gray-700 hover:border-brand-300',
                ].join(' ')}
              >
                {loc.x_lgr}, {loc.nro}{loc.uf === UF_EXTERIOR && loc.x_pais ? ` · ${loc.x_pais}` : ''}
              </button>
            ))}
          </div>
          <Button type="button" variant="ghost" size="sm" onClick={() => setManual(true)}
                  className="text-brand-600 hover:text-brand-700 px-0">
            + Endereço diferente
          </Button>
        </div>
      )}

      {showManualForm && (
        <div className="space-y-3">
          <label className="flex min-h-11 items-center gap-2 text-sm text-gray-700 sm:min-h-0">
            <input type="checkbox" checked={abroad} className="size-4 rounded border-gray-300"
                   onChange={(e) => set(e.target.checked
                     ? {uf: UF_EXTERIOR, c_mun: C_MUN_EXTERIOR, x_mun: '', cep: null}
                     : {uf: 'SP', c_mun: '', x_mun: '', c_pais: null, x_pais: null})}/>
            Local no exterior
          </label>

          {abroad ? (
            <div className="space-y-1">
              <Combobox value={value?.c_pais ?? ''} options={BACEN_FOREIGN_COUNTRY_OPTIONS}
                        placeholder="País" searchPlaceholder="Digite o país ou o código…" clearable={false}
                        onValueChange={(code) => set({c_pais: code, x_pais: bacenCountryName(code)})}/>
              <FieldError message={errors.c_pais ?? errors.x_pais}/>
            </div>
          ) : (
            <div className="space-y-1">
              <Input placeholder="CEP" inputMode="numeric" maxLength={9} value={maskCep(value?.cep ?? '')}
                     onChange={(e) => set({cep: e.target.value.replace(/\D/g, '').slice(0, 8) || null})}/>
              <FieldError message={errors.cep}/>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="space-y-1 sm:col-span-2">
              <Input placeholder="Logradouro" value={value?.x_lgr ?? ''} maxLength={L.addressText.max}
                     {...invalid('x_lgr')} onChange={(e) => set({x_lgr: e.target.value})}/>
              <FieldError message={errors.x_lgr}/>
            </div>
            <div className="space-y-1">
              <Input placeholder="Número" value={value?.nro ?? ''} maxLength={L.localNumber.max}
                     {...invalid('nro')} onChange={(e) => set({nro: e.target.value})}/>
              <FieldError message={errors.nro}/>
            </div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1">
              <Input placeholder="Complemento" value={value?.x_cpl ?? ''} maxLength={L.complement.max}
                     {...invalid('x_cpl')} onChange={(e) => set({x_cpl: e.target.value})}/>
              <FieldError message={errors.x_cpl}/>
            </div>
            <div className="space-y-1">
              <Input placeholder="Bairro" value={value?.x_bairro ?? ''} maxLength={L.addressText.max}
                     {...invalid('x_bairro')} onChange={(e) => set({x_bairro: e.target.value})}/>
              <FieldError message={errors.x_bairro}/>
            </div>
          </div>
          <div className="space-y-1">
            {abroad ? (
              <Input placeholder="Cidade" value={value?.x_mun ?? ''} maxLength={L.addressText.max}
                     {...invalid('x_mun')} onChange={(e) => set({x_mun: e.target.value})}/>
            ) : (
              <Combobox value={value?.c_mun ?? ''} options={CITY_OPTIONS} clearable={false}
                        placeholder="Cidade / UF" searchPlaceholder="Digite cidade ou UF…"
                        onValueChange={(code) => {
                          const city = CITIES.find((c) => c.code === code);
                          if (city) set({c_mun: code, x_mun: city.description, uf: city.uf});
                        }}/>
            )}
            <FieldError message={errors.x_mun ?? errors.c_mun}/>
          </div>
          <div className="space-y-1">
            <Input placeholder="Inscrição estadual (opcional)" value={value?.ie === IE_EXEMPT ? '' : value?.ie ?? ''}
                   maxLength={L.stateRegistration.max} disabled={value?.ie === IE_EXEMPT}
                   {...invalid('ie')} onChange={(e) => set({ie: e.target.value || null})}/>
            <label className="flex min-h-11 items-center gap-2 text-xs text-gray-600 sm:min-h-0">
              <input type="checkbox" checked={value?.ie === IE_EXEMPT} className="size-4 rounded border-gray-300"
                     onChange={(e) => set({ie: e.target.checked ? IE_EXEMPT : null})}/>
              Isento de inscrição
            </label>
            <FieldError message={errors.ie}/>
          </div>
          <label className="flex min-h-11 items-center gap-2 text-xs text-gray-600 sm:min-h-0">
            <input type="checkbox" checked={save} onChange={(e) => onSaveChange(e.target.checked)}
                   className="rounded border-gray-300"/>
            Salvar este local para reutilizar
          </label>
        </div>
      )}
    </div>
  );
}
