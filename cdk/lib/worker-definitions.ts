// Workers that call SEFAZ/municipal authorities via the go-dfe-egress Lambda must
// outlive it (EGRESS_TIMEOUT_SECONDS = 120), or Lambda's async retry re-sends a call the egress
// is still running.
const SEFAZ_CALL_TIMEOUT_SECONDS = 150

/** Worker invoked directly by the API and the dispatcher (no SNS subscription). */
export const DISTRIBUTION_WORKER_ID = 'distribution'

/** Function name of the distribution worker; the API and IAM stacks build its ARN from it
 * (a plain string, not a cross-stack reference, so the Worker stack can deploy last). */
export const distributionFunctionName = (environment: string) => `${environment}-distribution-worker`

/** Retransmits documents issued in EPEC/offline contingency; invoked by its dispatcher only. */
export const CONTINGENCY_WORKER_ID = 'contingency-retransmit'

export const contingencyFunctionName = (environment: string) => `${environment}-contingency-retransmit-worker`

export type WorkerDefinition = {
  id: string
  /** Go command under worker/cmd; defaults to the shared `worker`. */
  command?: string
  name: string
  timeoutSeconds: number
  memory: number
  environment: Record<string, string>
  dynamoTables: string[]
  /** Tables this worker can mutate atomically with TransactWriteItems. */
  dynamoTransactionTables?: string[]
  /** sefaz_service values this worker handles — used as SNS subscription filter */
  sefazServices: string[]
}

export const WORKERS: WorkerDefinition[] = [
  {
    id: 'nfe-emission',
    name: 'nfe-emission-worker',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['nfes', 'nfces'],
    sefazServices: ['NFeAutorizacao'],
    environment: {},
  },
  {
    id: 'nfe-event',
    name: 'nfe-event-worker',
    timeoutSeconds: SEFAZ_CALL_TIMEOUT_SECONDS,
    memory: 128,
    // nfes/nfces needed for cancellation: worker updates the document status to cancelled
    dynamoTables: ['nfes', 'nfces', 'nfe_events', 'nfce_events'],
    sefazServices: ['RecepcaoEvento'],
    environment: {},
  },
  {
    id: 'nfe-inutilization',
    name: 'nfe-inutilization-worker',
    timeoutSeconds: SEFAZ_CALL_TIMEOUT_SECONDS,
    memory: 128,
    dynamoTables: ['nfe_events', 'nfce_events'],
    sefazServices: ['NfeInutilizacao'],
    environment: {},
  },
  {
    id: 'cte-emission',
    name: 'cte-emission-worker',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['ctes'],
    sefazServices: ['CTeRecepcaoSinc', 'CTeRecepcaoOS', 'CTeRecepcaoGTVe', 'CTeRecepcaoSimp'],
    environment: {},
  },
  {
    id: 'cte-event',
    name: 'cte-event-worker',
    timeoutSeconds: SEFAZ_CALL_TIMEOUT_SECONDS,
    memory: 128,
    dynamoTables: ['ctes', 'cte_events'],
    sefazServices: ['CTeRecepcaoEvento'],
    environment: {},
  },
  {
    id: 'mdfe-emission',
    name: 'mdfe-emission-worker',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['mdfes'],
    sefazServices: ['MDFeRecepcaoSinc'],
    environment: {},
  },
  {
    id: 'mdfe-event',
    name: 'mdfe-event-worker',
    timeoutSeconds: SEFAZ_CALL_TIMEOUT_SECONDS,
    memory: 128,
    dynamoTables: ['mdfes', 'mdfe_events'],
    sefazServices: ['MDFeRecepcaoEvento'],
    environment: {},
  },
  {
    id: 'nfse-emission',
    name: 'nfse-emission-worker',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['nfses'],
    sefazServices: ['NFSeRecepcao'],
    environment: {},
  },
  {
    id: 'nfse-event',
    name: 'nfse-event-worker',
    timeoutSeconds: SEFAZ_CALL_TIMEOUT_SECONDS,
    memory: 128,
    // nfses needed for cancellation (TE101101): worker reverts the document status
    dynamoTables: ['nfses', 'nfse_events'],
    sefazServices: ['NFSeEvento'],
    environment: {},
  },
  {
    // No sefazServices → not subscribed to the SNS event bus.
    // Invoked asynchronously by the API (user-initiated) and the dispatcher Lambda (scheduled).
    id: DISTRIBUTION_WORKER_ID,
    command: 'distribution-worker',
    name: 'distribution-worker',
    timeoutSeconds: 300,
    memory: 256,
    sefazServices: [],
    dynamoTables: [
      'organization_nfe_configs',
      'organization_cte_configs',
      'organization_mdfe_configs',
      'organization_nfse_configs',
      'organization_certificates',
      'organization_persons',
      'organizations',
      'nfe_distributions',
      'cte_distributions',
      'mdfe_distributions',
      'nfse_distributions',
      'nfes',
      'ctes',
      'mdfes',
      'nfses',
      'nfe_events',
      'cte_events',
      'mdfe_events',
      'nfse_events',
      'audit_logs',
    ],
    dynamoTransactionTables: ['organization_persons', 'audit_logs'],
    environment: {},
  },
  {
    // No sefazServices: invoked asynchronously by the contingency dispatcher (every 15 min).
    id: CONTINGENCY_WORKER_ID,
    command: 'contingency-retransmit',
    name: 'contingency-retransmit-worker',
    timeoutSeconds: SEFAZ_CALL_TIMEOUT_SECONDS,
    memory: 128,
    sefazServices: [],
    dynamoTables: ['nfes', 'nfces', 'mdfes', 'ctes', 'nfe_events', 'cte_events', 'mdfe_events'],
    environment: {},
  },
]
