import type {CteIcmsCst, CteTakerRole, CteTakerSuggestion} from '@/lib/types/api';

/** Contrato: docs/specs/2026-11-05-cte-emissao-rodoviaria.md. */

/** Máximo de chaves por `derive` / emissão (spec: 1 a 50). */
export const CTE_MAX_NOTES = 50;

/** Espera entre a última mudança de notas e o `derive`. */
export const CTE_DERIVE_DEBOUNCE_MS = 300;

/** Notas recentes oferecidas no seletor (emitidas e recebidas, cada). */
export const CTE_RECENT_NOTES_LIMIT = 50;

/** `xProd` do produto predominante: 60 caracteres no leiaute. */
export const CTE_PRODUCT_MAX_LENGTH = 60;

/** `xObs`, observações gerais do CT-e. */
export const CTE_ADDITIONAL_INFO_MAX_LENGTH = 2000;

export const CTE_TAKER_ROLES: readonly CteTakerRole[] = ['remetente', 'destinatario', 'expedidor', 'recebedor', 'outro'];

export const CTE_TAKER_LABELS: Record<CteTakerRole, string> = {
  remetente: 'Remetente',
  destinatario: 'Destinatário',
  expedidor: 'Expedidor',
  recebedor: 'Recebedor',
  outro: 'Outro',
};

/** Papéis em que o tomador vem do cadastro de pessoas (`person_id` obrigatório). */
export const CTE_TAKER_NEEDS_PERSON: ReadonlySet<CteTakerRole> = new Set(['expedidor', 'recebedor', 'outro']);

export const CTE_TAKER_REASON_LABELS: Record<CteTakerSuggestion['reason'], string> = {
  mod_frete_cif: 'Frete CIF na nota',
  mod_frete_fob: 'Frete FOB na nota',
};

/** CST do ICMS do CT-e; `SN` é o ICMSSN (Simples Nacional, indSN 1). */
export const CTE_ICMS_CST_OPTIONS: { value: CteIcmsCst; label: string }[] = [
  {value: '00', label: '00 · Tributação normal'},
  {value: '20', label: '20 · Com redução de base de cálculo'},
  {value: '40', label: '40 · Isenta'},
  {value: '41', label: '41 · Não tributada'},
  {value: '51', label: '51 · Diferimento'},
  {value: '60', label: '60 · Cobrado anteriormente por ST'},
  {value: '90', label: '90 · Outras'},
  {value: 'SN', label: 'Simples Nacional'},
];

export const CTE_ICMS_SN: CteIcmsCst = 'SN';

/** CSTs que dispensam alíquota (spec: 40, 41 e 51; o SN também não destaca ICMS). */
export const CTE_CST_WITHOUT_RATE: ReadonlySet<CteIcmsCst> = new Set(['40', '41', '51', 'SN']);

/** CSTs em que o leiaute tem percentual de redução da base (ICMS20 e ICMS90). */
export const CTE_CST_WITH_REDUCTION: ReadonlySet<CteIcmsCst> = new Set(['20', '90']);

/** CRT 1 (Simples), 2 (excesso de sublimite) e 4 (MEI): ICMS padrão `SN`, como no servidor. */
export const CTE_SN_CRTS: ReadonlySet<string> = new Set(['1', '2', '4']);

/** Prefixo do CFOP: prestação dentro da UF (5) e entre UFs (6). */
export const CTE_CFOP_SCOPE_INTRA = '5';
export const CTE_CFOP_SCOPE_INTER = '6';

/**
 * Nomes de componente do valor da prestação (`Comp/xNome`, até 15 caracteres).
 * Escolha em lista, não digitação: são sempre os mesmos poucos nomes.
 * "FRETE VALOR" é reservado ao componente que o servidor monta com o restante.
 */
export const CTE_FREIGHT_COMPONENT_NAMES = [
  'Pedágio',
  'GRIS',
  'Ad valorem',
  'Seguro',
  'Coleta',
  'Entrega',
  'Descarga',
  'Armazenagem',
  'TDE',
  'Outros',
] as const;

/** 422 da emissão quando a organização não tem RNTRC; ele fica na configuração do MDF-e. */
export const CTE_ERROR_RNTRC_REQUIRED = 'rntrc_required';
export const CTE_RNTRC_CONFIG_HREF = '/fiscal-config?tab=mdfe';

