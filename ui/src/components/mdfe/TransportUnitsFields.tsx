'use client'

import {useQuery} from '@tanstack/react-query'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {OptionsSelect} from '@/components/ui/options-select'
import {RowCheckbox} from '@/components/ui/table-shell'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {MdfeTransportUnitIn} from '@/lib/types/api'

export interface TransportUnitsFieldsProps {
  units: MdfeTransportUnitIn[]
  onChange: (units: MdfeTransportUnitIn[]) => void
  /** Chaves dos documentos manifestados, para escolher o que cada unidade leva. */
  documentKeys: string[]
}

/**
 * Unidades de transporte da viagem (infUnidTransp). A unidade vem do cadastro;
 * aqui só se diz quais documentos ela leva e quais unidades de carga vão
 * dentro. O rateio (`qtdRat`) é calculado no backend a partir dos pesos.
 */
export function TransportUnitsFields({units, onChange, documentKeys}: TransportUnitsFieldsProps) {
  const {selectedOrg} = useAuth()

  const {data: page} = useQuery({
    queryKey: queryKeys.cargoUnits.list(selectedOrg?.pk),
    queryFn: () => apiClient.getCargoUnits({limit: 100}),
    enabled: !!selectedOrg,
  })
  const all = page?.items ?? []
  const transportOptions = all.filter((u) => u.kind === 'transport')
  const cargoOptions = all.filter((u) => u.kind === 'cargo')

  const patch = (i: number, p: Partial<MdfeTransportUnitIn>) =>
    onChange(units.map((u, k) => (k === i ? {...u, ...p} : u)))

  const toggleDoc = (i: number, key: string) => {
    const current = units[i].document_keys
    patch(i, {
      document_keys: current.includes(key) ? current.filter((k) => k !== key) : [...current, key],
    })
  }

  const toggleCargoUnit = (i: number, id: string) => {
    const current = units[i].cargo_unit_ids ?? []
    patch(i, {cargo_unit_ids: current.includes(id) ? current.filter((c) => c !== id) : [...current, id]})
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Unidades de transporte (opcional)
        </p>
        <Button type="button" variant="ghost" size="xs" disabled={transportOptions.length === 0}
                onClick={() => onChange([...units, {cargo_unit_id: '', document_keys: [], cargo_unit_ids: []}])}>
          + Unidade
        </Button>
      </div>

      {transportOptions.length === 0 ? (
        <p className="text-xs text-gray-500">
          Cadastre uma carreta ou vagão em <span className="font-medium">Cadastros → Unidades de carga</span> para
          apontá-la aqui.
        </p>
      ) : (
        <p className="text-xs text-gray-500">
          O rateio da carga entre os documentos é calculado a partir dos pesos — nada de percentual aqui.
        </p>
      )}

      {units.map((unit, i) => (
        <div key={i} className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-end gap-2">
            <div className="flex flex-col gap-1 flex-1">
              <Label htmlFor={`unit-${i}`} className="text-xs font-medium text-gray-600">Unidade</Label>
              <OptionsSelect id={`unit-${i}`} value={unit.cargo_unit_id} placeholder="Selecione"
                             onValueChange={(id: string) => patch(i, {cargo_unit_id: id})}
                             options={transportOptions.map((u) => ({
                               value: extractId(u.sk, SK_PREFIX.CARGO_UNIT),
                               label: `${u.name} · ${u.id_unid}`,
                             }))}/>
            </div>
            <Button type="button" variant="ghost" size="xs"
                    onClick={() => onChange(units.filter((_, k) => k !== i))}>
              Remover
            </Button>
          </div>

          <div className="space-y-1">
            <p className="text-xs font-medium text-gray-600">Documentos nesta unidade</p>
            {documentKeys.length === 0 ? (
              <p className="text-xs text-gray-500">Escolha os documentos do manifesto primeiro.</p>
            ) : documentKeys.map((key) => (
              <label key={key} className="flex items-center gap-2 text-xs text-gray-600 font-mono">
                <RowCheckbox checked={unit.document_keys.includes(key)}
                             onChange={() => toggleDoc(i, key)}
                             ariaLabel={`Documento ${key} nesta unidade`}/>
                {key}
              </label>
            ))}
          </div>

          {cargoOptions.length > 0 && (
            <div className="space-y-1">
              <p className="text-xs font-medium text-gray-600">Unidades de carga dentro dela</p>
              {cargoOptions.map((c) => {
                const id = extractId(c.sk, SK_PREFIX.CARGO_UNIT)
                return (
                  <label key={c.sk} className="flex items-center gap-2 text-xs text-gray-600">
                    <RowCheckbox checked={(unit.cargo_unit_ids ?? []).includes(id)}
                                 onChange={() => toggleCargoUnit(i, id)}
                                 ariaLabel={`${c.name} nesta unidade`}/>
                    {c.name} · {c.id_unid}
                  </label>
                )
              })}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
