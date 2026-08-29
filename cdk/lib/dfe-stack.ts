import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as path from 'path';
import {Construct} from 'constructs';
import {Environment} from './types';
import {execSync} from "node:child_process";

// Used to exclude files from the Lambda FUNCTION code asset hash.
// requirements.txt is intentionally NOT here — for layer assets it is the
// only file that distinguishes one layer from another, so excluding it would
// cause CDK to compute the same hash for all layers and bundle them once.
export const FUNCTION_EXCLUDE = [
  'tests', 'scripts', '*.toml', '*.md', 'requirements.txt', '.venv',
  '.pytest_cache', '.ruff_cache', '**/__pycache__',
];

// Used for layer assets: same list but keeps requirements.txt so CDK hashes
// the layer source correctly and rebuilds when dependencies change.
export const LAYER_EXCLUDE = [
  'tests', 'scripts', '*.toml', '*.md', '.venv',
  '.pytest_cache', '.ruff_cache', '**/__pycache__',
];

const PYDFE_LAYER_DIR = path.join(__dirname, '../../py-dfe/layer');

export function layerBundling(requirementsDir: string): cdk.BundlingOptions {
  const useLocal = process.env.CDK_LOCAL_BUNDLING === '1';

  // Docker is the default so compiled wheels match Lambda's arm64 runtime.
  // Local bundling is available only when the host already uses Python 3.14.
  const PIP_CMD = 'pip install -r requirements.txt -t {dest} --no-cache-dir';

  return {
    image: lambda.Runtime.PYTHON_3_14.bundlingImage,
    platform: 'linux/arm64',
    user: 'root',
    command: ['bash', 'build.sh'],
    local: useLocal ? {
      tryBundle(outputDir: string): boolean {
        const dest = path.join(outputDir, 'python');
        execSync(
          PIP_CMD.replace('{dest}', `"${dest}"`),
          {stdio: 'inherit', cwd: requirementsDir},
        );
        return true;
      },
    } : undefined,
  };
}

interface DfeStackProps extends cdk.StackProps {
  environment: Environment;
}

export class DfeStack extends cdk.Stack {
  public readonly dfeFunction: lambda.Function;
  public readonly role: iam.Role;

  constructor(scope: Construct, id: string, props: DfeStackProps) {
    super(scope, id, props);

    const {environment} = props;

    // Layer defined inline — no cross-stack reference, no export conflicts.
    const pyDfeLayer = new lambda.LayerVersion(this, 'PyDfeLayer', {
      layerVersionName: `${environment}-pydfe-core-layer`,
      description: 'Core libs (cryptography, lxml, httpx, signxml)',
      code: lambda.Code.fromAsset(
        PYDFE_LAYER_DIR,
        {bundling: layerBundling(PYDFE_LAYER_DIR), exclude: LAYER_EXCLUDE},
      ),
      compatibleRuntimes: [lambda.Runtime.PYTHON_3_14],
      compatibleArchitectures: [lambda.Architecture.ARM_64],
      removalPolicy: environment === 'prod' ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
    });

    this.role = new iam.Role(this, 'DfeLambdaRole', {
      roleName: `${environment}-py-dfe-function-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    });

    this.dfeFunction = new lambda.Function(this, 'DfeFunction', {
      functionName: `${environment}-py-dfe`,
      runtime: lambda.Runtime.PYTHON_3_14,
      handler: 'py_dfe.handler.handler',
      code: lambda.Code.fromAsset('../py-dfe', {exclude: FUNCTION_EXCLUDE}),
      role: this.role,
      layers: [pyDfeLayer],
      architecture: lambda.Architecture.ARM_64,
      timeout: cdk.Duration.seconds(30),
      memorySize: 512,
      environment: {
        APP_ENVIRONMENT: environment,
      },
    });

    new cdk.CfnOutput(this, 'DfeFunctionName', {value: this.dfeFunction.functionName});
    new cdk.CfnOutput(this, 'DfeFunctionArn', {value: this.dfeFunction.functionArn});
    new cdk.CfnOutput(this, 'DfeRoleArn', {value: this.role.roleArn});
  }
}
