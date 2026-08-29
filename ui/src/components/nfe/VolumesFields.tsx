'use client'

import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import type {NfeReboqueIn, NfeVolIn} from '@/lib/types/api'

export interface VolumesFieldsProps {
  vols: NfeVolIn[]
  onVolsChange: (vols: NfeVolIn[]) => void
  reboques: NfeReboqueIn[]
  onReboquesChange: (reboques: NfeReboqueIn[]) => void
}

const EMPTY_VOL: NfeVolIn = {q_vol: '', esp: '', marca: '', n_vol: '', peso_l: '', peso_b: '', lacres: []}
const EMPTY_REBOQUE: NfeReboqueIn = {placa: '', uf: 'SP', rntc: ''}

/** Um reboque só existe atrelado a um veículo; o XSD limita a 5. */
const MAX_REBOQUES = 5

/**
 * Volumes (transp/vol) e reboques (transp/reboque) da emissão. Sem volume
 * informado, o backend deriva um volume único com o peso somado dos itens —
 * que é o que a maioria das notas precisa, então esta seção fica opcional.
 */
export function VolumesFields({vols, onVolsChange, reboques, onReboquesChange}: VolumesFieldsProps) {
  const patchVol = (i: number, patch: Partial<NfeVolIn>) =>
    onVolsChange(vols.map((v, k) => (k === i ? {...v, ...patch} : v)))
  const patchReboque = (i: number, patch: Partial<NfeReboqueIn>) =>
    onReboquesChange(reboques.map((r, k) => (k === i ? {...r, ...patch} : r)))

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label className="text-sm font-medium text-gray-700">Volumes</Label>
          <Button type="button" variant="ghost" size="xs" onClick={() => onVolsChange([...vols, EMPTY_VOL])}>
            + Volume
          </Button>
        </div>
        {vols.length === 0 && (
          <p className="text-xs text-gray-500">
            Sem volume informado, o peso dos itens vira um volume único automaticamente.
          </p>
        )}
        {vols.map((vol, i) => (
          <div key={i} className="space-y-2 rounded border border-gray-200 p-2">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <div>
                <Label htmlFor={`vol-qvol-${i}`}>Quantidade</Label>
                <NumericInput id={`vol-qvol-${i}`} value={vol.q_vol ?? ''} decimalPlaces={0}
                              onChange={(v) => patchVol(i, {q_vol: v})}/>
              </div>
              <div>
                <Label htmlFor={`vol-esp-${i}`}>Espécie</Label>
                <Input id={`vol-esp-${i}`} maxLength={60} value={vol.esp ?? ''} placeholder="CAIXA"
                       onChange={(e) => patchVol(i, {esp: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor={`vol-marca-${i}`}>Marca</Label>
                <Input id={`vol-marca-${i}`} maxLength={60} value={vol.marca ?? ''}
                       onChange={(e) => patchVol(i, {marca: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor={`vol-nvol-${i}`}>Numeração</Label>
                <Input id={`vol-nvol-${i}`} maxLength={60} value={vol.n_vol ?? ''} placeholder="001/002"
                       onChange={(e) => patchVol(i, {n_vol: e.target.value})}/>
              </div>
              <div>
                <Label htmlFor={`vol-pesol-${i}`}>Peso líquido (kg)</Label>
                <NumericInput id={`vol-pesol-${i}`} value={vol.peso_l ?? ''} decimalPlaces={3}
                              onChange={(v) => patchVol(i, {peso_l: v})}/>
              </div>
              <div>
                <Label htmlFor={`vol-pesob-${i}`}>Peso bruto (kg)</Label>
                <NumericInput id={`vol-pesob-${i}`} value={vol.peso_b ?? ''} decimalPlaces={3}
                              onChange={(v) => patchVol(i, {peso_b: v})}/>
              </div>
            </div>
            <div>
              <Label htmlFor={`vol-lacres-${i}`}>Lacres (separados por vírgula)</Label>
              <Input id={`vol-lacres-${i}`} value={(vol.lacres ?? []).join(', ')}
                     placeholder="L1, L2"
                     onChange={(e) => patchVol(i, {
                       lacres: e.target.value.split(',').map((s) => s.trim()).filter(Boolean),
                     })}/>
            </div>
            <Button type="button" variant="ghost" size="xs"
                    onClick={() => onVolsChange(vols.filter((_, k) => k !== i))}>
              Remover volume
            </Button>
          </div>
        ))}
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label className="text-sm font-medium text-gray-700">Reboques</Label>
          <Button type="button" variant="ghost" size="xs" disabled={reboques.length >= MAX_REBOQUES}
                  onClick={() => onReboquesChange([...reboques, EMPTY_REBOQUE])}>
            + Reboque
          </Button>
        </div>
        {reboques.map((reb, i) => (
          <div key={i} className="grid grid-cols-1 sm:grid-cols-4 gap-2 items-end rounded border border-gray-200 p-2">
            <div>
              <Label htmlFor={`reb-placa-${i}`}>Placa</Label>
              <Input id={`reb-placa-${i}`} maxLength={7} value={reb.placa}
                     onChange={(e) => patchReboque(i, {placa: e.target.value.toUpperCase()})}/>
            </div>
            <div>
              <Label htmlFor={`reb-uf-${i}`}>UF</Label>
              <OptionsSelect id={`reb-uf-${i}`} value={reb.uf} options={UF_OPTIONS}
                             onValueChange={(v: string) => patchReboque(i, {uf: v})}/>
            </div>
            <div>
              <Label htmlFor={`reb-rntc-${i}`}>RNTC</Label>
              <Input id={`reb-rntc-${i}`} maxLength={20} value={reb.rntc ?? ''}
                     onChange={(e) => patchReboque(i, {rntc: e.target.value})}/>
            </div>
            <Button type="button" variant="ghost" size="xs"
                    onClick={() => onReboquesChange(reboques.filter((_, k) => k !== i))}>
              Remover
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}
