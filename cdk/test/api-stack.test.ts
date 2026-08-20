import * as cdk from 'aws-cdk-lib'
import { Template } from 'aws-cdk-lib/assertions'
import { ApiStack } from '../lib/api-stack'

/** EC2's hard cap on user data, which a deploy discovers and not a review. */
const USER_DATA_LIMIT_BYTES = 16384

function synth() {
  const app = new cdk.App()
  const stack = new ApiStack(app, 'TestApiStack', {
    env: { account: '868899309401', region: 'us-east-1' },
    environment: 'prod',
    vpcId: 'vpc-0adfd86727d17445b',
    instanceProfileName: 'prod-ctech-dfe-api-v2-instance-profile',
    deploymentsBucketName: 'prod-ctech-deployments',
    logsBucketName: 'prod-ctech-application-logs',
    certificatesBucketName: 'prod-ctech-dfe-certificates',
    documentsBucketName: 'prod-ctech-dfe-documents',
    resultsQueueUrl: 'https://sqs.us-east-1.amazonaws.com/868899309401/prod-dfe-results',
    nfeEmissionTopicArn: 'arn:aws:sns:us-east-1:868899309401:prod-dfe-emission',
    distributionQueueUrl: 'https://sqs.us-east-1.amazonaws.com/868899309401/prod-dfe-distribution',
    valkeyUrlSsmPath: '/ctech-dfe/prod/valkey-url',
  })
  return Template.fromStack(stack)
}

/** The rendered user data, with unresolved tokens standing in for their values. */
function userDataText(template: Template): string {
  const resources = template.findResources('AWS::EC2::LaunchTemplate')
  const launchTemplate = Object.values(resources)[0] as any
  const encoded = launchTemplate.Properties.LaunchTemplateData.UserData['Fn::Base64']
  if (typeof encoded === 'string') return encoded
  return (encoded['Fn::Join'][1] as unknown[])
    .map((part) => (typeof part === 'string' ? part : '<<token>>'))
    .join('')
}

test('user data stays under the EC2 limit', () => {
  // Regression: the launch template deploy failed with "User data is limited to
  // 16384 bytes" once nginx.conf, the systemd unit and three shell scripts were
  // all inlined. They now ship as an S3 asset — this is what keeps them there.
  expect(Buffer.byteLength(userDataText(synth()), 'utf8')).toBeLessThan(USER_DATA_LIMIT_BYTES)
})

test('user data only fetches and runs the shared scripts', () => {
  const text = userDataText(synth())
  expect(text).toContain('ctech_run')
  expect(text).toContain('setup-base.sh')
  expect(text).toContain('setup-nginx.sh')
  // Downloaded to a file and then executed: a pipe truncated mid-transfer runs a
  // partial script and reports success.
  expect(text).not.toMatch(/aws s3 cp [^\n]*\| *bash/)
  // Nothing is written inline any more except app-static.env, service-env.sh,
  // the three nginx fragments and the CloudWatch agent config.
  const heredocs = text.match(/cat > /g) ?? []
  expect(heredocs.length).toBeLessThanOrEqual(6)
})

test('no secret value is written into the launch template', () => {
  // The instance reads secrets from SSM at service start, using its own role.
  // Anything resolved at synthesis time would sit in the launch template, which
  // is readable by anyone holding ec2:DescribeLaunchTemplateVersions.
  const text = userDataText(synth())
  expect(text).toContain("'BILLING_CLIENT_SECRET=/ctech-dfe/prod/billing/client-secret'")
  expect(text).not.toMatch(/BILLING_CLIENT_SECRET=(?!\/|\$)/)
})

test('the CloudWatch agent ships logs only, and the ASG stays at one instance', () => {
  const template = synth()

  // No `metrics` block and no custom namespace: EC2 already publishes
  // CPUUtilization and CPUCreditBalance for free.
  const userData = userDataText(template)
  expect(userData).toContain('"logs_collected"')
  expect(userData).not.toContain('CtechDfe/prod/Host')
  expect(userData).not.toContain('"metrics"')

  template.resourceCountIs('AWS::CloudWatch::Alarm', 0)
  template.hasResourceProperties('AWS::AutoScaling::AutoScalingGroup', {
    MinSize: '1',
    MaxSize: '1',
  })
})

test('the SSM agent is disabled by default', () => {
  // Deploys replace the instances through an ASG instance refresh, so nothing
  // runs over RunCommand any more and the agent's ~70 MiB is pure overhead on a
  // t4g.nano. enableSsmAgent: true is the escape hatch for a debugging shell.
  expect(userDataText(synth())).toContain('disable --now amazon-ssm-agent')
})
