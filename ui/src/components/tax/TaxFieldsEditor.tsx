'use client'

import React, {Children, cloneElement, isValidElement, type ReactElement, useId, useState} from 'react'

import {Combobox} from '@/components/ui/combobox'
import {OptionsSelect} from '@/components/ui/options-select'
import {NumericInput} from '@/components/ui/numeric-input'
import {Input} from '@/components/ui/input'
import type {CfopConfigFormData} from '@/lib/schemas/products'
import {getAllCfopOptions} from '@/lib/data/cfop'
import {CITY_OPTIONS} from '@/lib/data/cities'
import {LC116_SERVICE_OPTIONS} from '@/lib/data/nfse_trib_nacional'
import {UNIT_OPTIONS} from '@/lib/data/unit'
import {getCfopHint} from '@/lib/data/cfop_rules'
import {IBS_CBS_CLASS_BY_CST, IBS_CBS_CST_OPTIONS} from '@/lib/data/ibs_cbs_cst'
import {
  ALC_ZFM_TP_CBS_OPTIONS,
  IBS_CBS_C_CRED_PRES_OPTIONS,
  IBS_IND_DOACAO_SIM,
} from '@/lib/data/ibs_cbs_reform'
import {IPI_CST_OPTIONS} from '@/lib/data/ipi'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import {ICMS_MOT_DESONE_OPTIONS, IS_CST_OPTIONS} from '@/lib/data/is'
import {CSOSN_OPTIONS} from '@/lib/data/csosn'
import {ICMS_CST_OPTIONS} from '@/lib/data/icms'
import {PIS_COFINS_OPTIONS} from '@/lib/data/pis_cofins'
import {MOD_BC_OPTIONS, MOD_BC_ST_OPTIONS} from '@/lib/data/mod_bc'
import {useIcmsAliqPreview} from '@/lib/hooks/useIcmsAliqPreview'

// Conjuntos de CST/CSOSN que decidem quais grupos de campos ficam visíveis.
// Ficam aqui, junto do editor que os usa, e são reexportados para o ProductForm
// (que ainda os consulta ao montar a linha de cfop_config).
export const ICMS_MONO_CSTS = new Set(['02', '15', '53', '61'])
export const ICMS_TAXED_CSTS = new Set(['00', '10', '20', '30', '51', '70', '90'])
export const ICMS_ST_CSTS = new Set(['10', '30', '70'])
export const CSOSN_CRED = new Set(['101', '201', '900'])
export const CSOSN_ST = new Set(['201', '202', '203'])
export const PIS_COFINS_ALIQ_CSTS = new Set(['01', '02'])
export const PIS_COFINS_QTDE_CSTS = new Set(['03'])

export const ISSQN_IND_ISS_OPTIONS = [
  {value: '1', label: '1 – Exigível'},
  {value: '2', label: '2 – Não incidente'},
  {value: '3', label: '3 – Isenção'},
  {value: '4', label: '4 – Exportação'},
  {value: '5', label: '5 – Imunidade'},
  {value: '6', label: '6 – Exig. Susp. Judicial'},
  {value: '7', label: '7 – Exig. Susp. Administrativa'},
]

/** Campos condicionais do ICMS por CST — espelha a tabela do leiaute. */
export function icmsConditionalFields(cst: string) {
  return {
    showPRedBC: ['20', '40', '70'].includes(cst),
    showMotDeSon: ['40', '41', '50', '51'].includes(cst),
    showPDif: cst === '51',
  }
}


/**
 * Rótulo ligado ao seu controle. Existe porque o mesmo bloco `label + campo` se
 * repetia ~90 vezes neste arquivo, e a cópia perdeu o `htmlFor` em quase todas:
 * a tela de tratamento tributário, onde um CST errado é nota rejeitada, era a
 * mais opaca do sistema para leitor de tela.
 *
 * O `useId` também resolve a colisão de id quando dois editores convivem na
 * mesma tela (produto e perfil fiscal) — com id literal, clicar num rótulo
 * mexia no campo do outro editor.
 */
function TaxField({label, className, children}: {
  label: React.ReactNode
  className?: string
  children: React.ReactNode
}) {
  const id = useId()
  const items = Children.toArray(children)
  const firstElement = items.findIndex(isValidElement)
  const withId = items.map((child, i) => (
    i === firstElement && isValidElement(child)
      ? cloneElement(child as ReactElement<{id?: string}>, {id})
      : child
  ))
  return (
    <div className={className ? `grid gap-1 ${className}` : 'grid gap-1'}>
      <label htmlFor={id} className="text-sm font-medium text-gray-700">{label}</label>
      {withId}
    </div>
  )
}

export interface TaxFieldsEditorProps {
  /** Linha de tributação em edição. */
  value: CfopConfigFormData
  /** Recebe o updater do react-hook-form/useState do dono do estado. */
  onChange: (updater: (r: CfopConfigFormData) => CfopConfigFormData) => void
  /** Simples Nacional usa CSOSN; Regime Normal usa CST de ICMS. */
  simples: boolean
  /** Oculta o campo CFOP — o perfil fiscal escolhe vários CFOPs à parte. */
  hideCfop?: boolean
  /** Quais grupos opcionais estão habilitados. Fica com quem monta a linha,
   *  porque a validação de "habilitado mas incompleto" é dele. */
  groups: TaxGroups
  onGroupsChange: (next: TaxGroups) => void
  /** UF emitente/destino e NCM — usados só para o warning de alíquota
   *  (consulta GET /v1.0/tax-tables/icms-aliq). Sem eles, o warning não
   *  aparece, mas o campo de override continua funcionando normalmente. */
  emitUf?: string
  destUf?: string
  ncm?: string
}

/** Grupos tributários opcionais que o editor revela sob demanda. */
export interface TaxGroups {
  ipi: boolean
  is: boolean
  ibsCbs: boolean
  ibsRed: boolean
  ibsDif: boolean
  issqn: boolean
  icmsMono: boolean
  pisCofinsSt: boolean
  /** Monofasia do IBS/CBS: retenção, já retido e diferimento (gIBSCBSMono). */
  ibsMono: boolean
  /** Tributação de referência e de compra governamental. */
  ibsRef: boolean
  /** Créditos presumidos (operação, ZFM) e alíquota zero da CBS em ALC/ZFM. */
  ibsCred: boolean
}

export const EMPTY_TAX_GROUPS: TaxGroups = {
  ipi: false, is: false, ibsCbs: false, ibsRed: false, ibsDif: false, issqn: false,
  icmsMono: false, pisCofinsSt: false, ibsMono: false, ibsRef: false, ibsCred: false,
}

/**
 * Deriva quais grupos opcionais já têm dado preenchido — usado ao abrir o editor com
 * uma linha existente, senão o toggle nasce desligado escondendo valores já salvos.
 */
