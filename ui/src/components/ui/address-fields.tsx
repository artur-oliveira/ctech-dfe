'use client';

import {useState} from 'react';
import {type Control, type FieldPath, type UseFormSetValue, useWatch} from 'react-hook-form';
import {FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Combobox} from '@/components/ui/combobox';
import {CITIES, CITY_OPTIONS} from '@/lib/data/cities';
import {maskCep} from '@/lib/utils/masks';
import type {EntityFormData} from "@/lib/schemas/entity";
import {LengthHint} from '@/components/ui/length-hint';
import {bacenCountryName, BACEN_FOREIGN_COUNTRY_OPTIONS} from '@/lib/data/bacen_countries';
import {C_MUN_EXTERIOR, LAYOUT_LIMITS, UF_EXTERIOR} from '@/lib/constants/layout-limits';

const L = LAYOUT_LIMITS;

interface AddressFieldsProps {
  control: Control<EntityFormData>
  setValue: UseFormSetValue<EntityFormData>
  /** Base path for the address object, e.g. "person.addresses.0" */
  basePath: FieldPath<EntityFormData>
}

export function AddressFields({control, setValue, basePath}: AddressFieldsProps) {
  const [cepLoading, setCepLoading] = useState(false);

  const p = (field: string) => `${basePath}.${field}` as FieldPath<EntityFormData>;
  const abroad = useWatch({control, name: p('state_federation')}) === UF_EXTERIOR;
  const street = useWatch({control, name: p('street')});

  // Exterior é um estado do endereço inteiro, não um campo: troca a UF, fixa o município 9999999 e limpa o que
  // não vale do outro lado (CEP e cidade brasileira, ou o país).
  const setAbroad = (on: boolean) => {
    // Sem validar: trocar de lado não é motivo para pintar de vermelho campos que o usuário ainda vai preencher.
    const opts = {shouldDirty: true};
    setValue(p('state_federation'), on ? UF_EXTERIOR : 'SP', opts);
    setValue(p('city_ibge_code'), on ? C_MUN_EXTERIOR : '', opts);
    setValue(p('city'), '', opts);
    setValue(p('postal_code'), '', opts);
    setValue(p('country_code'), '', opts);
    setValue(p('country_name'), '', opts);
  };

  const lookupCep = async (raw: string) => {
    if (raw.length !== 8) return;
    setCepLoading(true);
    try {
      const res = await fetch(`https://viacep.com.br/ws/${raw}/json/`);
      const data = await res.json() as {
        erro?: boolean; logradouro?: string; bairro?: string
        localidade?: string; uf?: string; ibge?: string
      };
      if (data.erro) return;
      setValue(p('street'), data.logradouro ?? '', {shouldValidate: true});
      setValue(p('neighborhood'), data.bairro ?? '', {shouldValidate: true});
      setValue(p('city'), data.localidade ?? '', {shouldValidate: true});
      setValue(p('state_federation'), data.uf ?? 'SP', {shouldValidate: true});
      setValue(p('city_ibge_code'), data.ibge ?? '', {shouldValidate: true});
    } catch {
      // silently ignore network errors
    } finally {
      setCepLoading(false);
    }
  };

  return (
    <div className="space-y-3">
      <label className="flex min-h-11 items-center gap-2 text-sm text-gray-700 sm:min-h-0">
        <input type="checkbox" id={`${basePath}.abroad`} checked={abroad} onChange={(e) => setAbroad(e.target.checked)}
               className="size-4 rounded border-gray-300"/>
        Endereço no exterior
      </label>

      {abroad && (
        <FormField
          control={control}
          name={p('country_code')}
          render={({field}) => (
            <FormItem>
              <FormLabel>País</FormLabel>
              <Combobox
                id={field.name}
                value={String(field.value ?? '')}
                onValueChange={(code) => {
                  field.onChange(code);
                  setValue(p('country_name'), bacenCountryName(code), {shouldValidate: true});
                }}
                options={BACEN_FOREIGN_COUNTRY_OPTIONS}
                placeholder="Selecione o país…"
                searchPlaceholder="Digite o país ou o código…"
                clearable={false}
              />
              <FormMessage/>
            </FormItem>
          )}
        />
      )}

      {/* CEP */}
      {!abroad && <FormField
        control={control}
        name={p('postal_code')}
        render={({field}) => (
          <FormItem>
            <FormLabel>
              CEP
              {cepLoading && <span className="ml-2 text-xs font-normal text-gray-400">Buscando…</span>}
            </FormLabel>
            <Input
              id={field.name}
              placeholder="12345-678"
              maxLength={9}
              autoComplete="postal-code"
              value={maskCep(String(field.value ?? ''))}
              onChange={(e) => {
                const raw = e.target.value.replace(/\D/g, '').slice(0, 8);
                field.onChange(raw);
                if (raw.length === 8) void lookupCep(raw);
              }}
              onBlur={field.onBlur}
              ref={field.ref}
            />
            <FormMessage/>
          </FormItem>
        )}
      />}

      {/* Logradouro */}
      <FormField
        control={control}
        name={p('street')}
        render={({field}) => (
          <FormItem>
            <FormLabel>Logradouro</FormLabel>
            <Input {...field} id={field.name} placeholder="Av. Paulista" maxLength={L.addressText.max}
                   value={String(field.value ?? '')}/>
            <LengthHint value={String(street ?? '')} max={L.addressText.max}/>
            <FormMessage/>
          </FormItem>
        )}
      />

      {/* Número + Bairro + Complemento */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <FormField
          control={control}
          name={p('number')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Número</FormLabel>
              <Input {...field} id={field.name} placeholder="1000 ou S/N" maxLength={L.addressNumber.max}
                     value={String(field.value ?? '')}/>
              <FormMessage/>
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={p('neighborhood')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Bairro</FormLabel>
              <Input {...field} id={field.name} placeholder="Centro" maxLength={L.addressText.max}
                     value={String(field.value ?? '')}/>
              <FormMessage/>
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={p('complement')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Complemento</FormLabel>
              <Input {...field} id={field.name} placeholder="Apto 101" maxLength={L.complement.max}
                     value={String(field.value ?? '')}/>
              <FormMessage/>
            </FormItem>
          )}
        />
      </div>

      {/* Cidade: no exterior é o nome digitado (não há tabela); no Brasil, o combobox de cidade/UF/IBGE */}
      {abroad ? (
        <FormField
          control={control}
          name={p('city')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Cidade</FormLabel>
              <Input {...field} id={field.name} placeholder="Lisboa" maxLength={L.addressText.max}
                     value={String(field.value ?? '')}/>
              <p className="text-xs text-gray-500">O município sai como {C_MUN_EXTERIOR}, o código do exterior.</p>
              <FormMessage/>
            </FormItem>
          )}
        />
      ) : (
        <FormField
          control={control}
          name={p('city_ibge_code')}
          render={({field}) => (
            <FormItem>
              <FormLabel>Cidade / UF</FormLabel>
              <Combobox
                id={field.name}
                value={field.value?.toString() || "SP"}
                onValueChange={(code) => {
                  const city = CITIES.find((c) => c.code === code);
                  if (!city) return;
                  field.onChange(code);
                  setValue(p('city'), city.description, {shouldValidate: true});
                  setValue(p('state_federation'), city.uf, {shouldValidate: true});
                }}
                options={CITY_OPTIONS}
                placeholder="Selecione a cidade…"
                searchPlaceholder="Digite cidade ou UF…"
              />
              <FormMessage/>
            </FormItem>
          )}
        />
      )}
    </div>
  );
}
