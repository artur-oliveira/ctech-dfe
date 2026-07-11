export type WorkerDefinition = {
  id: string
  name: string
  queueName: string
  timeoutSeconds: number
  memory: number
  environment: Record<string, string>
  dynamoTables: string[]
  /** sefaz_service values this worker handles — used as SNS subscription filter */
  sefazServices: string[]
}

export const WORKERS: WorkerDefinition[] = [
  {
    id: 'nfe-emission',
    name: 'nfe-emission-worker',
    queueName: 'nfe-emission',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['nfes', 'nfces'],
    sefazServices: ['NFeAutorizacao'],
    environment: {},
  },
  {
    id: 'nfe-event',
    name: 'nfe-event-worker',
    queueName: 'nfe-event',
    timeoutSeconds: 60,
    memory: 128,
    // nfes/nfces needed for cancellation: worker updates the document status to cancelled
    dynamoTables: ['nfes', 'nfces', 'nfe_events', 'nfce_events'],
    sefazServices: ['RecepcaoEvento'],
    environment: {},
  },
  {
    id: 'nfe-inutilization',
    name: 'nfe-inutilization-worker',
    queueName: 'nfe-inutilization',
    timeoutSeconds: 60,
    memory: 128,
    dynamoTables: ['nfe_events', 'nfce_events'],
    sefazServices: ['NfeInutilizacao'],
    environment: {},
  },
  {
    id: 'cte-emission',
    name: 'cte-emission-worker',
    queueName: 'cte-emission',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['ctes'],
    sefazServices: ['CTeRecepcaoSinc', 'CTeRecepcaoOS', 'CTeRecepcaoGTVe', 'CTeRecepcaoSimp'],
    environment: {},
  },
  {
    id: 'cte-event',
    name: 'cte-event-worker',
    queueName: 'cte-event',
    timeoutSeconds: 60,
    memory: 128,
    dynamoTables: ['ctes', 'cte_events'],
    sefazServices: ['CTeRecepcaoEvento'],
    environment: {},
  },
  {
    id: 'mdfe-emission',
    name: 'mdfe-emission-worker',
    queueName: 'mdfe-emission',
    timeoutSeconds: 300,
    memory: 128,
    dynamoTables: ['mdfes'],
    sefazServices: ['MDFeRecepcaoSinc'],
    environment: {},
  },
  {
    id: 'mdfe-event',
    name: 'mdfe-event-worker',
    queueName: 'mdfe-event',
    timeoutSeconds: 60,
    memory: 128,
    dynamoTables: ['mdfes', 'mdfe_events'],
    sefazServices: ['MDFeRecepcaoEvento'],
    environment: {},
  },
  {
    // No sefazServices → queue is not subscribed to the SNS event bus.
    // Triggered directly via SQS by the API (user-initiated) and the dispatcher Lambda (scheduled).
    id: 'distribution',
    name: 'distribution-worker',
    queueName: 'distribution',
    timeoutSeconds: 300,
    memory: 256,
    sefazServices: [],
    dynamoTables: [
      'organization_nfe_configs',
      'organization_cte_configs',
      'organization_mdfe_configs',
      'organization_certificates',
      'organization_persons',
      'organizations',
      'nfe_distributions',
      'cte_distributions',
      'mdfe_distributions',
      'nfes',
      'ctes',
      'mdfes',
      'nfe_events',
      'cte_events',
      'mdfe_events',
    ],
    environment: {},
  },
]