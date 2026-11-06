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
