'use client'

import {Button} from '@/components/ui/button'
import {CurrencyInput} from '@/components/ui/currency-input'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import {
  AGRO_MODE_OPTIONS,
  AGRO_TP_GUIA_OPTIONS,
  CANA_DIA_OPTIONS,
  canaRefOptions,
  MAX_CANA_DEDUCOES,
  MAX_CANA_DELIVERIES,
  MAX_AGRO_RECEITUARIOS,
  type AgroMode,
} from '@/lib/data/nfe_niche'
import type {NfeAgroGuiaIn, NfeAgroIn, NfeCanaDeducIn, NfeCanaDeliveryIn, NfeCanaIn} from '@/lib/types/api'

export interface NicheGroupsValue {
  /** infNFe/compra — pedido e contrato desta nota. */
  compraXPed: string
  compraXCont: string
  /** infNFe/cana — null quando a nota não é de aquisição de cana. */
  cana: NfeCanaIn | null
  /** infNFe/agropecuario — null quando a nota não é de defensivo nem guia. */
  agro: NfeAgroIn | null
}

export const EMPTY_NICHE_GROUPS: NicheGroupsValue = {
  compraXPed: '', compraXCont: '', cana: null, agro: null,
}

export interface NicheGroupsFieldsProps {
  value: NicheGroupsValue
  onChange: (value: NicheGroupsValue) => void
  /** Safra cadastrada na natureza de operação. Sem ela, cana é recusada. */
  canaSafra?: string | null
  /** CPF do responsável técnico agronômico do emitente. Sem ele, receituário é recusado. */
  technicalManagerCpf?: string | null
}

/** Calculada uma vez por carga do módulo: a lista não muda durante a sessão. */
const CANA_REF_OPTIONS = canaRefOptions()

const EMPTY_DELIVERY: NfeCanaDeliveryIn = {dia: '1', qtde: ''}
const EMPTY_DEDUC: NfeCanaDeducIn = {x_ded: '', v_ded: ''}

/**
 * Grupos de nicho da NF-e (compra, cana e agropecuario), todos opcionais.
 *
 * Nada aqui é campo livre onde existe tabela: o mês de referência e o dia do
 * fornecimento são selects, o tipo de guia e a UF são selects, e a escolha
 * entre receituário e guia é um radio — o XSD é um choice, e um radio não deixa
 * o operador marcar os dois e levar 400 na emissão.
 *
 * Os totais da cana (qTotMes, qTotGer, vTotDed, vLiqFor) não aparecem: são
 * derivados no backend a partir dos lançamentos.
 */
