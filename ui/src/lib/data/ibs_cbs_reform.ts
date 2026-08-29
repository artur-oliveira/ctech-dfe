/**
 * Tabelas de domínio dos grupos avançados da reforma tributária (IBS/CBS),
 * conforme PL_010e_v1.02 (DFeTiposBasicos_v1.00.xsd).
 *
 * Nada aqui é convenção nossa: cada lista é a enumeração do próprio XSD ou a
 * tabela citada por ele. Onde o leiaute enumera, o formulário seleciona — um
 * campo livre num domínio fechado só serve para o operador errar.
 */

/** `TIndDoacao` enumera um valor só: 1. "S"/"N" era o domínio de uma NT anterior. */
export const IBS_IND_DOACAO_SIM = '1'

/** `TTpCredPresIBSZFM` — classificação da subapuração do IBS na ZFM. */
export const TP_CRED_PRES_IBS_ZFM_OPTIONS = [
  {value: '0', label: '0 – Não se aplica'},
  {value: '1', label: '1 – Bens de consumo final'},
  {value: '2', label: '2 – Bens de capital'},
  {value: '3', label: '3 – Bens intermediários'},
  {value: '4', label: '4 – Bens de informática e demais'},
]

/** `tpALCZFMCBS` — tipo de aplicação da alíquota zero da CBS em ALC/ZFM. */
export const ALC_ZFM_TP_CBS_OPTIONS = [
  {value: '1', label: '1 – Alíquota zero da CBS'},
  {value: '2', label: '2 – Alíquota zero com crédito presumido'},
]

/** `TEnteGov` — ente governamental comprador (ide/gCompraGov/tpEnteGov). */
export const COMPRA_GOV_TP_ENTE_OPTIONS = [
  {value: '1', label: '1 – União'},
  {value: '2', label: '2 – Estados'},
  {value: '3', label: '3 – Distrito Federal'},
  {value: '4', label: '4 – Municípios'},
  {value: '5', label: '5 – Consórcio Público'},
  {value: '6', label: '6 – Comitê Gestor do IBS'},
]

/**
 * `TOperCompraGov` — tipo da operação com ente governamental. O rótulo diz o
 * que a escolha implica em `refDFeAnt`, porque essa é exatamente a regra que
 * só apareceria como rejeição.
 */
export const COMPRA_GOV_TP_OPER_OPTIONS = [
  {value: '1', label: '1 – Fornecimento com pagamento posterior (sem documento anterior)'},
  {value: '2', label: '2 – Recebimento do pagamento, fornecimento já feito (1 documento anterior)'},
  {value: '3', label: '3 – Fornecimento com pagamento já feito (1 ou mais documentos anteriores)'},
  {value: '4', label: '4 – Recebimento do pagamento, fornecimento posterior (sem documento anterior)'},
]

/** Tipos de operação governamental que exigem `refDFeAnt`. */
export const COMPRA_GOV_TP_OPER_COM_REFERENCIA = new Set(['2', '3'])

/** Tipo de operação governamental que aceita **uma** chave referenciada só. */
export const COMPRA_GOV_TP_OPER_REFERENCIA_UNICA = '2'

/**
 * Tabela `cCredPres` — código de classificação do crédito presumido do IBS e da
 * CBS (Informe Técnico RT 2025.002, LC 214/2025). O XSD (`TcCredPres`) exige
 * **dois dígitos**, então os códigos vão com zero à esquerda.
 */
export const IBS_CBS_C_CRED_PRES_OPTIONS = [
  {value: '01', label: '01 – Produtor rural (e integrado) não contribuinte'},
  {value: '02', label: '02 – Transportador autônomo de carga PF não contribuinte'},
  {value: '03', label: '03 – Resíduos e materiais para reciclagem, reutilização ou logística reversa'},
  {value: '04', label: '04 – Bens móveis usados de PF não contribuinte, para revenda'},
  {value: '05', label: '05 – Regime automotivo (art. 311)'},
  {value: '06', label: '06 – Regime automotivo (art. 312)'},
  {value: '07', label: '07 – Zona Franca de Manaus (art. 444)'},
  {value: '08', label: '08 – Zona Franca de Manaus (art. 447)'},
  {value: '09', label: '09 – Zona Franca de Manaus (art. 449)'},
  {value: '10', label: '10 – Zona Franca de Manaus (art. 450)'},
  {value: '11', label: '11 – Área de Livre Comércio (art. 462)'},
  {value: '12', label: '12 – Área de Livre Comércio (art. 465)'},
  {value: '13', label: '13 – Aquisição pela indústria na Área de Livre Comércio (art. 467)'},
]

/**
 * `TTpNFDebito` — motivo da nota de débito (`ide/tpNFDebito`). São códigos de
 * **dois dígitos**, não o 0/1 de entrada/saída do `tpNF`.
 */
export const TP_NF_DEBITO_OPTIONS = [
  {value: '01', label: '01 – Transferência de créditos para cooperativas'},
  {value: '02', label: '02 – Anulação de crédito por saídas imunes/isentas'},
  {value: '03', label: '03 – Débitos de notas não processadas na apuração'},
  {value: '04', label: '04 – Multa e juros'},
  {value: '05', label: '05 – Transferência de crédito na sucessão'},
  {value: '06', label: '06 – Pagamento antecipado'},
  {value: '07', label: '07 – Perda em estoque (perecimento, perda, furto, roubo)'},
  {value: '08', label: '08 – Desenquadramento do Simples Nacional'},
]

/** `TTpNFCredito` — motivo da nota de crédito (`ide/tpNFCredito`). */
export const TP_NF_CREDITO_OPTIONS = [
  {value: '01', label: '01 – Multa e juros'},
  {value: '02', label: '02 – Crédito presumido de IBS sobre saldo devedor na ZFM (art. 450, §1º)'},
  {value: '03', label: '03 – Retorno por recusa na entrega ou destinatário não localizado'},
  {value: '04', label: '04 – Redução de valores'},
  {value: '05', label: '05 – Transferência de crédito na sucessão'},
  {value: '06', label: '06 – Retorno por recusa parcial na entrega'},
]
