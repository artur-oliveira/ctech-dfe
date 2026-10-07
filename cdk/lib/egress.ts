import {Environment} from './types'

export const EGRESS_REGION = 'sa-east-1'

// Chain: callers (150 s) > egress (120 s) > NFS-e worst case (4 x 20 s + backoff).
export const EGRESS_TIMEOUT_SECONDS = 120

export const egressFunctionName = (environment: Environment): string => `${environment}-go-dfe-egress`

// Cross-region: built from strings, never from a stack token (CloudFormation
// cannot reference a resource in another region).
export const egressFunctionArn = (environment: Environment, account: string): string =>
  `arn:aws:lambda:${EGRESS_REGION}:${account}:function:${egressFunctionName(environment)}`