export const deriveTaxGroups = (data: Partial<CfopConfigFormData>): TaxGroups => ({
  ipi: !!data.ipi_cst,
  is: !!data.is_cst,
  ibsCbs: !!data.ibs_cbs_cst,
  ibsRed: !!(data.ibs_uf_p_red || data.ibs_mun_p_red || data.cbs_p_red),
  ibsDif: !!(data.ibs_uf_p_dif || data.ibs_mun_p_dif || data.cbs_p_dif),
  issqn: !!data.issqn_ind_iss,
  icmsMono: !!data.icms_ad_rem,
  pisCofinsSt: !!(data.pis_st_aliq || data.cofins_st_aliq || data.pis_st_v_bc || data.cofins_st_v_bc),
  ibsMono: !!(data.ibs_ad_rem || data.cbs_ad_rem || data.ibs_ad_rem_reten || data.cbs_ad_rem_reten
    || data.ibs_ad_rem_ret || data.cbs_ad_rem_ret || data.ibs_p_dif_mono || data.cbs_p_dif_mono),
  ibsRef: !!(data.ibs_reg_cst || data.ibs_gov_uf_aliq || data.ibs_gov_mun_aliq || data.cbs_gov_aliq),
  ibsCred: !!(data.ibs_cbs_c_cred_pres || data.ibs_zfm_p_cred_pres || data.alc_zfm_tp_cbs),
})

/** icms_mod_bc cujo cálculo usa um valor fixo em vez do valor de venda. */
const ICMS_MOD_BC_PAUTA = new Set(['1', '2'])

/** A tabela de CFOP é estática: recriar o array a cada render invalidava o memo
 *  do Combobox e refazia o filtro sobre a lista inteira. */
const CFOP_OPTIONS = getAllCfopOptions()

/**
 * Editor de tributação — ICMS/CSOSN, ST, PIS, COFINS, IBS/CBS, IPI, IS e ISSQN.
 *
 * É o mesmo bloco em dois lugares: dentro do produto (preso a um CFOP, em
 * `cfop_config[]`) e dentro do perfil fiscal (o mesmo tratamento nomeado uma vez
 * e reutilizado por vários produtos). Duas cópias desses ~60 campos seriam
 * exatamente a duplicação que os perfis existem para eliminar.
 *
 * Os toggles de grupos opcionais são estado interno: quem usa reseta remontando
 * o componente (`key`), como o ProductForm faz ao adicionar um CFOP à lista.
 */
