import type {CiotEnvironment, CiotPendingOperation, CiotStatus} from '@/lib/types/api';

/** Permissões do recurso `ciots` (api: `middleware.PermChecker`). */
export const PERMISSION_CIOT_CREATE = 'create.ciots';
export const PERMISSION_CIOT_LIST = 'list.ciots';
export const PERMISSION_CIOT_GET = 'get.ciots';
export const PERMISSION_CIOT_UPDATE = 'update.ciots';

/** Prazo de cancelamento: até 24 h depois do início da viagem (regra B35 da ANTT). */
export const CIOT_CANCEL_WINDOW_HOURS = 24;
export const CIOT_CANCEL_REASON_MAX_LENGTH = 500;

/** Limites do contrato (ciots.yaml). */
export const CIOT_MAX_VEHICLES = 5;
export const CIOT_FLEET_MAX_PLATES = 7;
export const CIOT_RNTRC_MIN_DIGITS = 8;
export const CIOT_RNTRC_DIGITS = 9;
export const CIOT_NATURE_DIGITS = 4;
export const CIOT_CODE_LENGTH = 16;
export const MDFE_ACCESS_KEY_LENGTH = 44;

/** Código do problema quando a tabela do piso ainda não foi verificada contra a ANTT. */
export const PROBLEM_CIOT_FLOOR_UNVERIFIED = 'ciot_floor_unverified';
export const PROBLEM_CIOT_VALIDATION = 'ciot_validation';

export const OPERATION_LOTACAO = 1;
export const OPERATION_FRACIONADA = 2;
export const OPERATION_TAC_AGREGADO = 3;

export const CIOT_OPERATION_TYPES = [
  {value: String(OPERATION_LOTACAO), label: 'Carga lotação'},
  {value: String(OPERATION_FRACIONADA), label: 'Carga fracionada'},
  {value: String(OPERATION_TAC_AGREGADO), label: 'TAC-Agregado'},
];

export const CIOT_ENVIRONMENTS: { value: CiotEnvironment; label: string }[] = [
  {value: 'prod', label: 'Produção'},
  {value: 'hom', label: 'Homologação'},
];

/** Tipos de carga da ANTT (1 a 12, CodigoTipoCarga; mesma numeração do Anexo II do piso). */
export const CIOT_CARGO_TYPES = [
  'Granel sólido', 'Granel líquido', 'Frigorificada', 'Conteinerizada', 'Carga geral', 'Neogranel',
  'Perigosa (granel sólido)', 'Perigosa (granel líquido)', 'Perigosa (frigorificada)',
  'Perigosa (conteinerizada)', 'Perigosa (carga geral)', 'Granel pressurizado',
].map((label, i) => ({value: String(i + 1), label: `${i + 1}. ${label}`}));

export const PAYMENT_PIX = 6;
export const PAYMENT_BANK_TYPES = [2, 3, 4];

/** Meios de pagamento do frete (spec, `InfPagamento.TipoPagamento`). */
export const CIOT_PAYMENT_TYPES = [
  {value: '1', label: 'Cartão pré-pago de instituição de pagamento'},
  {value: '2', label: 'Conta corrente'},
  {value: '3', label: 'Conta poupança'},
  {value: '4', label: 'Conta de pagamento'},
  {value: '5', label: 'Outros'},
  {value: String(PAYMENT_PIX), label: 'Pix'},
];

export const CIOT_STATUS_LABELS: Record<CiotStatus, string> = {
  pending: 'Em processamento',
  declared: 'Declarado',
  rectified: 'Retificado',
  cancelled: 'Cancelado',
  closed: 'Encerrado',
  rejected: 'Rejeitado',
  failed: 'Falhou',
};

/** Paleta semântica fixa de status (mesma dos demais documentos): verde, vermelho, âmbar, cinza, azul. */
export const CIOT_STATUS_CLASSES: Record<CiotStatus, string> = {
  pending: 'bg-amber-50 text-amber-700',
  declared: 'bg-green-50 text-green-700',
  rectified: 'bg-blue-50 text-blue-700',
  cancelled: 'bg-gray-100 text-gray-600',
  closed: 'bg-gray-100 text-gray-700',
  rejected: 'bg-red-50 text-red-700',
  failed: 'bg-red-50 text-red-700',
};

export const CIOT_PENDING_LABELS: Record<CiotPendingOperation, string> = {
  declare: 'Declaração em andamento',
  cancel: 'Cancelamento em andamento',
  rectify: 'Retificação em andamento',
  close: 'Encerramento em andamento',
};

/** Filtros da lista (a api ainda não filtra por status; o filtro age sobre a página carregada). */
export const CIOT_STATUS_FILTERS: { value: '' | CiotStatus; label: string }[] = [
  {value: '', label: 'Todos'},
  ...(['pending', 'declared', 'rectified', 'cancelled', 'closed', 'rejected', 'failed'] as CiotStatus[])
    .map((s) => ({value: s, label: CIOT_STATUS_LABELS[s]})),
];

/** Nome da tabela na mensagem `dfe_result` do WebSocket. */
export const CIOT_TABLE_NAME = 'ciots';
export const CIOT_SK_PREFIX = 'CIOT_';