/** Códigos de aviso do `derive`. */
export const CTE_WARNING = {
  MIXED_SENDER: 'mixed_sender',
  MIXED_RECIPIENT: 'mixed_recipient',
  TAKER_AMBIGUOUS: 'taker_ambiguous',
  WEIGHT_MISSING: 'weight_missing',
  NOTE_NOT_AUTHORIZED: 'note_not_authorized',
  NOTE_NOT_FOUND: 'note_not_found',
} as const;

/** Bloco da tela onde cada aviso aparece; aviso desconhecido fica no bloco das notas. */
export type CteBlock = 'notes' | 'taker' | 'transport'

export const CTE_WARNING_BLOCK: Record<string, CteBlock> = {
  [CTE_WARNING.MIXED_SENDER]: 'transport',
  [CTE_WARNING.MIXED_RECIPIENT]: 'transport',
  [CTE_WARNING.WEIGHT_MISSING]: 'transport',
  [CTE_WARNING.TAKER_AMBIGUOUS]: 'taker',
  [CTE_WARNING.NOTE_NOT_AUTHORIZED]: 'notes',
  [CTE_WARNING.NOTE_NOT_FOUND]: 'notes',
};

/** Ação sugerida para resolver cada aviso; a mensagem em si vem do servidor. */
export const CTE_WARNING_ACTIONS: Record<string, string> = {
  [CTE_WARNING.MIXED_SENDER]: 'Emita um CT-e por remetente ou escolha o remetente abaixo.',
  [CTE_WARNING.MIXED_RECIPIENT]: 'Emita um CT-e por destinatário ou escolha o destinatário abaixo.',
  [CTE_WARNING.WEIGHT_MISSING]: 'Informe o peso da carga abaixo.',
  [CTE_WARNING.TAKER_AMBIGUOUS]: 'Escolha abaixo quem paga o frete.',
  [CTE_WARNING.NOTE_NOT_AUTHORIZED]: 'Remova a nota ou aguarde a autorização dela.',
  [CTE_WARNING.NOTE_NOT_FOUND]: 'Remova a nota; ela não está no cadastro da organização.',
};

// ─── eventos do CT-e ────────────────────────────────────────────────────────

/** tpEvento dos eventos que a tela dispara ou lê. */
export const CTE_EVENT = {
  CANCEL: '110111',
  CORRECTION: '110110',
  DISAGREEMENT: '610110',
  DISAGREEMENT_CANCEL: '610111',
  DELIVERY_RECEIPT: '110180',
  DELIVERY_FAILURE: '110190',
  PAYMENT_LINK: '110300',
} as const;

/** TJust do cancelamento e xObs do desacordo: 15 a 255 caracteres. */
export const CTE_EVENT_TEXT_MIN = 15;
export const CTE_EVENT_TEXT_MAX = 255;

/** `corrections[]` da CC-e: 1 a 20 itens, valor até 500 caracteres, nroItemAlterado 1 a 99. */
export const CTE_CORRECTIONS_MAX = 20;
export const CTE_CORRECTION_VALUE_MAX = 500;
export const CTE_CORRECTION_ITEM_MAX = 99;

/** Status do CT-e em que os eventos valem. */
export const CTE_STATUS_AUTHORIZED = 'authorized';

/** 422 `code` da api que a tela explica em pt-BR (cancelamento e eventos). */
export const CTE_EVENT_ERRORS: Record<string, string> = {
  not_cte_issuer: 'Só o emitente do CT-e pode fazer isto. A sua empresa não é a emitente deste CT-e.',
  not_cte_taker: 'Só o tomador do serviço pode fazer isto. A sua empresa não é a tomadora deste CT-e.',
  cte_not_authorized: 'O CT-e não está autorizado. Eventos só valem para CT-e autorizado.',
  cte_protocol_missing: 'Este CT-e não tem o protocolo de autorização gravado. Consulte a situação na SEFAZ antes de tentar de novo.',
  event_invalid: 'Os dados do evento não passam na validação. Confira os campos e o tamanho dos textos.',
  event_not_found: 'O evento escolhido não existe mais neste CT-e. Atualize a página e escolha de novo.',
  event_not_authorized: 'Só dá para cancelar um evento que a SEFAZ já autorizou (status "sucesso").',
  no_certificate: 'A organização não tem certificado digital válido. Cadastre um em Certificados antes de enviar eventos.',
};

