'use client'

import {Combobox} from '@/components/ui/combobox'
import {OptionsSelect} from '@/components/ui/options-select'
import {NumericInput} from '@/components/ui/numeric-input'
import {Input} from '@/components/ui/input'
import type {CfopConfigFormData} from '@/lib/schemas/products'
import {getAllCfopOptions} from '@/lib/data/cfop'
import {getCfopHint} from '@/lib/data/cfop_rules'
import {IBS_CBS_CLASS_BY_CST, IBS_CBS_CST_OPTIONS} from '@/lib/data/ibs_cbs_cst'
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
}

export const EMPTY_TAX_GROUPS: TaxGroups = {
  ipi: false, is: false, ibsCbs: false, ibsRed: false, ibsDif: false, issqn: false,
  icmsMono: false, pisCofinsSt: false,
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
})

/** icms_mod_bc cujo cálculo usa um valor fixo em vez do valor de venda. */
const ICMS_MOD_BC_PAUTA = new Set(['1', '2'])

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
    pisCofinsSt: showPisCofinsSt} = groups
  const setGroup = (key: keyof TaxGroups) => (on: boolean) => onGroupsChange({...groups, [key]: on})
  const setShowIpi = setGroup('ipi')
  const setShowIs = setGroup('is')
  const setShowIbsCbs = setGroup('ibsCbs')
  const setShowIbsCbsRed = setGroup('ibsRed')
  const setShowIbsCbsDif = setGroup('ibsDif')
  const setShowIssqn = setGroup('issqn')
  const setShowIcmsMono = setGroup('icmsMono')
  const setShowPisCofinsSt = setGroup('pisCofinsSt')

  const systemAliq = useIcmsAliqPreview(emitUf, destUf, ncm)
  const aliqDiverges = !!systemAliq && !!value.icms_aliq_override &&
    value.icms_aliq_override !== systemAliq.icms_aliq

  const {showPRedBC, showMotDeSon, showPDif} = icmsConditionalFields(value.icms ?? '')
  const cfopOptions = getAllCfopOptions()
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
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">Dados da operação</p>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {!hideCfop && (
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">CFOP *</label>
              <Combobox value={value.cfop}
                        onValueChange={(v) => onChange((r) => ({...r, cfop: v}))}
                        options={cfopOptions} placeholder="CFOP" searchPlaceholder="Código ou descrição..."/>
            </div>
          )}
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">
              {simples ? 'CSOSN *' : 'ICMS CST *'}
            </label>
            {simples ? (
              <OptionsSelect value={value.csosn ?? ''}
                             onValueChange={(v) => onChange((r) => ({...r, csosn: v}))}
                             options={CSOSN_OPTIONS} placeholder="CSOSN"/>
            ) : (
              <OptionsSelect value={value.icms ?? ''}
                             onValueChange={(v) => onChange((r) => ({
                               ...r, icms: v,
                               icms_p_red_bc: '', icms_mot_des: '', icms_p_dif: '',
                               icms_aliq_override: '', icms_fcp_override: '',
                               icms_st_mva: '', icms_st_aliq: '', icms_st_fcp_aliq: '',
                             }))}
                             options={ICMS_CST_OPTIONS} placeholder="CST"/>
            )}
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">PIS *</label>
            <OptionsSelect value={value.pis}
                           onValueChange={(v) => onChange((r) => ({
                             ...r,
                             pis: v,
                             pis_aliq: '',
                             pis_aliq_unid: ''
                           }))}
                           options={PIS_COFINS_OPTIONS} placeholder="PIS"/>
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">COFINS *</label>
            <OptionsSelect value={value.cofins}
                           onValueChange={(v) => onChange((r) => ({
                             ...r,
                             cofins: v,
                             cofins_aliq: '',
                             cofins_aliq_unid: ''
                           }))}
                           options={PIS_COFINS_OPTIONS} placeholder="COFINS"/>
          </div>
        </div>

        {/* Hint fiscal CFOP */}
        {!hideCfop && cfopHint && (
          <div
            className="col-span-full flex items-center gap-2 rounded-lg bg-amber-50 border border-amber-200 px-3 py-2 text-sm text-amber-700">
            <span className="font-medium">!</span>
            <span>{cfopHint.label}</span>
          </div>
        )}

        {/* Campos condicionais ICMS Regime Normal */}
        {!simples && (showPRedBC || showMotDeSon || showPDif) && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-2 border-t border-gray-200">
            {showPRedBC && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">% Redução BC</label>
                <NumericInput value={value.icms_p_red_bc ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_red_bc: v}))}/>
              </div>
            )}
            {showMotDeSon && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Motivo desoneração *</label>
                <OptionsSelect value={value.icms_mot_des ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, icms_mot_des: v}))}
                               options={ICMS_MOT_DESONE_OPTIONS} placeholder="Motivo"/>
              </div>
            )}
            {showPDif && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">% Diferimento do FCP</label>
                <NumericInput value={value.icms_p_fcp_dif ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_fcp_dif: v}))}/>
              </div>
            )}
            {showPDif && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">% Diferimento</label>
                <NumericInput value={value.icms_p_dif ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_dif: v}))}/>
              </div>
            )}
          </div>
        )}

        {/* Alíquota ICMS — para CSTs tributados */}
        {!simples && value.icms && ICMS_TAXED_CSTS.has(value.icms) && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-2 border-t border-gray-200">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Alíquota ICMS %</label>
              <NumericInput value={value.icms_aliq_override ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                            placeholder="Padrão: tabela da UF"
                            onChange={(v) => onChange((r) => ({...r, icms_aliq_override: v}))}/>
              <p className="text-xs text-gray-400">
                {systemAliq ? `Vazio ou igual = usa a alíquota do sistema (${systemAliq.icms_aliq}%)`
                  : 'Vazio = usa alíquota padrão da UF de destino'}
              </p>
            </div>
            {['00', '10', '20', '51', '70', '90'].includes(value.icms) && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Modo de cálculo</label>
                <OptionsSelect value={value.icms_mod_bc ?? '3'}
                               onValueChange={(v) => onChange((r) => ({...r, icms_mod_bc: v}))}
                               options={MOD_BC_OPTIONS} placeholder="Modo de cálculo"/>
              </div>
            )}
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">FCP %</label>
              <NumericInput value={value.icms_fcp_override ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                            placeholder="Padrão: tabela da UF"
                            onChange={(v) => onChange((r) => ({...r, icms_fcp_override: v}))}/>
            </div>
            {ICMS_MOD_BC_PAUTA.has(value.icms_mod_bc ?? '') && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Valor da pauta fiscal (R$)</label>
                <NumericInput value={value.icms_pauta_valor ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, icms_pauta_valor: v}))}/>
              </div>
            )}
            {aliqDiverges && (
              <div role="alert"
                   className="col-span-full rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700">
                Alíquota ICMS digitada ({value.icms_aliq_override}%) diverge da tabela do sistema
                para esta UF/NCM ({systemAliq?.icms_aliq}%).
              </div>
            )}
          </div>
        )}

        {/* pCredSN — Simples Nacional com crédito */}
        {simples && value.csosn && CSOSN_CRED.has(value.csosn) && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-2 border-t border-gray-200">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Crédito aproveitável</label>
              <NumericInput value={value.icms_sn_cred_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                            placeholder="Ex: 4.0000"
                            onChange={(v) => onChange((r) => ({...r, icms_sn_cred_aliq: v}))}/>
            </div>
          </div>
        )}

        {/* ST — Substituição Tributária */}
        {showSt && (
          <div className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 space-y-2">
            <p className="text-xs font-semibold text-blue-700 uppercase tracking-wider">
              Substituição Tributária (ST)
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Cálculo da BC ST</label>
                <OptionsSelect value={value.icms_st_mod_bc ?? '4'}
                               onValueChange={(v) => onChange((r) => ({...r, icms_st_mod_bc: v}))}
                               options={MOD_BC_ST_OPTIONS} placeholder="Modo"/>
              </div>
              {(!value.icms_st_mod_bc || value.icms_st_mod_bc === '4') && (
                <div className="grid gap-1">
                  <label className="text-sm font-medium text-gray-700">MVA %</label>
                  <NumericInput value={value.icms_st_mva ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                                placeholder="Ex: 30.0000"
                                onChange={(v) => onChange((r) => ({...r, icms_st_mva: v}))}/>
                </div>
              )}
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Alíquota ST %</label>
                <NumericInput value={value.icms_st_aliq ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="Ex: 18.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_st_aliq: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">FCP ST %</label>
                <NumericInput value={value.icms_st_fcp_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_st_fcp_aliq: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Motivo desoneração da ST</label>
                <OptionsSelect value={value.icms_mot_des_st ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, icms_mot_des_st: v}))}
                               options={ICMS_MOT_DESONE_OPTIONS} placeholder="Não desonerada"/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">% Redução BC ST</label>
                <NumericInput value={value.icms_st_red_bc ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_st_red_bc: v}))}/>
              </div>
            </div>
          </div>
        )}

        {/* ST retida anteriormente + ICMS efetivo */}
        {showStRet && (
          <div className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 space-y-2">
            <p className="text-xs font-semibold text-blue-700 uppercase tracking-wider">
              ST retida anteriormente
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">BC da ST retida</label>
                <NumericInput value={value.icms_v_bc_st_ret ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, icms_v_bc_st_ret: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">ICMS-ST retido</label>
                <NumericInput value={value.icms_v_icms_st_ret ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, icms_v_icms_st_ret: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Alíquota suportada %</label>
                <NumericInput value={value.icms_p_st ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_st: v}))}/>
              </div>
              {value.icms === '41' && (
                <>
                  <div className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">BC da ST na UF de destino</label>
                    <NumericInput value={value.icms_v_bc_st_dest ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                                  placeholder="0.00"
                                  onChange={(v) => onChange((r) => ({...r, icms_v_bc_st_dest: v}))}/>
                  </div>
                  <div className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">ICMS-ST da UF de destino</label>
                    <NumericInput value={value.icms_v_icms_st_dest ?? ''} decimal integerPlaces={13} decimalPlaces={2}
                                  placeholder="0.00"
                                  onChange={(v) => onChange((r) => ({...r, icms_v_icms_st_dest: v}))}/>
                  </div>
                </>
              )}
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">% Redução BC efetiva</label>
                <NumericInput value={value.icms_p_red_bc_efet ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_red_bc_efet: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Alíquota efetiva %</label>
                <NumericInput value={value.icms_p_icms_efet ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_p_icms_efet: v}))}/>
              </div>
            </div>
            <p className="text-xs text-gray-500">
              A base e o valor do ICMS efetivo são calculados na emissão — informe só os percentuais.
            </p>
          </div>
        )}

        {/* Partilha do ICMS entre UFs (ICMSPart) */}
        {showPart && (
          <div className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 space-y-2">
            <p className="text-xs font-semibold text-blue-700 uppercase tracking-wider">
              Partilha do ICMS entre UFs
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">% da BC na origem</label>
                <NumericInput value={value.icms_part_p_bc_op ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                              placeholder="0.0000"
                              onChange={(v) => onChange((r) => ({...r, icms_part_p_bc_op: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">UF do ST</label>
                <OptionsSelect value={value.icms_part_uf_st ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, icms_part_uf_st: v}))}
                               options={UF_OPTIONS} placeholder="UF"/>
              </div>
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
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Alíquota PIS %</label>
                <NumericInput value={value.pis_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="Ex: 0.6500"
                              onChange={(v) => onChange((r) => ({...r, pis_aliq: v}))}/>
              </div>
            )}
            {value.pis && PIS_COFINS_QTDE_CSTS.has(value.pis) && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">PIS R$/unid</label>
                <NumericInput value={value.pis_aliq_unid ?? ''} decimal integerPlaces={4} decimalPlaces={4}
                              placeholder="Ex: 0.0065"
                              onChange={(v) => onChange((r) => ({...r, pis_aliq_unid: v}))}/>
              </div>
            )}
            {value.cofins && PIS_COFINS_ALIQ_CSTS.has(value.cofins) && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Alíquota COFINS %</label>
                <NumericInput value={value.cofins_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="Ex: 3.0000"
                              onChange={(v) => onChange((r) => ({...r, cofins_aliq: v}))}/>
              </div>
            )}
            {value.cofins && PIS_COFINS_QTDE_CSTS.has(value.cofins) && (
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">COFINS R$/unid</label>
                <NumericInput value={value.cofins_aliq_unid ?? ''} decimal integerPlaces={4} decimalPlaces={4}
                              placeholder="Ex: 0.0300"
                              onChange={(v) => onChange((r) => ({...r, cofins_aliq_unid: v}))}/>
              </div>
            )}
          </div>
        )}
      </div>

      {/* ── PIS/COFINS-ST ───────────────────────────────────────── */}
      <div className="rounded-lg border border-gray-100 p-3 space-y-3">
        <div className="flex items-center gap-2">
          <input type="checkbox" id="toggle-pis-cofins-st" checked={showPisCofinsSt}
                 onChange={(e) => {
                   setShowPisCofinsSt(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({
                     ...r, pis_st_aliq: '', cofins_st_aliq: '', pis_st_v_bc: '', cofins_st_v_bc: '',
                   }))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-pis-cofins-st"
                 className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer select-none">
            PIS/COFINS-ST — Substituição Tributária
          </label>
        </div>
        {showPisCofinsSt && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Alíquota PIS-ST %</label>
              <NumericInput value={value.pis_st_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                            placeholder="0.0000"
                            onChange={(v) => onChange((r) => ({...r, pis_st_aliq: v}))}/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Alíquota COFINS-ST %</label>
              <NumericInput value={value.cofins_st_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                            placeholder="0.0000"
                            onChange={(v) => onChange((r) => ({...r, cofins_st_aliq: v}))}/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">BC PIS-ST (R$)</label>
              <NumericInput value={value.pis_st_v_bc ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                            placeholder="0.00"
                            onChange={(v) => onChange((r) => ({...r, pis_st_v_bc: v}))}/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">BC COFINS-ST (R$)</label>
              <NumericInput value={value.cofins_st_v_bc ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                            placeholder="0.00"
                            onChange={(v) => onChange((r) => ({...r, cofins_st_v_bc: v}))}/>
            </div>
          </div>
        )}
      </div>

      {/* ── IPI ─────────────────────────────────────────────────── */}
      <div className="rounded-lg border border-gray-100 p-3 space-y-3">
        <div className="flex items-center gap-2">
          <input type="checkbox" id="toggle-ipi" checked={showIpi}
                 onChange={(e) => {
                   setShowIpi(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({...r, ipi_cst: '', ipi_aliq: ''}))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-ipi"
                 className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer select-none">
            IPI — Imposto sobre Produtos Industrializados
          </label>
        </div>
        {showIpi && (
          <div className="grid grid-cols-2 gap-2 max-w-sm">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">CST IPI *</label>
              <OptionsSelect value={value.ipi_cst ?? ''}
                             onValueChange={(v) => onChange((r) => ({...r, ipi_cst: v}))}
                             options={IPI_CST_OPTIONS} placeholder="CST"/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Alíquota %</label>
              <NumericInput value={value.ipi_aliq ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                            placeholder="0.0000" disabled={!!value.ipi_v_unid}
                            onChange={(v) => onChange((r) => ({...r, ipi_aliq: v}))}/>
            </div>
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
          <input type="checkbox" id="toggle-is" checked={showIs}
                 onChange={(e) => {
                   setShowIs(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({
                     ...r, is_cst: '', is_aliq: '', is_class_trib: '', is_aliq_espec: '', is_unid_trib: ''
                   }))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-is"
                 className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer select-none">
            IS — Imposto Seletivo (NT 2024.001)
          </label>
        </div>
        {showIs && (
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">CST IS *</label>
              <OptionsSelect value={value.is_cst ?? ''}
                             onValueChange={(v) => onChange((r) => ({...r, is_cst: v}))}
                             options={IS_CST_OPTIONS} placeholder="CST"/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Alíquota %</label>
              <NumericInput value={value.is_aliq ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                            placeholder="0.0000"
                            onChange={(v) => onChange((r) => ({...r, is_aliq: v}))}/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Classificação IS</label>
              <NumericInput value={value.is_class_trib ?? ''}
                            placeholder="000000" maxLength={6}
                            onChange={(v) => onChange((r) => ({...r, is_class_trib: v}))}/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Alíquota específica</label>
              <NumericInput value={value.is_aliq_espec ?? ''} decimal integerPlaces={3} decimalPlaces={4}
                            placeholder="0.0000"
                            onChange={(v) => onChange((r) => ({...r, is_aliq_espec: v}))}/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">Unid. tributável IS</label>
              <Input value={value.is_unid_trib ?? ''}
                     placeholder="Ex: UN"
                     onChange={(e) => onChange((r) => ({...r, is_unid_trib: e.target.value}))}/>
            </div>
          </div>
        )}
      </div>

      {/* ── ICMS Monofásico — Combustíveis (CST 02/15/53/61) ── */}
      {!simples && (
        <div className="rounded-lg border border-gray-100 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <input type="checkbox" id="toggle-mono" checked={showIcmsMono}
                   onChange={(e) => {
                     setShowIcmsMono(e.target.checked)
                     if (!e.target.checked) onChange((r) => ({
                       ...r, icms_ad_rem: '', icms_ad_rem_reten: '',
                       icms_p_red_ad_rem: '', icms_mot_red_ad_rem: '', icms_p_dif_mono: '',
                     }))
                   }}
                   className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
            <label htmlFor="toggle-mono"
                   className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer select-none">
              ICMS Monofásico — Combustíveis (CST 02/15/53/61)
            </label>
          </div>
          {(showIcmsMono || ICMS_MONO_CSTS.has(value.icms ?? '')) && (
            <div className="space-y-2">
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                <div className="grid gap-1">
                  <label className="text-sm font-medium text-gray-700">Ad rem ICMS (R$/un) *</label>
                  <NumericInput value={value.icms_ad_rem ?? ''} decimal integerPlaces={4} decimalPlaces={4}
                                placeholder="Ex: 1.5000"
                                onChange={(v) => onChange((r) => ({...r, icms_ad_rem: v}))}/>
                </div>
                {value.icms === '53' && (
                  <div className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">% Diferimento (53)</label>
                    <NumericInput value={value.icms_p_dif_mono ?? ''} decimal integerPlaces={3}
                                  decimalPlaces={4}
                                  placeholder="0.0000"
                                  onChange={(v) => onChange((r) => ({...r, icms_p_dif_mono: v}))}/>
                  </div>
                )}
                {value.icms === '15' && (
                  <>
                    <div className="grid gap-1">
                      <label className="text-sm font-medium text-gray-700">Ad rem retenção (15)</label>
                      <NumericInput value={value.icms_ad_rem_reten ?? ''} decimal integerPlaces={4}
                                    decimalPlaces={4}
                                    placeholder="0.0000"
                                    onChange={(v) => onChange((r) => ({...r, icms_ad_rem_reten: v}))}/>
                    </div>
                    <div className="grid gap-1">
                      <label className="text-sm font-medium text-gray-700">% Redução ad rem</label>
                      <NumericInput value={value.icms_p_red_ad_rem ?? ''} decimal integerPlaces={3}
                                    decimalPlaces={4}
                                    placeholder="0.0000"
                                    onChange={(v) => onChange((r) => ({...r, icms_p_red_ad_rem: v}))}/>
                    </div>
                    <div className="grid gap-1">
                      <label className="text-sm font-medium text-gray-700">Motivo redução</label>
                      <OptionsSelect value={value.icms_mot_red_ad_rem ?? ''}
                                     onValueChange={(v) => onChange((r) => ({...r, icms_mot_red_ad_rem: v}))}
                                     options={[{value: '1', label: '1 – Transporte coletivo'}, {
                                       value: '9',
                                       label: '9 – Outros'
                                     }]}
                                     placeholder="Motivo"/>
                    </div>
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
          <input type="checkbox" id="toggle-issqn" checked={showIssqn}
                 onChange={(e) => {
                   setShowIssqn(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({
                     ...r, issqn_ind_iss: '', issqn_c_list_serv: '',
                     issqn_c_mun_fg: '', issqn_aliq: '', issqn_v_deducao: '', issqn_v_iss_ret: '',
                   }))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-issqn"
                 className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer select-none">
            ISSQN — Imposto Sobre Serviços (LC 116/2003)
          </label>
        </div>
        {showIssqn && (
          <div className="space-y-2">
            <div className="rounded-sm border border-blue-100 bg-blue-50/20 px-3 py-1.5 text-xs text-blue-700">
              Quando habilitado, o item usa ISSQN no lugar de ICMS no XML da NF-e.
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Exigibilidade ISS *</label>
                <OptionsSelect value={value.issqn_ind_iss ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, issqn_ind_iss: v}))}
                               options={ISSQN_IND_ISS_OPTIONS} placeholder="Exigibilidade"/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Lista LC 116 (cListServ)</label>
                <Input value={value.issqn_c_list_serv ?? ''}
                       placeholder="Ex: 01.01"
                       onChange={(e) => onChange((r) => ({...r, issqn_c_list_serv: e.target.value}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">IBGE Município FG</label>
                <NumericInput value={value.issqn_c_mun_fg ?? ''} maxLength={7}
                              placeholder="7 dígitos"
                              onChange={(v) => onChange((r) => ({...r, issqn_c_mun_fg: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Alíquota ISSQN %</label>
                <NumericInput value={value.issqn_aliq ?? ''} decimal integerPlaces={2} decimalPlaces={4}
                              placeholder="5.0000"
                              onChange={(v) => onChange((r) => ({...r, issqn_aliq: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Dedução R$</label>
                <NumericInput value={value.issqn_v_deducao ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, issqn_v_deducao: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Retenção ISS R$</label>
                <NumericInput value={value.issqn_v_iss_ret ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, issqn_v_iss_ret: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Outras retenções R$</label>
                <NumericInput value={value.issqn_v_outro ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, issqn_v_outro: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Desconto incondicional R$</label>
                <NumericInput value={value.issqn_v_desc_incond ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, issqn_v_desc_incond: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Desconto condicional R$</label>
                <NumericInput value={value.issqn_v_desc_cond ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                              placeholder="0.00"
                              onChange={(v) => onChange((r) => ({...r, issqn_v_desc_cond: v}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Código do serviço no município</label>
                <Input value={value.issqn_c_servico ?? ''} maxLength={20}
                       onChange={(e) => onChange((r) => ({...r, issqn_c_servico: e.target.value}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Município de incidência (IBGE)</label>
                <Input value={value.issqn_c_mun ?? ''} maxLength={7} inputMode="numeric"
                       onChange={(e) => onChange((r) => ({...r, issqn_c_mun: e.target.value.replace(/\D/g, '')}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">País do serviço</label>
                <Input value={value.issqn_c_pais ?? ''} maxLength={4} inputMode="numeric" placeholder="1058"
                       onChange={(e) => onChange((r) => ({...r, issqn_c_pais: e.target.value.replace(/\D/g, '')}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Nº do processo</label>
                <Input value={value.issqn_n_processo ?? ''} maxLength={30}
                       onChange={(e) => onChange((r) => ({...r, issqn_n_processo: e.target.value}))}/>
              </div>
              <div className="grid gap-1">
                <label className="text-sm font-medium text-gray-700">Incentivo fiscal</label>
                <OptionsSelect value={value.issqn_ind_incentivo ?? ''}
                               onValueChange={(v) => onChange((r) => ({...r, issqn_ind_incentivo: v}))}
                               placeholder="Não informado"
                               options={[{value: '1', label: '1 – Sim'}, {value: '2', label: '2 – Não'}]}/>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* ── Observação fiscal do item (obsItem) ─────────────────── */}
      <div className="rounded-lg border border-gray-100 p-3 space-y-3">
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
          Observação fiscal do item
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">Campo</label>
            <Input value={value.obs_item_x_campo ?? ''} maxLength={20} placeholder="Ex: Beneficio"
                   onChange={(e) => onChange((r) => ({...r, obs_item_x_campo: e.target.value}))}/>
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">Texto</label>
            <Input value={value.obs_item_x_texto ?? ''} maxLength={60}
                   onChange={(e) => onChange((r) => ({...r, obs_item_x_texto: e.target.value}))}/>
          </div>
        </div>
      </div>

      {/* ── IBS / CBS ───────────────────────────────────────────── */}
      <div className="rounded-lg border border-gray-100 p-3 space-y-3">
        <div className="flex items-center gap-2">
          <input type="checkbox" id="toggle-ibs-cbs" checked={showIbsCbs}
                 onChange={(e) => {
                   setShowIbsCbs(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({
                     ...r, ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
                     ibs_uf_p_red: '', ibs_mun_p_red: '', cbs_p_red: '',
                     ibs_uf_p_dif: '', ibs_mun_p_dif: '', cbs_p_dif: '',
                   }))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-ibs-cbs"
                 className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer select-none">
            IBS / CBS — Reforma Tributária
          </label>
        </div>
        {showIbsCbs && (
        <>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">CST</label>
            <OptionsSelect value={value.ibs_cbs_cst ?? ''}
                           onValueChange={(v) => onChange((r) => ({
                             ...r,
                             ibs_cbs_cst: v,
                             ibs_cbs_class_trib: IBS_CBS_CLASS_BY_CST[v]?.[0]?.value ?? ''
                           }))}
                           options={IBS_CBS_CST_OPTIONS} placeholder="CST"/>
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">Classificação</label>
            <OptionsSelect value={value.ibs_cbs_class_trib ?? ''}
                           onValueChange={(v) => onChange((r) => ({...r, ibs_cbs_class_trib: v}))}
                           options={IBS_CBS_CLASS_BY_CST[value.ibs_cbs_cst ?? ''] ?? []} placeholder="Código"/>
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">IBS UF %</label>
            <NumericInput decimal decimalPlaces={4} integerPlaces={3} value={value.ibs_uf_aliq ?? ''}
                          onChange={(v) => onChange((r) => ({...r, ibs_uf_aliq: v}))} placeholder="0.0000"/>
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">IBS Mun %</label>
            <NumericInput decimal decimalPlaces={4} integerPlaces={3} value={value.ibs_mun_aliq ?? ''}
                          onChange={(v) => onChange((r) => ({...r, ibs_mun_aliq: v}))} placeholder="0.0000"/>
          </div>
          <div className="grid gap-1">
            <label className="text-sm font-medium text-gray-700">CBS %</label>
            <NumericInput decimal decimalPlaces={4} integerPlaces={3} value={value.cbs_aliq ?? ''}
                          onChange={(v) => onChange((r) => ({...r, cbs_aliq: v}))} placeholder="0.0000"/>
          </div>
        </div>

        {/* Toggle redução */}
        <div className="flex items-center gap-2">
          <input type="checkbox" id="toggle-ibs-red" checked={showIbsCbsRed}
                 onChange={(e) => {
                   setShowIbsCbsRed(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({
                     ...r, ibs_uf_p_red: '', ibs_mun_p_red: '', cbs_p_red: ''
                   }))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-ibs-red" className="text-xs font-medium text-gray-500 cursor-pointer">
            Redução de alíquota (CST 010/011)
          </label>
        </div>
        {showIbsCbsRed && (
          <div className="grid grid-cols-3 gap-2">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Redução IBS UF</label>
              <NumericInput value={value.ibs_uf_p_red ?? ''} decimal decimalPlaces={4}
                            onChange={(v) => onChange((r) => ({...r, ibs_uf_p_red: v}))} placeholder="0.0000"/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Redução IBS Mun</label>
              <NumericInput value={value.ibs_mun_p_red ?? ''} decimal decimalPlaces={4}
                            onChange={(v) => onChange((r) => ({...r, ibs_mun_p_red: v}))} placeholder="0.0000"/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Redução CBS</label>
              <NumericInput value={value.cbs_p_red ?? ''} decimal decimalPlaces={4}
                            onChange={(v) => onChange((r) => ({...r, cbs_p_red: v}))} placeholder="0.0000"/>
            </div>
          </div>
        )}

        {/* Toggle diferimento */}
        <div className="flex items-center gap-2">
          <input type="checkbox" id="toggle-ibs-dif" checked={showIbsCbsDif}
                 onChange={(e) => {
                   setShowIbsCbsDif(e.target.checked)
                   if (!e.target.checked) onChange((r) => ({
                     ...r, ibs_uf_p_dif: '', ibs_mun_p_dif: '', cbs_p_dif: ''
                   }))
                 }}
                 className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
          <label htmlFor="toggle-ibs-dif" className="text-xs font-medium text-gray-500 cursor-pointer">
            Diferimento (CST 200/220)
          </label>
        </div>
        {showIbsCbsDif && (
          <div className="grid grid-cols-3 gap-2">
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Diferimento IBS UF</label>
              <NumericInput value={value.ibs_uf_p_dif ?? ''} decimal decimalPlaces={4}
                            onChange={(v) => onChange((r) => ({...r, ibs_uf_p_dif: v}))} placeholder="0.0000"/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Diferimento IBS Mun</label>
              <NumericInput value={value.ibs_mun_p_dif ?? ''} decimal decimalPlaces={4}
                            onChange={(v) => onChange((r) => ({...r, ibs_mun_p_dif: v}))} placeholder="0.0000"/>
            </div>
            <div className="grid gap-1">
              <label className="text-sm font-medium text-gray-700">% Diferimento CBS</label>
              <NumericInput value={value.cbs_p_dif ?? ''} decimal decimalPlaces={4}
                            onChange={(v) => onChange((r) => ({...r, cbs_p_dif: v}))} placeholder="0.0000"/>
            </div>
          </div>
        )}
        </>
        )}
      </div>
    </div>
  )
}
