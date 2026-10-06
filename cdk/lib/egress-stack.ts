import * as cdk from 'aws-cdk-lib'
import * as iam from 'aws-cdk-lib/aws-iam'
import * as lambda from 'aws-cdk-lib/aws-lambda'
import {Construct} from 'constructs'
import path from 'node:path'
import {goLambdaCode} from './go-code'
import {EGRESS_TIMEOUT_SECONDS, egressFunctionName} from './egress'
import {Environment} from './types'

const EGRESS_DIR = path.join(__dirname, '../../go-dfe-egress')
const EGRESS_MEMORY_MB = 256

interface GoDfeEgressStackProps extends cdk.StackProps {
  environment: Environment
}

// Runs in sa-east-1 so every SEFAZ/municipal call originates from a Brazilian IP
// (some authorities drop foreign traffic). No AWS permissions beyond logging:
// it only talks to tax authorities.
export class GoDfeEgressStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: GoDfeEgressStackProps) {
    super(scope, id, props)
    const {environment} = props

    const role = new iam.Role(this, 'EgressRole', {
      roleName: `${environment}-go-dfe-egress-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole')],
    })

    new lambda.Function(this, 'EgressFunction', {
      functionName: egressFunctionName(environment),
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code: goLambdaCode(EGRESS_DIR, '.'),
      role,
      architecture: lambda.Architecture.ARM_64,
      timeout: cdk.Duration.seconds(EGRESS_TIMEOUT_SECONDS),
      memorySize: EGRESS_MEMORY_MB,
      environment: {APP_ENVIRONMENT: environment},
    })
  }
}
