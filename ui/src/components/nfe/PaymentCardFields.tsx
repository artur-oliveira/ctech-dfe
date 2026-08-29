'use client'

import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {OptionsSelect} from '@/components/ui/options-select'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {maskCnpj} from '@/lib/utils/masks'
import type {NfeCardIn} from '@/lib/types/api'
import {CARD_PAYMENT_TYPES, isPixPaymentType, TBAND_OPTIONS} from '@/lib/data/payment-tables'

// Bandeiras e classificação dos meios de pagamento vivem na tabela oficial
// (lib/data/payment-tables.ts). Reexportadas aqui porque é onde os formulários
// já as importam.
export const CARD_BAND_OPTIONS = TBAND_OPTIONS

export {CARD_PAYMENT_TYPES, isPixPaymentType}

/**
 * Card / PIX transaction fields (tpIntegra, bandeira, NSU/autorização, CNPJ).
 * Used identically by the NF-e and NFC-e issuance forms.
 */
export function PaymentCardFields({card, onChange, isPix, terminalId, onTerminalChange}: {
  card: NfeCardIn | null
  onChange: (card: NfeCardIn) => void
  isPix: boolean
  /** Terminal de captura que processou o pagamento (organization_payment_terminals). */
  terminalId?: string | null
  onTerminalChange?: (terminalId: string) => void
}) {
  const {selectedOrg} = useAuth()
  const set = (patch: Partial<NfeCardIn>) => onChange({...(card ?? {tp_integra: '2'}), ...patch})

  // O cadastro traz CNPJReceb e idTermPag; aqui só se escolhe qual maquininha.
  const {data: terminalPage} = useQuery({
    queryKey: queryKeys.paymentTerminals.list(selectedOrg?.pk),
    queryFn: () => apiClient.getPaymentTerminals({limit: 100}),
    enabled: !!selectedOrg && !!onTerminalChange,
  })
  const terminals = terminalPage?.items ?? []

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
      {onTerminalChange && terminals.length > 0 && (
        <div className="flex flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">Terminal</Label>
          <OptionsSelect value={terminalId ?? ''} onValueChange={onTerminalChange}
                         placeholder="Nenhum"
                         options={terminals.map((t) => ({
                           value: extractId(t.sk, SK_PREFIX.PAYMENT_TERMINAL),
                           label: t.name,
                         }))}/>
        </div>
      )}
      <div className="flex flex-col gap-1">
        <Label className="text-xs font-medium text-gray-600">Integração</Label>
        <OptionsSelect value={card?.tp_integra ?? '2'}
                       onValueChange={(v) => set({tp_integra: v as '1' | '2'})}
                       options={[{value: '1', label: '1 – Integrado (TEF)'}, {
                         value: '2',
                         label: '2 – Não integrado'
                       }]}/>
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