// ─── IBS/CBS e autXML ───────────────────────────────────────────────────────

/** CRT em que o grupo IBS/CBS do CT-e é aceito (regime normal). Simples e MEI recebem 422 `ibscbs_regime_unsupported`. */
export const CTE_IBSCBS_CRT = '3';
/** Ano a partir do qual o vTotDFe soma IBS e CBS (MOC, regra IBSCBS-365). */
export const CTE_TOTAL_WITH_TAXES_FROM_YEAR = 2027;
export const CTE_ERROR_IBSCBS_REGIME = 'ibscbs_regime_unsupported';
export const CTE_ERROR_AUTXML = 'autxml_invalid';

/** `autXML` do CT-e: 0 a 10. */
export const CTE_AUTHORIZED_XML_MAX = 10;

// ─── complementar, substituto e CT-e OS (F6 parte 3) ────────────────────────

/** Permissão das três emissões novas (mesma de `POST /v1.0/ctes`). */
export const PERMISSION_CTE_CREATE = 'create.ctes';

/** Modelo do CT-e OS na chave de acesso (posições 21 e 22). */
export const CTE_MODEL_OS = '67';

/** Espécie do documento, derivada de `cte_type` e `model` do registro. */
export type CteKind = 'normal' | 'complement' | 'substitute' | 'os'

export const CTE_KIND_LABELS: Record<CteKind, string> = {
  normal: 'Normal',
  complement: 'Complemento',
  substitute: 'Substituto',
  os: 'CT-e OS',
};

/**
 * Prazo do substituto (OpenAPI `substitution_window_expired`): 60 dias da emissão do original mais 168 h.
 * Entre os 60 dias e este limite a decisão é da SEFAZ (G096); depois dele o servidor recusa.
 */
export const CTE_SUBSTITUTE_SEFAZ_DAYS = 60;
export const CTE_SUBSTITUTE_LIMIT_DAYS = 67;

/**
 * Papéis de tomador que o substituto oferece: todos os participantes. Remetente, expedidor, recebedor e destinatário
 * são os do XML do original (a api devolve `expedidor` e `recebedor` no detalhe; só aparecem os que o original tem);
 * `outro` vem do cadastro de pessoas.
 */
export const CTE_SUBSTITUTE_TAKER_ROLES: readonly CteTakerRole[] = ['remetente', 'expedidor', 'recebedor', 'destinatario', 'outro'];

/** No substituto só `outro` pede pessoa do cadastro; os demais papéis são participantes do original. */
export const CTE_SUBSTITUTE_NEEDS_PERSON: ReadonlySet<CteTakerRole> = new Set(['outro']);

export const CTE_OS_SERVICES: { value: 'people' | 'values' | 'baggage'; label: string; hint: string }[] = [
  {value: 'people', label: 'Transporte de pessoas', hint: 'Fretamento de passageiros (tpServ 6)'},
  {value: 'values', label: 'Transporte de valores', hint: 'Malotes com GTV-e (tpServ 7)'},
  {value: 'baggage', label: 'Excesso de bagagem', hint: 'Bagagem acima do permitido no BP-e (tpServ 8)'},
];

/** `qCarga`: o nome da quantidade muda com o serviço. */
export const CTE_OS_QUANTITY_LABELS = {people: 'Passageiros', values: 'Malotes', baggage: 'Volumes'} as const;

/** `xDescServ`: 1 a 30 caracteres. */
export const CTE_OS_DESCRIPTION_MAX = 30;
/** `infPercurso`: até 25 UFs. */
export const CTE_OS_ROUTE_UFS_MAX = 25;
export const CTE_OS_TAF_LENGTH = 12;
export const CTE_OS_STATE_REGISTRATION_LENGTH = 25;
export const CTE_ACCESS_KEY_LENGTH = 44;

/** `tpFretamento`: 1 eventual (exige data e hora da viagem), 2 contínuo. */
export const CTE_OS_CHARTER_TYPES = [
  {value: '1', label: 'Eventual'},
  {value: '2', label: 'Contínuo'},
] as const;
export const CTE_OS_CHARTER_EVENTUAL = '1';

