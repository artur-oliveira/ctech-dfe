import * as cdk from 'aws-cdk-lib'
import { Template } from 'aws-cdk-lib/assertions'
import { S3Stack } from '../lib/s3-stack'

function synth(environment: 'dev' | 'stage' | 'prod') {
  const app = new cdk.App()
  const stack = new S3Stack(app, `TestS3Stack-${environment}`, {
    environment,
    bucketPrefix: `${environment}-ctech-dfe`,
  })
  return Template.fromStack(stack)
}

test('prod buckets are RETAIN, dev buckets are DESTROY', () => {
  const prod = synth('prod')
  prod.hasResource('AWS::S3::Bucket', { DeletionPolicy: 'Retain' })

  const dev = synth('dev')
  dev.hasResource('AWS::S3::Bucket', { DeletionPolicy: 'Delete' })
})

test('every prod bucket resource is RETAIN (none left as DESTROY)', () => {
  const prod = synth('prod')
  const json = prod.toJSON()
  const buckets = Object.values(json.Resources).filter((r: any) => r.Type === 'AWS::S3::Bucket') as any[]
  expect(buckets.length).toBe(2)
  for (const b of buckets) {
    expect(b.DeletionPolicy).toBe('Retain')
  }
})

test('documents bucket has a Standard-IA lifecycle transition', () => {
  const prod = synth('prod')
  const json = prod.toJSON()
  const docsBucket = Object.values(json.Resources).find(
    (r: any) => r.Type === 'AWS::S3::Bucket' && r.Properties?.BucketName === 'prod-ctech-dfe-documents'
  ) as any
  expect(docsBucket).toBeDefined()
  const rules = docsBucket.Properties.LifecycleConfiguration.Rules
  const transitionRule = rules.find((r: any) => r.Transitions?.some((t: any) => t.StorageClass === 'STANDARD_IA'))
  expect(transitionRule).toBeDefined()
  expect(transitionRule.Transitions[0].TransitionInDays).toBe(90)
})