export function TaxFieldsEditor({
  value, onChange, simples, hideCfop = false, groups, onGroupsChange, emitUf, destUf, ncm,
}: TaxFieldsEditorProps) {
  const {ipi: showIpi, is: showIs, ibsCbs: showIbsCbs, ibsRed: showIbsCbsRed,
    ibsDif: showIbsCbsDif, issqn: showIssqn, icmsMono: showIcmsMono,
    pisCofinsSt: showPisCofinsSt, ibsMono: showIbsMono, ibsRef: showIbsRef,
    ibsCred: showIbsCred} = groups
  const setGroup = (key: keyof TaxGroups) => (on: boolean) => onGroupsChange({...groups, [key]: on})
  const setShowIpi = setGroup('ipi')
  const setShowIs = setGroup('is')
  const setShowIbsCbs = setGroup('ibsCbs')
  const setShowIbsCbsRed = setGroup('ibsRed')
  const setShowIbsCbsDif = setGroup('ibsDif')
  const setShowIssqn = setGroup('issqn')
  const setShowIcmsMono = setGroup('icmsMono')
  const setShowPisCofinsSt = setGroup('pisCofinsSt')
  const setShowIbsMono = setGroup('ibsMono')
  const setShowIbsRef = setGroup('ibsRef')
  const setShowIbsCred = setGroup('ibsCred')

  // Prefixo dos ids dos toggles: dois editores na mesma tela (produto e perfil)
  // colidiam, e clicar num rótulo mexia no grupo do outro editor.
  const uid = useId()
  // Quantos grupos opcionais já têm dado — abre sozinho quando há, senão o
  // operador salva sem ver o que está configurado.
  const activeGroupCount = Object.values(groups).filter(Boolean).length
  const [showOtherTaxes, setShowOtherTaxes] = useState(activeGroupCount > 0)
  const systemAliq = useIcmsAliqPreview(emitUf, destUf, ncm)
  const aliqDiverges = !!systemAliq && !!value.icms_aliq_override &&
    value.icms_aliq_override !== systemAliq.icms_aliq

  const {showPRedBC, showMotDeSon, showPDif} = icmsConditionalFields(value.icms ?? '')
  const cfopHint = getCfopHint(value.cfop)
  const showSt = (!simples && !!value.icms && ICMS_ST_CSTS.has(value.icms)) ||
    (simples && !!value.csosn && ICMS_ST_CSTS.has(value.csosn))
  // ST já retida + ICMS efetivo: revenda de mercadoria com ST (CST 41/60,
  // CSOSN 500). É o mesmo grupo de campos nos três casos.
  const showStRet = (!simples && ['41', '60'].includes(value.icms ?? '')) ||
    (simples && value.csosn === '500')
  // Partilha do ICMS (ICMSPart): não tem CST próprio, o par pBCOp+UFST é que
  // troca ICMS10/ICMS90 pelo grupo.
  const showPart = !simples && ['10', '90'].includes(value.icms ?? '')

  return (
    <div className="space-y-5">
      {/* ── Linha obrigatória ───────────────────────────────────── */}
      <div className="space-y-3 rounded-lg border border-gray-100 p-3 bg-gray-50/50">
        <p className="text-sm font-medium text-gray-600">Dados da operação</p>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {!hideCfop && (
            <TaxField label="CFOP *">
              <Combobox value={value.cfop}
                        onValueChange={(v) => onChange((r) => ({...r, cfop: v}))}
                        options={CFOP_OPTIONS} placeholder="CFOP" searchPlaceholder="Código ou descrição..."/>
            </TaxField>
          )}
          <TaxField label={simples ? 'CSOSN *' : 'ICMS CST *'}>
            {simples ? (
              <Combobox value={value.csosn ?? ''}
                             onValueChange={(v) => onChange((r) => ({...r, csosn: v}))}
                             options={CSOSN_OPTIONS} placeholder="CSOSN"/>
            ) : (
              <Combobox value={value.icms ?? ''}
                             onValueChange={(v) => onChange((r) => ({
                               ...r, icms: v,
                               icms_p_red_bc: '', icms_mot_des: '', icms_p_dif: '',
                               icms_aliq_override: '', icms_fcp_override: '',
                               icms_st_mva: '', icms_st_aliq: '', icms_st_fcp_aliq: '',
                             }))}
                             options={ICMS_CST_OPTIONS} placeholder="CST"/>
            )}
          </TaxField>
          <TaxField label="PIS *">
            <OptionsSelect value={value.pis}
                           onValueChange={(v) => onChange((r) => ({
                             ...r,
                             pis: v,
                             pis_aliq: '',
                             pis_aliq_unid: ''
                           }))}
                           options={PIS_COFINS_OPTIONS} placeholder="PIS"/>
          </TaxField>
          <TaxField label="COFINS *">
            <OptionsSelect value={value.cofins}
                           onValueChange={(v) => onChange((r) => ({
                             ...r,
                             cofins: v,
                             cofins_aliq: '',
                             cofins_aliq_unid: ''
                           }))}
                           options={PIS_COFINS_OPTIONS} placeholder="COFINS"/>
          </TaxField>
        </div>

        {/* Hint fiscal CFOP */}
        {!hideCfop && cfopHint && (
          <div
            className="col-span-full flex items-center gap-2 rounded-lg bg-amber-50 border border-amber-200 px-3 py-2 text-sm text-warning">
            <span className="font-medium">!</span>
            <span>{cfopHint.label}</span>
          </div>
        )}

        {/* Campos condicionais ICMS Regime Normal */}
        {!simples && (showPRedBC || showMotDeSon || showPDif) && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-2 border-t border-gray-200">
            {showPRedBC && (
              <TaxField label="% Redução BC">
                <NumericInput value={value.icms_p_red_bc ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_red_bc: v}))}/>
              </TaxField>
            )}
            {showMotDeSon && (
              <TaxField label="Motivo desoneração *">
                <OptionsSelect value={value.icms_mot_des ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, icms_mot_des: v}))}
                               options={ICMS_MOT_DESONE_OPTIONS} placeholder="Motivo"/>
              </TaxField>
            )}
            {showPDif && (
              <TaxField label="% Diferimento do FCP">
                <NumericInput value={value.icms_p_fcp_dif ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_fcp_dif: v}))}/>
              </TaxField>
            )}
            {showPDif && (
              <TaxField label="% Diferimento">
                <NumericInput value={value.icms_p_dif ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_dif: v}))}/>
              </TaxField>
            )}
          </div>
        )}

        {/* Alíquota ICMS — para CSTs tributados */}
        {!simples && value.icms && ICMS_TAXED_CSTS.has(value.icms) && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-2 border-t border-gray-200">
            <TaxField label="Alíquota ICMS %">
              <NumericInput value={value.icms_aliq_override ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                            placeholder="Padrão: tabela da UF"
                            onChange={(v) => onChange((r) => ({...r, icms_aliq_override: v}))}/>
              <p className="text-xs text-gray-400">
                {systemAliq ? `Vazio ou igual = usa a alíquota do sistema (${systemAliq.icms_aliq}%)`
                  : 'Vazio = usa alíquota padrão da UF de destino'}
              </p>
            </TaxField>
            {['00', '10', '20', '51', '70', '90'].includes(value.icms) && (
              <TaxField label="Modo de cálculo">
                <OptionsSelect value={value.icms_mod_bc ?? '3'}
                               onValueChange={(v) => onChange((r) => ({...r, icms_mod_bc: v}))}
                               options={MOD_BC_OPTIONS} placeholder="Modo de cálculo"/>
              </TaxField>
            )}
            <TaxField label="FCP %">
              <NumericInput value={value.icms_fcp_override ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                            placeholder="Padrão: tabela da UF"
                            onChange={(v) => onChange((r) => ({...r, icms_fcp_override: v}))}/>
            </TaxField>
            {ICMS_MOD_BC_PAUTA.has(value.icms_mod_bc ?? '') && (
              <TaxField label="Valor da pauta fiscal (R$)">
                <NumericInput value={value.icms_pauta_valor ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, icms_pauta_valor: v}))}/>
              </TaxField>
            )}
            {aliqDiverges && (
              <div role="alert"
                   className="col-span-full rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-warning">
                Alíquota ICMS digitada ({value.icms_aliq_override}%) diverge da tabela do sistema
                para esta UF/NCM ({systemAliq?.icms_aliq}%).
              </div>
            )}
          </div>
        )}

        {/* pCredSN — Simples Nacional com crédito */}
        {simples && value.csosn && CSOSN_CRED.has(value.csosn) && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-2 border-t border-gray-200">
            <TaxField label="% Crédito aproveitável">
              <NumericInput value={value.icms_sn_cred_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                            placeholder="Ex: 4.0000"
                            onChange={(v) => onChange((r) => ({...r, icms_sn_cred_aliq: v}))}/>
            </TaxField>
          </div>
        )}

        {/* ST — Substituição Tributária */}
        {showSt && (
          <div className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 space-y-2">
            <p className="text-sm font-medium text-blue-700">
              Substituição Tributária (ST)
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <TaxField label="Cálculo da BC ST">
                <OptionsSelect value={value.icms_st_mod_bc ?? '4'}
                               onValueChange={(v) => onChange((r) => ({...r, icms_st_mod_bc: v}))}
                               options={MOD_BC_ST_OPTIONS} placeholder="Modo"/>
              </TaxField>
              {(!value.icms_st_mod_bc || value.icms_st_mod_bc === '4') && (
                <TaxField label="MVA %">
                  <NumericInput value={value.icms_st_mva ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                                placeholder="Ex: 30.0000"
                                onChange={(v) => onChange((r) => ({...r, icms_st_mva: v}))}/>
                </TaxField>
              )}
              <TaxField label="Alíquota ST %">
                <NumericInput value={value.icms_st_aliq ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="Ex: 18.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_st_aliq: v}))}/>
              </TaxField>
              <TaxField label="FCP ST %">
                <NumericInput value={value.icms_st_fcp_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_st_fcp_aliq: v}))}/>
              </TaxField>
              <TaxField label="Motivo desoneração da ST">
                <OptionsSelect value={value.icms_mot_des_st ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, icms_mot_des_st: v}))}
                               options={ICMS_MOT_DESONE_OPTIONS} placeholder="Não desonerada"/>
              </TaxField>
              <TaxField label="% Redução BC ST">
                <NumericInput value={value.icms_st_red_bc ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_st_red_bc: v}))}/>
              </TaxField>
            </div>
          </div>
        )}

        {/* ST retida anteriormente + ICMS efetivo */}
        {showStRet && (
          <div className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 space-y-2">
            <p className="text-sm font-medium text-blue-700">
              ST retida anteriormente
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <TaxField label="BC da ST retida">
                <NumericInput value={value.icms_v_bc_st_ret ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, icms_v_bc_st_ret: v}))}/>
              </TaxField>
              <TaxField label="ICMS-ST retido">
                <NumericInput value={value.icms_v_icms_st_ret ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, icms_v_icms_st_ret: v}))}/>
              </TaxField>
              <TaxField label="Alíquota suportada %">
                <NumericInput value={value.icms_p_st ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_st: v}))}/>
              </TaxField>
              {value.icms === '41' && (
                <>
                  <TaxField label="BC da ST na UF de destino">
                    <NumericInput value={value.icms_v_bc_st_dest ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                                  placeholder="0.00"
                                  onChange={(v) => onChange((r) => ({...r, icms_v_bc_st_dest: v}))}/>
                  </TaxField>
                  <TaxField label="ICMS-ST da UF de destino">
                    <NumericInput value={value.icms_v_icms_st_dest ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                                  placeholder="0.00"
                                  onChange={(v) => onChange((r) => ({...r, icms_v_icms_st_dest: v}))}/>
                  </TaxField>
                </>
              )}
              <TaxField label="% Redução BC efetiva">
                <NumericInput value={value.icms_p_red_bc_efet ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_red_bc_efet: v}))}/>
              </TaxField>
              <TaxField label="Alíquota efetiva %">
                <NumericInput value={value.icms_p_icms_efet ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_icms_efet: v}))}/>
              </TaxField>
            </div>
            <p className="text-xs text-gray-500">
              A base e o valor do ICMS efetivo são calculados na emissão — informe só os percentuais.
            </p>
          </div>
        )}

        {/* Partilha do ICMS entre UFs (ICMSPart) */}
        {showPart && (
          <div className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 space-y-2">
            <p className="text-sm font-medium text-blue-700">
              Partilha do ICMS entre UFs
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <TaxField label="% da BC na origem">
                <NumericInput value={value.icms_part_p_bc_op ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_part_p_bc_op: v}))}/>
              </TaxField>
              <TaxField label="UF do ST">
                <OptionsSelect value={value.icms_part_uf_st ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, icms_part_uf_st: v}))}
                               options={UF_OPTIONS} placeholder="UF"/>
              </TaxField>
            </div>
            <p className="text-xs text-gray-500">
              Preencha os dois para emitir o grupo ICMSPart no lugar de ICMS{value.icms}.
            </p>
          </div>
        )}

        {/* Alíquotas PIS/COFINS condicionais */}
        {((value.pis && (PIS_COFINS_ALIQ_CSTS.has(value.pis) || PIS_COFINS_QTDE_CSTS.has(value.pis))) ||
          (value.cofins && (PIS_COFINS_ALIQ_CSTS.has(value.cofins) || PIS_COFINS_QTDE_CSTS.has(value.cofins)))) && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 pt-2 border-t border-gray-200">
            {value.pis && PIS_COFINS_ALIQ_CSTS.has(value.pis) && (
              <TaxField label="Alíquota PIS %">
                <NumericInput value={value.pis_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="Ex: 0.6500"
                              onChange={(v) => onChange((r) => ({...r, pis_aliq: v}))}/>
              </TaxField>
            )}
            {value.pis && PIS_COFINS_QTDE_CSTS.has(value.pis) && (
              <TaxField label="PIS R$/unid">
                <NumericInput value={value.pis_aliq_unid ?? ''} decimal integerPlaces={4} decimalPlaces={4}
                              placeholder="Ex: 0.0065"
                              onChange={(v) => onChange((r) => ({...r, pis_aliq_unid: v}))}/>
              </TaxField>
            )}
            {value.cofins && PIS_COFINS_ALIQ_CSTS.has(value.cofins) && (
              <TaxField label="Alíquota COFINS %">
                <NumericInput value={value.cofins_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="Ex: 3.0000"
                              onChange={(v) => onChange((r) => ({...r, cofins_aliq: v}))}/>
              </TaxField>
            )}
            {value.cofins && PIS_COFINS_QTDE_CSTS.has(value.cofins) && (
              <TaxField label="COFINS R$/unid">
                <NumericInput value={value.cofins_aliq_unid ?? ''} decimal integerPlaces={4} decimalPlaces={4}
                              placeholder="Ex: 0.0300"
                              onChange={(v) => onChange((r) => ({...r, cofins_aliq_unid: v}))}/>
              </TaxField>
            )}
          </div>
        )}
      </div>

      {/* Visão avançada: os grupos que a maioria das operações não usa ficam
          atrás de uma dobra. O contador diz que há algo configurado lá dentro,
          para fechado não virar escondido. */}
      <div className="rounded-lg border border-gray-200">
        <button type="button" onClick={() => setShowOtherTaxes((v) => !v)}
                aria-expanded={showOtherTaxes} aria-controls={`${uid}-outros-impostos`}
                className="flex min-h-11 w-full items-center justify-between gap-2 px-3 py-2 text-left">
          <span className="flex items-center gap-2">
            <span className="text-sm font-medium text-gray-700">Outros impostos e regimes</span>
            {activeGroupCount > 0 && (
              <span aria-label={`${activeGroupCount} grupo(s) configurado(s)`}
                    className="rounded-full bg-brand-100 px-1.5 py-0.5 text-xs font-semibold text-brand-700">
                {activeGroupCount}
              </span>
            )}
          </span>
          <span aria-hidden="true" className="text-sm text-gray-400">{showOtherTaxes ? "−" : "+"}</span>
        </button>
        {!showOtherTaxes && (
          <p className="px-3 pb-2 text-xs text-gray-500">
            IPI, IS, ISSQN, IBS/CBS, monofásico e PIS/COFINS-ST.
          </p>
        )}
        {showOtherTaxes && (
          <div id={`${uid}-outros-impostos`} className="space-y-5 border-t border-gray-100 p-3">
        {/* ── PIS/COFINS-ST ───────────────────────────────────────── */}
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-pis-cofins-st`} checked={showPisCofinsSt}
                   onChange={(e) => {
                     setShowPisCofinsSt(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, pis_st_aliq: '', cofins_st_aliq: '', pis_st_v_bc: '', cofins_st_v_bc: '',
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-pis-cofins-st`}
                   className="flex min-h-11 items-center text-sm font-medium text-gray-600 cursor-pointer select-none sm:min-h-0">
              PIS/COFINS-ST — Substituição Tributária
            </label>
          </div>
          {showPisCofinsSt && (
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              <TaxField label="Alíquota PIS-ST %">
                <NumericInput value={value.pis_st_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, pis_st_aliq: v}))}/>
              </TaxField>
              <TaxField label="Alíquota COFINS-ST %">
                <NumericInput value={value.cofins_st_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, cofins_st_aliq: v}))}/>
              </TaxField>
              <TaxField label="BC PIS-ST (R$)">
                <NumericInput value={value.pis_st_v_bc ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, pis_st_v_bc: v}))}/>
              </TaxField>
              <TaxField label="BC COFINS-ST (R$)">
                <NumericInput value={value.cofins_st_v_bc ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, cofins_st_v_bc: v}))}/>
              </TaxField>
            </div>
          )}
        </div>

        {/* ── IPI ─────────────────────────────────────────────────── */}
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ipi`} checked={showIpi}
                   onChange={(e) => {
                     setShowIpi(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({...r, ipi_cst: '', ipi_aliq: ''}))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ipi`}
                   className="flex min-h-11 items-center text-sm font-medium text-gray-600 cursor-pointer select-none sm:min-h-0">
              IPI — Imposto sobre Produtos Industrializados
            </label>
          </div>
          {showIpi && (
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 sm:max-w-sm">
              <TaxField label="CST IPI *">
                <Combobox value={value.ipi_cst ?? ''}
                          onValueChange={(v) => onChange((r) => ({...r, ipi_cst: v}))}
                          options={IPI_CST_OPTIONS} placeholder="CST"
                          searchPlaceholder="Código ou descrição..."/>
              </TaxField>
              <TaxField label="Alíquota %">
                <NumericInput value={value.ipi_aliq ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000" disabled={!!value.ipi_v_unid}
                              onChange={(v) => onChange((r) => ({...r, ipi_aliq: v}))}/>
              </TaxField>
              <div className="grid gap-1 col-span-2">
                <label className="text-sm font-medium text-gray-700">Valor por unidade (R$)</label>
                <NumericInput value={value.ipi_v_unid ?? ''} decimal integerPlaces={13} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, ipi_v_unid: v}))}/>
                <p className="text-xs text-gray-500">
                  Bebidas e cigarros recolhem IPI por unidade. Preenchido, substitui a alíquota — os dois
                  modos são exclusivos no leiaute.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* ── IS ──────────────────────────────────────────────────── */}
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-is`} checked={showIs}
                   onChange={(e) => {
                     setShowIs(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, is_cst: '', is_aliq: '', is_class_trib: '', is_aliq_espec: '', is_unid_trib: ''
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-is`}
                   className="flex min-h-11 items-center text-sm font-medium text-gray-600 cursor-pointer select-none sm:min-h-0">
              IS — Imposto Seletivo (NT 2024.001)
            </label>
          </div>
          {showIs && (
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <TaxField label="CST IS *">
                <Combobox value={value.is_cst ?? ''}
                          onValueChange={(v) => onChange((r) => ({...r, is_cst: v}))}
                          options={IS_CST_OPTIONS} placeholder="CST"
                          searchPlaceholder="Código ou descrição..."/>
              </TaxField>
              <TaxField label="Alíquota %">
                <NumericInput value={value.is_aliq ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, is_aliq: v}))}/>
              </TaxField>
              <TaxField label="Classificação IS">
                <NumericInput value={value.is_class_trib ?? ''}
                              placeholder="000000" maxLength={6}
                              onChange={(v) => onChange((r) => ({...r, is_class_trib: v}))}/>
              </TaxField>
              <TaxField label="Alíquota específica">
                <NumericInput value={value.is_aliq_espec ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, is_aliq_espec: v}))}/>
              </TaxField>
              <TaxField label="Unid. tributável IS">
                <Combobox value={value.is_unid_trib ?? ''} options={UNIT_OPTIONS}
                          onValueChange={(v) => onChange((r) => ({...r, is_unid_trib: v}))}
                          placeholder="Unidade" searchPlaceholder="Código ou descrição..."/>
              </TaxField>
            </div>
          )}
        </div>

        {/* ── ICMS Monofásico — Combustíveis (CST 02/15/53/61) ── */}
        {!simples && (
          <div className="rounded-lg border border-gray-100 p-3 space-y-3">
            <div className="flex items-center gap-2">
              <input type="checkbox" id={`${uid}-toggle-mono`} checked={showIcmsMono}
                     onChange={(e) => {
                       setShowIcmsMono(e.target.checked)
                       if (!e.target.checked) onChange((r) => ({
                         ...r, icms_ad_rem: '', icms_ad_rem_reten: '',
                         icms_p_red_ad_rem: '', icms_mot_red_ad_rem: '', icms_p_dif_mono: '',
                       }))
                     }}
                     className="size-4 rounded border-gray-300 text-brand-600"/>
              <label htmlFor={`${uid}-toggle-mono`}
                     className="flex min-h-11 items-center text-sm font-medium text-gray-600 cursor-pointer select-none sm:min-h-0">
                ICMS Monofásico — Combustíveis (CST 02/15/53/61)
              </label>
            </div>
            {(showIcmsMono || ICMS_MONO_CSTS.has(value.icms ?? '')) && (
              <div className="space-y-2">
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                  <TaxField label="Ad rem ICMS (R$/un) *">
                    <NumericInput value={value.icms_ad_rem ?? ''} decimal integerPlaces={4} decimalPlaces={4}
                                  placeholder="Ex: 1.5000"
                                  onChange={(v) => onChange((r) => ({...r, icms_ad_rem: v}))}/>
                  </TaxField>
                  {value.icms === '53' && (
                    <TaxField label="% Diferimento (53)">
                      <NumericInput value={value.icms_p_dif_mono ?? ''} decimal integerPlaces={3}
                                    decimalPlaces={4}
                                    placeholder="0.0000"
                                    onChange={(v) => onChange((r) => ({...r, icms_p_dif_mono: v}))}/>
                    </TaxField>
                  )}
                  {value.icms === '15' && (
                    <>
                      <TaxField label="Ad rem retenção (15)">
                        <NumericInput value={value.icms_ad_rem_reten ?? ''} decimal integerPlaces={4}
                                      decimalPlaces={4}
                                      placeholder="0.0000"
                                      onChange={(v) => onChange((r) => ({...r, icms_ad_rem_reten: v}))}/>
                      </TaxField>
                      <TaxField label="% Redução ad rem">
                        <NumericInput value={value.icms_p_red_ad_rem ?? ''} decimal integerPlaces={3}
                                      decimalPlaces={4}
                                      placeholder="0.0000"
                                      onChange={(v) => onChange((r) => ({...r, icms_p_red_ad_rem: v}))}/>
                      </TaxField>
                      <TaxField label="Motivo redução">
                        <OptionsSelect value={value.icms_mot_red_ad_rem ?? ''}
                                       onValueChange={(v) => onChange((r) => ({...r, icms_mot_red_ad_rem: v}))}
                                       options={[{value: '1', label: '1 – Transporte coletivo'}, {
                                         value: '9',
                                         label: '9 – Outros'
                                       }]}
                                       placeholder="Motivo"/>
                      </TaxField>
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {/* ── ISSQN — Imposto Sobre Serviços ──────────────────── */}
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-issqn`} checked={showIssqn}
                   onChange={(e) => {
                     setShowIssqn(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, issqn_ind_iss: '', issqn_c_list_serv: '',
                       issqn_c_mun_fg: '', issqn_aliq: '', issqn_v_deducao: '', issqn_v_iss_ret: '',
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-issqn`}
                   className="flex min-h-11 items-center text-sm font-medium text-gray-600 cursor-pointer select-none sm:min-h-0">
              ISSQN — Imposto Sobre Serviços (LC 116/2003)
            </label>
          </div>
          {showIssqn && (
            <div className="space-y-2">
              <div className="rounded-sm border border-blue-100 bg-blue-50/20 px-3 py-1.5 text-xs text-blue-700">
                Quando habilitado, o item usa ISSQN no lugar de ICMS no XML da NF-e.
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                <TaxField label="Exigibilidade ISS *">
                  <OptionsSelect value={value.issqn_ind_iss ?? ''}
                                 onValueChange={(v) => onChange((r) => ({...r, issqn_ind_iss: v}))}
                                 options={ISSQN_IND_ISS_OPTIONS} placeholder="Exigibilidade"/>
                </TaxField>
                <TaxField label="Serviço prestado (LC 116)">
                  <Combobox value={value.issqn_c_list_serv ?? ''} options={LC116_SERVICE_OPTIONS}
                            onValueChange={(v) => onChange((r) => ({...r, issqn_c_list_serv: v}))}
                            placeholder="Item da lista" searchPlaceholder="Código ou descrição..." fuzzySearch/>
                </TaxField>
                <TaxField label="Município do fato gerador">
                  <Combobox value={value.issqn_c_mun_fg ?? ''} options={CITY_OPTIONS}
                            onValueChange={(v) => onChange((r) => ({...r, issqn_c_mun_fg: v}))}
                            placeholder="Município" searchPlaceholder="Nome ou UF..." fuzzySearch/>
                </TaxField>
                <TaxField label="Alíquota ISSQN %">
                  <NumericInput value={value.issqn_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                                placeholder="5.0000"
                                onChange={(v) => onChange((r) => ({...r, issqn_aliq: v}))}/>
                </TaxField>
                <TaxField label="Dedução R$">
                  <NumericInput value={value.issqn_v_deducao ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                                placeholder="0.00"
                                onChange={(v) => onChange((r) => ({...r, issqn_v_deducao: v}))}/>
                </TaxField>
                <TaxField label="Retenção ISS R$">
                  <NumericInput value={value.issqn_v_iss_ret ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                                placeholder="0.00"
                                onChange={(v) => onChange((r) => ({...r, issqn_v_iss_ret: v}))}/>
                </TaxField>
                <TaxField label="Outras retenções R$">
                  <NumericInput value={value.issqn_v_outro ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                                placeholder="0.00"
                                onChange={(v) => onChange((r) => ({...r, issqn_v_outro: v}))}/>
                </TaxField>
                <TaxField label="Desconto incondicional R$">
                  <NumericInput value={value.issqn_v_desc_incond ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                                placeholder="0.00"
                                onChange={(v) => onChange((r) => ({...r, issqn_v_desc_incond: v}))}/>
                </TaxField>
                <TaxField label="Desconto condicional R$">
                  <NumericInput value={value.issqn_v_desc_cond ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                                placeholder="0.00"
                                onChange={(v) => onChange((r) => ({...r, issqn_v_desc_cond: v}))}/>
                </TaxField>
                <TaxField label="Código do serviço no município">
                  <Input value={value.issqn_c_servico ?? ''} maxLength={20}
                         onChange={(e) => onChange((r) => ({...r, issqn_c_servico: e.target.value}))}/>
                </TaxField>
                <TaxField label="Município de incidência">
                  <Combobox value={value.issqn_c_mun ?? ''} options={CITY_OPTIONS}
                            onValueChange={(v) => onChange((r) => ({...r, issqn_c_mun: v}))}
                            placeholder="Município" searchPlaceholder="Nome ou UF..." fuzzySearch/>
                </TaxField>
                <TaxField label="País do serviço">
                  <Input value={value.issqn_c_pais ?? ''} maxLength={4} inputMode="numeric" placeholder="1058"
                         onChange={(e) => onChange((r) => ({...r, issqn_c_pais: e.target.value.replace(/\D/g, '')}))}/>
                </TaxField>
                <TaxField label="Nº do processo">
                  <Input value={value.issqn_n_processo ?? ''} maxLength={30}
                         onChange={(e) => onChange((r) => ({...r, issqn_n_processo: e.target.value}))}/>
                </TaxField>
                <TaxField label="Incentivo fiscal">
                  <OptionsSelect value={value.issqn_ind_incentivo ?? ''}
                                 onValueChange={(v) => onChange((r) => ({...r, issqn_ind_incentivo: v}))}
                                 placeholder="Não informado"
                                 options={[{value: '1', label: '1 – Sim'}, {value: '2', label: '2 – Não'}]}/>
                </TaxField>
              </div>
            </div>
          )}
        </div>

        {/* ── Observação fiscal do item (obsItem) ─────────────────── */}
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <p className="text-sm font-medium text-gray-600">
            Observação fiscal do item
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <TaxField label="Campo">
              <Input value={value.obs_item_x_campo ?? ''} maxLength={20} placeholder="Ex: Beneficio"
                     onChange={(e) => onChange((r) => ({...r, obs_item_x_campo: e.target.value}))}/>
            </TaxField>
            <TaxField label="Texto">
              <Input value={value.obs_item_x_texto ?? ''} maxLength={60}
                     onChange={(e) => onChange((r) => ({...r, obs_item_x_texto: e.target.value}))}/>
            </TaxField>
          </div>
        </div>

        {/* ── IBS / CBS ───────────────────────────────────────────── */}
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ibs-cbs`} checked={showIbsCbs}
                   onChange={(e) => {
                     setShowIbsCbs(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
                       ibs_uf_p_red: '', ibs_mun_p_red: '', cbs_p_red: '',
                       ibs_uf_p_dif: '', ibs_mun_p_dif: '', cbs_p_dif: '',
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ibs-cbs`}
                   className="flex min-h-11 items-center text-sm font-medium text-gray-600 cursor-pointer select-none sm:min-h-0">
              IBS / CBS — Reforma Tributária
            </label>
          </div>
          {showIbsCbs && (
          <>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
            <TaxField label="CST">
              <Combobox value={value.ibs_cbs_cst ?? ''}
                        onValueChange={(v) => onChange((r) => ({
                          ...r,
                          ibs_cbs_cst: v,
                          ibs_cbs_class_trib: IBS_CBS_CLASS_BY_CST[v]?.[0]?.value ?? ''
                        }))}
                        options={IBS_CBS_CST_OPTIONS} placeholder="CST"
                        searchPlaceholder="Código ou descrição..."/>
            </TaxField>
            <TaxField label="Classificação">
              <Combobox value={value.ibs_cbs_class_trib ?? ''}
                        onValueChange={(v) => onChange((r) => ({...r, ibs_cbs_class_trib: v}))}
                        options={IBS_CBS_CLASS_BY_CST[value.ibs_cbs_cst ?? ''] ?? []} placeholder="Código"
                        searchPlaceholder="Código ou descrição..."/>
            </TaxField>
            <TaxField label="IBS UF %">
              <NumericInput decimal decimalPlaces={4} integerPlaces={3} value={value.ibs_uf_aliq ?? ''}
                            onChange={(v) => onChange((r) => ({...r, ibs_uf_aliq: v}))} placeholder="0.0000"/>
            </TaxField>
            <TaxField label="IBS Mun %">
              <NumericInput decimal decimalPlaces={4} integerPlaces={3} value={value.ibs_mun_aliq ?? ''}
                            onChange={(v) => onChange((r) => ({...r, ibs_mun_aliq: v}))} placeholder="0.0000"/>
            </TaxField>
            <TaxField label="CBS %">
              <NumericInput decimal decimalPlaces={4} integerPlaces={3} value={value.cbs_aliq ?? ''}
                            onChange={(v) => onChange((r) => ({...r, cbs_aliq: v}))} placeholder="0.0000"/>
            </TaxField>
          </div>

          {/* Toggle redução */}
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ibs-red`} checked={showIbsCbsRed}
                   onChange={(e) => {
                     setShowIbsCbsRed(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, ibs_uf_p_red: '', ibs_mun_p_red: '', cbs_p_red: ''
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ibs-red`} className="text-xs font-medium text-gray-500 cursor-pointer">
              Redução de alíquota (CST 010/011)
            </label>
          </div>
          {showIbsCbsRed && (
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
              <TaxField label="% Redução IBS UF">
                <NumericInput value={value.ibs_uf_p_red ?? ''} decimal decimalPlaces={4}
                              onChange={(v) => onChange((r) => ({...r, ibs_uf_p_red: v}))} placeholder="0.0000"/>
              </TaxField>
              <TaxField label="% Redução IBS Mun">
                <NumericInput value={value.ibs_mun_p_red ?? ''} decimal decimalPlaces={4}
                              onChange={(v) => onChange((r) => ({...r, ibs_mun_p_red: v}))} placeholder="0.0000"/>
              </TaxField>
              <TaxField label="% Redução CBS">
                <NumericInput value={value.cbs_p_red ?? ''} decimal decimalPlaces={4}
                              onChange={(v) => onChange((r) => ({...r, cbs_p_red: v}))} placeholder="0.0000"/>
              </TaxField>
            </div>
          )}

          {/* Toggle diferimento */}
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ibs-dif`} checked={showIbsCbsDif}
                   onChange={(e) => {
                     setShowIbsCbsDif(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, ibs_uf_p_dif: '', ibs_mun_p_dif: '', cbs_p_dif: ''
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ibs-dif`} className="text-xs font-medium text-gray-500 cursor-pointer">
              Diferimento (CST 200/220)
            </label>
          </div>
          {showIbsCbsDif && (
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
              <TaxField label="% Diferimento IBS UF">
                <NumericInput value={value.ibs_uf_p_dif ?? ''} decimal decimalPlaces={4}
                              onChange={(v) => onChange((r) => ({...r, ibs_uf_p_dif: v}))} placeholder="0.0000"/>
              </TaxField>
              <TaxField label="% Diferimento IBS Mun">
                <NumericInput value={value.ibs_mun_p_dif ?? ''} decimal decimalPlaces={4}
                              onChange={(v) => onChange((r) => ({...r, ibs_mun_p_dif: v}))} placeholder="0.0000"/>
              </TaxField>
              <TaxField label="% Diferimento CBS">
                <NumericInput value={value.cbs_p_dif ?? ''} decimal decimalPlaces={4}
                              onChange={(v) => onChange((r) => ({...r, cbs_p_dif: v}))} placeholder="0.0000"/>
              </TaxField>
            </div>
          )}

          {/* Doação e devolução de tributo — dois campos avulsos do grupo. */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <label htmlFor={`${uid}-ibs-ind-doacao`}
                   className="flex items-center gap-2 min-h-11 text-sm text-gray-700 cursor-pointer">
              <input type="checkbox" id={`${uid}-ibs-ind-doacao`}
                     checked={value.ibs_ind_doacao === IBS_IND_DOACAO_SIM}
                     onChange={(e) => onChange((r) => ({
                       ...r, ibs_ind_doacao: e.target.checked ? IBS_IND_DOACAO_SIM : '',
                     }))}
                     className="h-4 w-4 rounded border-gray-300 text-brand-600"/>
              Operação é doação (indDoacao)
            </label>
            <TaxField label="% Devolução de tributo">
              <NumericInput value={value.ibs_cbs_p_dev_trib ?? ''} decimal decimalPlaces={4} integerPlaces={3}
                            onChange={(v) => onChange((r) => ({...r, ibs_cbs_p_dev_trib: v}))}
                            placeholder="0.0000"/>
              <p className="text-xs text-gray-500">
                Um percentual só: vale nas três esferas, sobre o tributo de cada uma.
              </p>
            </TaxField>
          </div>

          {/* Toggle monofasia do IBS/CBS (CST 620) */}
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ibs-mono`} checked={showIbsMono}
                   onChange={(e) => {
                     setShowIbsMono(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, ibs_ad_rem: '', cbs_ad_rem: '',
                       ibs_ad_rem_reten: '', cbs_ad_rem_reten: '',
                       ibs_ad_rem_ret: '', cbs_ad_rem_ret: '',
                       ibs_p_dif_mono: '', cbs_p_dif_mono: '',
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ibs-mono`} className="text-xs font-medium text-gray-500 cursor-pointer">
              Monofasia IBS/CBS (CST 620)
            </label>
          </div>
          {showIbsMono && (
            <div className="space-y-2">
              <p className="text-xs text-gray-500">
                Alíquota por unidade (R$), não percentual: a base é a quantidade vendida. Os valores e
                os totais do item são calculados na emissão.
              </p>
              {([
                ['ibs_ad_rem', 'cbs_ad_rem', 'Padrão'],
                ['ibs_ad_rem_reten', 'cbs_ad_rem_reten', 'Com retenção'],
                ['ibs_ad_rem_ret', 'cbs_ad_rem_ret', 'Já retido anteriormente'],
              ] as const).map(([ibsKey, cbsKey, label]) => (
                <div key={label} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-2">
                  <TaxField label={`${label} — IBS R$/un`}>
                    <NumericInput value={value[ibsKey] ?? ''} decimal decimalPlaces={4}
                                  onChange={(v) => onChange((r) => ({...r, [ibsKey]: v}))} placeholder="0.0000"/>
                  </TaxField>
                  <TaxField label={`${label} — CBS R$/un`}>
                    <NumericInput value={value[cbsKey] ?? ''} decimal decimalPlaces={4}
                                  onChange={(v) => onChange((r) => ({...r, [cbsKey]: v}))} placeholder="0.0000"/>
                  </TaxField>
                </div>
              ))}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                <TaxField label="% Diferimento IBS monofásico">
                  <NumericInput value={value.ibs_p_dif_mono ?? ''} decimal decimalPlaces={4}
                                onChange={(v) => onChange((r) => ({...r, ibs_p_dif_mono: v}))} placeholder="0.0000"/>
                </TaxField>
                <TaxField label="% Diferimento CBS monofásica">
                  <NumericInput value={value.cbs_p_dif_mono ?? ''} decimal decimalPlaces={4}
                                onChange={(v) => onChange((r) => ({...r, cbs_p_dif_mono: v}))} placeholder="0.0000"/>
                </TaxField>
              </div>
            </div>
          )}

          {/* Toggle tributação de referência */}
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ibs-ref`} checked={showIbsRef}
                   onChange={(e) => {
                     setShowIbsRef(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, ibs_reg_cst: '', ibs_reg_class_trib: '',
                       ibs_reg_uf_aliq: '', ibs_reg_mun_aliq: '', cbs_reg_aliq: '',
                       ibs_gov_uf_aliq: '', ibs_gov_mun_aliq: '', cbs_gov_aliq: '',
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ibs-ref`} className="text-xs font-medium text-gray-500 cursor-pointer">
              Tributação de referência e de compra governamental
            </label>
          </div>
          {showIbsRef && (
            <div className="space-y-2">
              <p className="text-xs text-gray-500">
                Quanto o item pagaria fora do regime ou benefício — é o que mede o incentivo. Os
                valores saem das alíquotas na emissão.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                <TaxField label="CST de referência">
                  <Combobox value={value.ibs_reg_cst ?? ''}
                            onValueChange={(v) => onChange((r) => ({
                              ...r, ibs_reg_cst: v,
                              ibs_reg_class_trib: IBS_CBS_CLASS_BY_CST[v]?.[0]?.value ?? '',
                            }))}
                            options={IBS_CBS_CST_OPTIONS} placeholder="Não se aplica"
                            searchPlaceholder="Código ou descrição..."/>
                </TaxField>
                <TaxField label="Classificação de referência">
                  <Combobox value={value.ibs_reg_class_trib ?? ''}
                            onValueChange={(v) => onChange((r) => ({...r, ibs_reg_class_trib: v}))}
                            options={IBS_CBS_CLASS_BY_CST[value.ibs_reg_cst ?? ''] ?? []}
                            placeholder="Código" searchPlaceholder="Código ou descrição..."/>
                </TaxField>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                {([
                  ['ibs_reg_uf_aliq', 'IBS UF de referência %'],
                  ['ibs_reg_mun_aliq', 'IBS Mun de referência %'],
                  ['cbs_reg_aliq', 'CBS de referência %'],
                  ['ibs_gov_uf_aliq', 'IBS UF compra gov %'],
                  ['ibs_gov_mun_aliq', 'IBS Mun compra gov %'],
                  ['cbs_gov_aliq', 'CBS compra gov %'],
                ] as const).map(([key, label]) => (
                  <div key={key} className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">{label}</label>
                    <NumericInput value={value[key] ?? ''} decimal decimalPlaces={4} integerPlaces={3}
                                  onChange={(v) => onChange((r) => ({...r, [key]: v}))} placeholder="0.0000"/>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Toggle créditos presumidos e ALC/ZFM */}
          <div className="flex items-center gap-2">
            <input type="checkbox" id={`${uid}-toggle-ibs-cred`} checked={showIbsCred}
                   onChange={(e) => {
                     setShowIbsCred(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, ibs_cbs_c_cred_pres: '', ibs_p_cred_pres: '', cbs_p_cred_pres: '',
                       ibs_cbs_cred_pres_cond_sus: '', ibs_zfm_p_cred_pres: '',
                       alc_zfm_tp_cbs: '', alc_zfm_n_proc_suframa: '',
                     }))
                   }}
                   className="size-4 rounded border-gray-300 text-brand-600"/>
            <label htmlFor={`${uid}-toggle-ibs-cred`} className="text-xs font-medium text-gray-500 cursor-pointer">
              Crédito presumido e ALC/ZFM
            </label>
          </div>
          {showIbsCred && (
            <div className="space-y-2">
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <div className="grid gap-1 sm:col-span-3">
                  <label className="text-sm font-medium text-gray-700">Código do crédito presumido</label>
                  <OptionsSelect value={value.ibs_cbs_c_cred_pres ?? ''}
                                 onValueChange={(v) => onChange((r) => ({...r, ibs_cbs_c_cred_pres: v}))}
                                 options={IBS_CBS_C_CRED_PRES_OPTIONS} placeholder="Não se aplica"/>
                </div>
                <TaxField label="% Crédito IBS">
                  <NumericInput value={value.ibs_p_cred_pres ?? ''} decimal decimalPlaces={4} integerPlaces={3}
                                onChange={(v) => onChange((r) => ({...r, ibs_p_cred_pres: v}))} placeholder="0.0000"/>
                </TaxField>
                <TaxField label="% Crédito CBS">
                  <NumericInput value={value.cbs_p_cred_pres ?? ''} decimal decimalPlaces={4} integerPlaces={3}
                                onChange={(v) => onChange((r) => ({...r, cbs_p_cred_pres: v}))} placeholder="0.0000"/>
                </TaxField>
                <label htmlFor={`${uid}-ibs-cred-cond-sus`}
                       className="flex items-center gap-2 min-h-11 text-sm text-gray-700 cursor-pointer">
                  <input type="checkbox" id={`${uid}-ibs-cred-cond-sus`}
                         checked={value.ibs_cbs_cred_pres_cond_sus === IBS_IND_DOACAO_SIM}
                         onChange={(e) => onChange((r) => ({
                           ...r, ibs_cbs_cred_pres_cond_sus: e.target.checked ? IBS_IND_DOACAO_SIM : '',
                         }))}
                         className="h-4 w-4 rounded border-gray-300 text-brand-600"/>
                  Condição suspensiva
                </label>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <TaxField label="% Crédito IBS na ZFM">
                  <NumericInput value={value.ibs_zfm_p_cred_pres ?? ''} decimal decimalPlaces={4} integerPlaces={3}
                                onChange={(v) => onChange((r) => ({...r, ibs_zfm_p_cred_pres: v}))}
                                placeholder="0.0000"/>
                </TaxField>
                <TaxField label="Alíquota zero CBS (ALC/ZFM)">
                  <OptionsSelect value={value.alc_zfm_tp_cbs ?? ''}
                                 onValueChange={(v) => onChange((r) => ({...r, alc_zfm_tp_cbs: v as never}))}
                                 options={ALC_ZFM_TP_CBS_OPTIONS} placeholder="Não se aplica"/>
                </TaxField>
                <TaxField label="Processo Suframa">
                  <Input value={value.alc_zfm_n_proc_suframa ?? ''} maxLength={12} className="w-full"
                         placeholder="8 a 12 caracteres"
                         onChange={(e) => onChange((r) => ({...r, alc_zfm_n_proc_suframa: e.target.value}))}/>
                </TaxField>
              </div>
              <p className="text-xs text-gray-500">
                O crédito da operação e o da ZFM são alternativos no leiaute — com os dois preenchidos,
                o da operação é o emitido.
              </p>
            </div>
          )}
          </>
          )}
        </div>
          </div>
        )}
      </div>
    </div>
  )
}
