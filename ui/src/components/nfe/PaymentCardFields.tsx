'use client'

import {OptionsSelect} from '@/components/ui/options-select'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {maskCnpj} from '@/lib/utils/masks'
import type {NfeCardIn} from '@/lib/types/api'

// Card brand codes (cartao de crédito/débito). Shared by NF-e and NFC-e.
export const CARD_BAND_OPTIONS = [
  {value: '01', label: '01 – Visa'},
  {value: '02', label: '02 – Mastercard'},
  {value: '03', label: '03 – American Express'},
  {value: '04', label: '04 – Sorocred'},
  {value: '05', label: '05 – Diners Club'},
  {value: '06', label: '06 – Elo'},
  {value: '07', label: '07 – Hipercard'},
  {value: '99', label: '99 – Outros'},
]

// Payment method codes that carry card/PIX transaction data (tpIntegra/tBand/…).
// 03=Crédito, 04=Débito, 10/11=Vale, 12=PIX dinâmico, 13=PIX, 17=PIX estático.
export const CARD_PAYMENT_TYPES = new Set(['03', '04', '10', '11', '12', '13', '17'])

export const isPixPaymentType = (paymentType: string): boolean =>
  paymentType === '17' || paymentType === '12' || paymentType === '13'

/**
 * Card / PIX transaction fields (tpIntegra, bandeira, NSU/autorização, CNPJ).
 * Used identically by the NF-e and NFC-e issuance forms.
 */
export function PaymentCardFields({card, onChange, isPix}: {
  card: NfeCardIn | null
  onChange: (card: NfeCardIn) => void
  isPix: boolean
}) {
  const set = (patch: Partial<NfeCardIn>) => onChange({...(card ?? {tp_integra: '2'}), ...patch})
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
      <div className="flex flex-col gap-1">
        <Label className="text-xs font-medium text-gray-600">Integração</Label>
        <OptionsSelect value={card?.tp_integra ?? '2'}
                       onValueChange={(v) => set({tp_integra: v as '1' | '2'})}
                       options={[{value: '1', label: '1 – Integrado (TEF)'}, {value: '2', label: '2 – Não integrado'}]}/>
      </div>
      {!isPix && (
        <div className="flex flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">Bandeira</Label>
          <OptionsSelect value={card?.t_band ?? ''} onValueChange={(v) => set({t_band: v})}
                         options={CARD_BAND_OPTIONS}/>
        </div>
      )}
      <div className="flex flex-col gap-1">
        <Label className="text-xs font-medium text-gray-600">{isPix ? 'Chave/NSU' : 'NSU / Autorização'}</Label>
        <Input value={card?.c_aut ?? ''} onChange={(e) => set({c_aut: e.target.value})}
               placeholder="Código de autorização"/>
      </div>
      <div className="flex flex-col gap-1">
        <Label className="text-xs font-medium text-gray-600">CNPJ Instituição</Label>
        <Input value={maskCnpj(card?.cnpj ?? '')}
               onChange={(e) => set({cnpj: e.target.value.replace(/\D/g, '').slice(0, 14)})}
               placeholder="00.000.000/0000-00" maxLength={18}/>
      </div>
    </div>
  )
}
