import * as cdk from 'aws-cdk-lib'
import {Template} from 'aws-cdk-lib/assertions'
import {GoDfeEgressStack} from '../lib/egress-stack'
import {EGRESS_REGION, EGRESS_TIMEOUT_SECONDS, egressFunctionName} from '../lib/egress'

function build(): Template {
  const app = new cdk.App()
  const stack = new GoDfeEgressStack(app, 'TestEgress', {
    env: {account: '123456789012', region: EGRESS_REGION},
    environment: 'dev',
  })
  return Template.fromStack(stack)
}

test('egress is an arm64 provided.al2023 Lambda named {env}-go-dfe-egress', () => {
  build().hasResourceProperties('AWS::Lambda::Function', {
    FunctionName: egressFunctionName('dev'),
    Runtime: 'provided.al2023',
    Architectures: ['arm64'],
    Handler: 'bootstrap',
  })
})

test('egress timeout is the shared EGRESS_TIMEOUT_SECONDS', () => {
  const fns: any = build().findResources('AWS::Lambda::Function')
  const fn: any = Object.values(fns)[0]
  expect(fn.Properties.Timeout).toBe(EGRESS_TIMEOUT_SECONDS)
})

test('egress role has no permission beyond basic execution', () => {
  build().resourceCountIs('AWS::IAM::Policy', 0)
})