/** `tpComp` da GTV-e (XSD `cteTiposBasico`); 6 é "outros" e exige o nome (até 15 caracteres). */
export const CTE_OS_GTVE_COMPONENT_TYPES = [
  {value: '1', label: 'Custódia'},
  {value: '2', label: 'Embarque'},
  {value: '3', label: 'Tempo de espera'},
  {value: '4', label: 'Malote'},
  {value: '5', label: 'Ad valorem'},
  {value: '6', label: 'Outros'},
] as const;
export const CTE_OS_GTVE_COMPONENT_OTHERS = '6';
export const CTE_OS_GTVE_COMPONENT_NAME_MAX = 15;
/** Tolerância da soma dos componentes da GTV-e contra o frete (H053), em centavos. */
export const CTE_OS_GTVE_TOLERANCE_CENTS = 10;

/** `infTribFed`. */
export const CTE_OS_FEDERAL_TAXES = [
  {key: 'pis', label: 'PIS'},
  {key: 'cofins', label: 'COFINS'},
  {key: 'ir', label: 'IR'},
  {key: 'inss', label: 'INSS'},
  {key: 'csll', label: 'CSLL'},
] as const;
export type CteOsFederalTaxKey = typeof CTE_OS_FEDERAL_TAXES[number]['key']

/** O CT-e OS não tem o grupo ICMS60 (422 `os_icms_cst_not_allowed`). */
export const CTE_OS_ICMS_CST_EXCLUDED = '60';

/** Limite da lista de documentos referenciados e de GTV-e do leiaute. */
export const CTE_OS_DOCS_MAX = 500;

