import * as cdk from 'aws-cdk-lib'
import { Template } from 'aws-cdk-lib/assertions'
import { FrontendStack } from '../lib/frontend-stack'

function synth() {
  const app = new cdk.App()
  const stack = new FrontendStack(app, 'TestFrontendStack', {
    environment: 'dev',
    certificateArn: 'arn:aws:acm:us-east-1:000000000000:certificate/test',
    domainName: 'app.example.com',
    apiDomainName: 'api.example.com',
    authDomainName: 'accounts.example.com',
    authApiDomainName: 'accounts-api.example.com',
    extraConnectSrc: ['viacep.com.br'],
  })
  return Template.fromStack(stack)
}

function cspOf(template: Template, name: string): string {
  const policies = template.findResources('AWS::CloudFront::ResponseHeadersPolicy')
  const policy = Object.values(policies).find(
    (p: any) => p.Properties.ResponseHeadersPolicyConfig.Name === name
  ) as any
  expect(policy).toBeDefined()
  return policy.Properties.ResponseHeadersPolicyConfig.SecurityHeadersConfig
    .ContentSecurityPolicy.ContentSecurityPolicy
}

test('the OpenAPI spec and the docs page are forwarded to the API origin', () => {
  const json = synth().toJSON()
  const distribution = Object.values(json.Resources).find(
    (r: any) => r.Type === 'AWS::CloudFront::Distribution'
  ) as any
  const patterns = distribution.Properties.DistributionConfig.CacheBehaviors.map(
    (b: any) => b.PathPattern
  )
  expect(patterns).toEqual(expect.arrayContaining(['/v1.0/*', '/docs', '/openapi.json', '/openapi.yaml']))
})

// The CDN exception exists for Stoplight Elements only; leaking it into the app
// CSP would let any page on the domain execute third-party script.
test('unpkg is allowed on the docs behavior and nowhere else', () => {
  const template = synth()
  expect(cspOf(template, 'dev-CtechDfe-docs-security-headers')).toContain('https://unpkg.com')
  expect(cspOf(template, 'dev-CtechDfe-security-headers')).not.toContain('unpkg')
})