export function NicheGroupsFields({
                                    value, onChange, canaSafra, technicalManagerCpf,
                                  }: NicheGroupsFieldsProps) {
  const patch = (p: Partial<NicheGroupsValue>) => onChange({...value, ...p})

  const cana = value.cana
  const patchCana = (p: Partial<NfeCanaIn>) => {
    if (!cana) return
    patch({cana: {...cana, ...p}})
  }
  const deliveries = cana?.deliveries ?? []
  const deducoes = cana?.deducoes ?? []

  const agro = value.agro
  const agroMode: AgroMode = agro?.guia ? 'guia' : agro ? 'defensivo' : 'none'

  const setAgroMode = (mode: string) => {
    if (mode === 'none') return patch({agro: null})
    if (mode === 'guia') {
      return patch({agro: {guia: {tp_guia: '1', uf_guia: 'PI', serie_guia: '', n_guia: ''}}})
    }
    return patch({agro: {receituarios: ['']}})
  }

  const receituarios = agro?.receituarios ?? []

  return (
    <div className="space-y-5">
      {/* ── compra ─────────────────────────────────────────────────────── */}
      <div className="space-y-2">
        <Label className="text-sm font-medium text-gray-700">Compra pública (pedido e contrato)</Label>
        <p className="text-xs text-gray-500">
          A nota de empenho vem da natureza de operação. Aqui só o que muda por nota.
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <div className="flex flex-col gap-1">
            <Label htmlFor="compra-x-ped" className="text-xs font-medium text-gray-600">Pedido</Label>
            <Input id="compra-x-ped" maxLength={60} value={value.compraXPed} placeholder="PED-4455"
                   className="w-full" onChange={(e) => patch({compraXPed: e.target.value})}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="compra-x-cont" className="text-xs font-medium text-gray-600">Contrato</Label>
            <Input id="compra-x-cont" maxLength={60} value={value.compraXCont} placeholder="CT-2026/09"
                   className="w-full" onChange={(e) => patch({compraXCont: e.target.value})}/>
          </div>
        </div>
      </div>

      {/* ── cana ───────────────────────────────────────────────────────── */}
      <div className="space-y-2 pt-3 border-t border-gray-100">
        <div className="flex items-center gap-2">
          <input type="checkbox" id="toggle-cana" checked={cana !== null} disabled={!canaSafra}
                 className="h-4 w-4 rounded border-gray-300 text-brand-600"
                 onChange={(e) => patch({
                   cana: e.target.checked
                     ? {ref: CANA_REF_OPTIONS[0].value, deliveries: [EMPTY_DELIVERY], deducoes: [], q_tot_ant: ''}
                     : null,
                 })}/>
          <label htmlFor="toggle-cana" className="text-sm font-medium text-gray-700 cursor-pointer">
            Aquisição de cana (cana)
          </label>
        </div>
        {!canaSafra && (
          <p className="text-xs text-gray-500">
            Cadastre a safra na natureza de operação para habilitar este grupo.
          </p>
        )}
        {cana && (
          <div className="space-y-3">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">Safra</Label>
                <Input value={canaSafra ?? ''} readOnly disabled className="w-full"/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="cana-ref" className="text-xs font-medium text-gray-600">Mês de referência</Label>
                <OptionsSelect id="cana-ref" value={cana.ref} options={CANA_REF_OPTIONS}
                               onValueChange={(v) => patchCana({ref: v})}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="cana-qtotant" className="text-xs font-medium text-gray-600">
                  Acumulado anterior
                </Label>
                <NumericInput id="cana-qtotant" decimal decimalPlaces={4} value={cana.q_tot_ant ?? ''}
                              placeholder="0.0000" onChange={(v) => patchCana({q_tot_ant: v})}/>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="text-sm font-medium text-gray-700">Fornecimentos diários</Label>
                <Button type="button" variant="ghost" size="xs"
                        disabled={deliveries.length >= MAX_CANA_DELIVERIES}
                        onClick={() => patchCana({deliveries: [...deliveries, EMPTY_DELIVERY]})}>
                  + Dia
                </Button>
              </div>
              <p className="text-xs text-gray-500">
                Um lançamento por dia. Os totais do mês e o geral são calculados na emissão.
              </p>
              {deliveries.map((dl, i) => (
                <div key={i}
                     className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto] gap-2 items-end">
                  <div className="flex flex-col gap-1">
                    <Label htmlFor={`cana-dia-${i}`} className="text-xs font-medium text-gray-600">Dia</Label>
                    <OptionsSelect id={`cana-dia-${i}`} value={dl.dia} options={CANA_DIA_OPTIONS}
                                   onValueChange={(v) => patchCana({
                                     deliveries: deliveries.map((x, k) => (k === i ? {...x, dia: v} : x)),
                                   })}/>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label htmlFor={`cana-qtde-${i}`} className="text-xs font-medium text-gray-600">
                      Quantidade
                    </Label>
                    <NumericInput id={`cana-qtde-${i}`} decimal decimalPlaces={4} value={dl.qtde}
                                  placeholder="0.0000" onChange={(v) => patchCana({
                      deliveries: deliveries.map((x, k) => (k === i ? {...x, qtde: v} : x)),
                    })}/>
                  </div>
                  <Button type="button" variant="ghost" size="xs" className="min-h-11"
                          onClick={() => patchCana({deliveries: deliveries.filter((_, k) => k !== i)})}>
                    Remover
                  </Button>
                </div>
              ))}
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="text-sm font-medium text-gray-700">Deduções</Label>
                <Button type="button" variant="ghost" size="xs" disabled={deducoes.length >= MAX_CANA_DEDUCOES}
                        onClick={() => patchCana({deducoes: [...deducoes, EMPTY_DEDUC]})}>
                  + Dedução
                </Button>
              </div>
              {deducoes.map((dd, i) => (
                <div key={i}
                     className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto] gap-2 items-end">
                  <div className="flex flex-col gap-1">
                    <Label htmlFor={`cana-xded-${i}`} className="text-xs font-medium text-gray-600">Descrição</Label>
                    <Input id={`cana-xded-${i}`} maxLength={60} value={dd.x_ded} placeholder="CONSECANA"
                           className="w-full" onChange={(e) => patchCana({
                      deducoes: deducoes.map((x, k) => (k === i ? {...x, x_ded: e.target.value} : x)),
                    })}/>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label htmlFor={`cana-vded-${i}`} className="text-xs font-medium text-gray-600">Valor</Label>
                    <CurrencyInput id={`cana-vded-${i}`} value={dd.v_ded} onChange={(v) => patchCana({
                      deducoes: deducoes.map((x, k) => (k === i ? {...x, v_ded: v} : x)),
                    })}/>
                  </div>
                  <Button type="button" variant="ghost" size="xs" className="min-h-11"
                          onClick={() => patchCana({deducoes: deducoes.filter((_, k) => k !== i)})}>
                    Remover
                  </Button>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* ── agropecuario ───────────────────────────────────────────────── */}
      <div className="space-y-2 pt-3 border-t border-gray-100">
        <Label className="text-sm font-medium text-gray-700">Agropecuário (agropecuario)</Label>
        <p className="text-xs text-gray-500">
          Receituário de defensivo e guia de trânsito são alternativos no leiaute — escolha um.
        </p>
        <div className="flex flex-col sm:flex-row gap-2 sm:gap-4">
          {AGRO_MODE_OPTIONS.map((opt) => (
            <label key={opt.value} htmlFor={`agro-mode-${opt.value}`}
                   className="flex items-center gap-2 min-h-11 text-sm text-gray-700 cursor-pointer">
              <input type="radio" id={`agro-mode-${opt.value}`} name="agro-mode" value={opt.value}
                     checked={agroMode === opt.value} onChange={() => setAgroMode(opt.value)}
                     className="h-4 w-4 border-gray-300 text-brand-600"/>
              {opt.label}
            </label>
          ))}
        </div>

        {agroMode === 'defensivo' && (
          <div className="space-y-2">
            {!technicalManagerCpf && (
              <p className="text-xs text-amber-700">
                Cadastre o CPF do responsável técnico agronômico na organização — o leiaute exige um por receituário.
              </p>
            )}
            <div className="flex items-center justify-between">
              <Label className="text-sm font-medium text-gray-700">Receituários</Label>
              <Button type="button" variant="ghost" size="xs"
                      disabled={receituarios.length >= MAX_AGRO_RECEITUARIOS}
                      onClick={() => patch({agro: {receituarios: [...receituarios, '']}})}>
                + Receituário
              </Button>
            </div>
            {receituarios.map((rec, i) => (
              <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_auto] gap-2 items-end">
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`agro-rec-${i}`} className="text-xs font-medium text-gray-600">Número</Label>
                  <Input id={`agro-rec-${i}`} maxLength={30} value={rec} placeholder="REC-2026-001"
                         className="w-full" onChange={(e) => patch({
                    agro: {receituarios: receituarios.map((x, k) => (k === i ? e.target.value : x))},
                  })}/>
                </div>
                <Button type="button" variant="ghost" size="xs" className="min-h-11"
                        onClick={() => patch({agro: {receituarios: receituarios.filter((_, k) => k !== i)}})}>
                  Remover
                </Button>
              </div>
            ))}
          </div>
        )}

        {agroMode === 'guia' && agro?.guia && (() => {
          const guia = agro.guia
          return (
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-2">
            <div className="flex flex-col gap-1">
              <Label htmlFor="agro-tp-guia" className="text-xs font-medium text-gray-600">Tipo</Label>
              <OptionsSelect id="agro-tp-guia" value={guia.tp_guia} options={AGRO_TP_GUIA_OPTIONS}
                             onValueChange={(v) => patch({
                               agro: {guia: {...guia, tp_guia: v as NfeAgroGuiaIn['tp_guia']}},
                             })}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="agro-uf-guia" className="text-xs font-medium text-gray-600">UF</Label>
              <OptionsSelect id="agro-uf-guia" value={guia.uf_guia} options={UF_OPTIONS}
                             onValueChange={(v) => patch({agro: {guia: {...guia, uf_guia: v}}})}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="agro-serie-guia" className="text-xs font-medium text-gray-600">Série</Label>
              <Input id="agro-serie-guia" maxLength={9} value={guia.serie_guia ?? ''} className="w-full"
                     placeholder="Opcional"
                     onChange={(e) => patch({agro: {guia: {...guia, serie_guia: e.target.value}}})}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="agro-n-guia" className="text-xs font-medium text-gray-600">Número</Label>
              <NumericInput id="agro-n-guia" maxLength={9} value={guia.n_guia} placeholder="123456"
                            onChange={(v) => patch({agro: {guia: {...guia, n_guia: v}}})}/>
            </div>
          </div>
          )
        })()}
      </div>
    </div>
  )
}