/** 422 `code` do complemento, do substituto e do CT-e OS, em linguagem de quem emite. A mensagem do servidor é o plano B. */
export const CTE_DOC_ERRORS: Record<string, string> = {
  original_not_found: 'O CT-e original não foi encontrado nesta organização. Abra-o pela lista de CT-e emitidos.',
  original_not_issued_by_organization: 'Só dá para complementar ou substituir um CT-e emitido pela sua própria empresa.',
  original_not_authorized: 'O CT-e original não está autorizado. Aguarde a autorização, ou emita um novo CT-e se ele foi rejeitado.',
  original_xml_unavailable: 'O XML do CT-e original ainda não está disponível. Tente de novo em instantes.',
  original_xml_invalid: 'Não foi possível ler o XML do CT-e original. Fale com o suporte informando a chave.',
  original_type_invalid: 'Só um CT-e normal ou substituto pode ser complementado ou substituído. Um complemento não aceita outro.',
  original_unsupported: 'Por enquanto só o CT-e rodoviário (modelo 57) pode ser complementado ou substituído.',
  original_too_old: 'O CT-e original tem mais de 12 meses e não pode mais ser complementado.',
  originals_mismatch: 'Os CT-e originais têm participantes, percurso ou serviço diferentes. Emita um complemento por original.',
  emitter_ie_changed: 'A inscrição estadual do emitente mudou desde o CT-e original. Fale com o suporte antes de emitir.',
  contingency_not_allowed: 'Neste modo de contingência (SVC) a SEFAZ não aceita este documento. Volte à emissão normal para emitir.',
  substitution_window_expired: 'O prazo para substituir passou: são 60 dias da emissão do original, mais 168 horas. Emita um CT-e novo e cancele o original, se ainda couber.',
  taker_change_not_requested: 'O tomador só pode ser informado quando você escolhe trocá-lo.',
  taker_required: 'Escolha o novo tomador do serviço.',
  taker_role_missing: 'O CT-e original não tem esse participante, então ele não pode ser o novo tomador. Escolha outro papel.',
  taker_unchanged: 'O novo tomador é o mesmo do CT-e original. Escolha outro, ou desligue a troca de tomador.',
  taker_base_not_in_original: 'A troca de tomador precisa de um participante que exista no CT-e original. Escolha remetente, destinatário ou outra pessoa do cadastro.',
  taker_uf_mismatch: 'A UF do novo tomador não confere com a do participante no CT-e original. Escolha outra pessoa.',
  taker_person_required: 'Escolha a pessoa do cadastro que será o tomador.',
  person_not_found: 'A pessoa escolhida não está mais no cadastro. Escolha de novo.',
  cfop_required: 'Escolha o CFOP da prestação.',
  icms_required: 'Escolha o CST do ICMS.',
  icms_rate_required: 'Informe a alíquota do ICMS; nenhuma é presumida.',
  icms_sn_not_allowed: 'O CST Simples Nacional só vale para empresas do Simples. Escolha outro CST.',
  components_exceed_total: 'Os componentes somam mais que o valor. Revise os valores.',
  freight_total_invalid: 'O valor informado é inválido. Use um valor maior que zero, com até duas casas.',
  autxml_invalid: 'Há um CPF ou CNPJ inválido, repetido ou que já faz parte do CT-e na lista de quem pode baixar o XML.',
  cte_config_required: 'Falta a configuração fiscal do CT-e. Cadastre-a em Configuração fiscal antes de emitir.',
  os_emitter_cnpj_required: 'O CT-e OS só pode ser emitido por pessoa jurídica (CNPJ).',
  os_taker_required: 'Escolha o tomador do serviço.',
  os_place_required: 'Informe origem e destino do transporte de pessoas, ou o destino do transporte de valores.',
  os_place_uf_mismatch: 'O município escolhido não pertence à UF informada. Escolha o município de novo.',
  os_road_required: 'Informe os dados do transporte rodoviário (TAF ou registro estadual).',
  os_road_not_allowed: 'O transporte de valores não leva dados do modal rodoviário.',
  os_state_registration_required: 'Transporte de pessoas dentro da mesma UF exige o número do registro estadual.',
  os_taf_required: 'Transporte de pessoas entre UFs exige o TAF (Termo de Autorização de Fretamento).',
  os_charter_required: 'Informe o tipo de fretamento do transporte de pessoas.',
  os_trip_at_required: 'O fretamento eventual exige a data e a hora da viagem.',
  os_trip_at_past: 'A data e a hora da viagem precisam ser depois do momento da emissão.',
  os_doc_refs_required: 'Informe ao menos um documento referenciado do excesso de bagagem.',
  os_doc_refs_not_allowed: 'Documentos referenciados só valem para excesso de bagagem.',
  os_bpe_key_invalid: 'A chave do BP-e é inválida ou o bilhete tem mais de 6 meses. Confira a chave de 44 dígitos.',
  os_gtve_required: 'Informe ao menos uma GTV-e do transporte de valores.',
  os_gtve_not_allowed: 'GTV-e só vale para transporte de valores.',
  os_gtve_invalid: 'Confira as GTV-e: chave repetida, com mais de 6 meses, ou componente do tipo outros sem nome.',
  os_gtve_total_mismatch: 'A soma dos componentes das GTV-e precisa fechar com o valor do frete (diferença de até R$ 0,10).',
  os_inss_required: 'Transporte de pessoas com tomador pessoa jurídica exige o valor do INSS nos tributos federais.',
  os_icms_cst_not_allowed: 'O CT-e OS não tem o CST 60. Escolha outro CST do ICMS.',
};

/** Códigos 422 que pertencem a um passo dos formulários em passos; o resto cai na revisão. */
export const CTE_CHARGE_ERROR_CODES = [
  'cfop_required', 'icms_required', 'icms_rate_required', 'icms_sn_not_allowed', 'components_exceed_total',
  'freight_total_invalid', 'autxml_invalid', 'os_gtve_total_mismatch', 'os_inss_required', 'os_icms_cst_not_allowed',
] as const;
export const CTE_TAKER_ERROR_CODES = [
  'taker_change_not_requested', 'taker_required', 'taker_role_missing', 'taker_unchanged', 'taker_base_not_in_original',
  'taker_uf_mismatch', 'taker_person_required', 'person_not_found',
] as const;
export const CTE_OS_ROUTE_ERROR_CODES = ['os_taker_required', 'os_place_required', 'os_place_uf_mismatch'] as const;
export const CTE_OS_ROAD_ERROR_CODES = [
  'os_road_required', 'os_road_not_allowed', 'os_state_registration_required', 'os_taf_required', 'os_charter_required',
  'os_trip_at_required', 'os_trip_at_past',
] as const;
export const CTE_OS_DOCS_ERROR_CODES = [
  'os_doc_refs_required', 'os_doc_refs_not_allowed', 'os_bpe_key_invalid', 'os_gtve_required', 'os_gtve_not_allowed', 'os_gtve_invalid',
] as const;
