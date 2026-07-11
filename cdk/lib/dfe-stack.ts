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
  'tests', 'scripts', '*.toml', '*.md', 'requirements.txt',
  '.pytest_cache', '.ruff_cache', '**/__pycache__',
];

// Used for layer assets: same list but keeps requirements.txt so CDK hashes
// the layer source correctly and rebuilds when dependencies change.
export const LAYER_EXCLUDE = [
  'tests', 'scripts', '*.toml', '*.md',
  '.pytest_cache', '.ruff_cache', '**/__pycache__',
];

const PYDFE_LAYER_DIR = path.join(__dirname, '../../py-dfe/layer');

export function layerBundling(requirementsDir: string): cdk.BundlingOptions {
  const useLocal = process.env.CDK_LOCAL_BUNDLING === '1';

  // build.sh (in the layer dir) does pip install AND copies the native
  // pango/cairo/fontconfig .so deps + fonts that WeasyPrint dlopen()s at
  // runtime. pip alone ships none of those → libpango load failure on Lambda.
  // • Docker path (default on CI): sam/build-python3.14 = Amazon Linux 2023,
  //   arm64, root → dnf installs the pango stack, cp314/aarch64 wheels.
  // • Local path (CDK_LOCAL_BUNDLING=1): ONLY pip (no dnf on ubuntu). Native
  //   libs must already be on the host. Use only when local Python is 3.14.
  const PIP_CMD = 'pip install -r requirements.txt -t {dest} --no-cache-dir';

  return {
    image: lambda.Runtime.PYTHON_3_14.bundlingImage,
    platform: 'linux/arm64',
    // build.sh runs `dnf install` for the native pango stack — needs root.
    // CDK defaults the container to the host uid (-u 1001) → override to root.
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
      // WeasyPrint PDF rendering needs more than 128 MB; pango/cairo + fonts.
      memorySize: 512,
      environment: {
        APP_ENVIRONMENT: environment,
        // Native libs land in /opt/lib (already on Lambda's default
        // LD_LIBRARY_PATH). Fontconfig: config + fonts in /opt/fonts, cache
        // in /tmp (only writable path on Lambda).
        FONTCONFIG_PATH: '/opt/fonts',
        XDG_CACHE_HOME: '/tmp',
      },
    });
    
    new cdk.CfnOutput(this, 'DfeFunctionName', {value: this.dfeFunction.functionName});
    new cdk.CfnOutput(this, 'DfeFunctionArn', {value: this.dfeFunction.functionArn});
    new cdk.CfnOutput(this, 'DfeRoleArn', {value: this.role.roleArn});
  }
}
