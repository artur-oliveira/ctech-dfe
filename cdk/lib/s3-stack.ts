import * as cdk from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import {Construct} from 'constructs';
import {Environment} from './types';

const AUXILIARY_DOCUMENT_CACHE_TAG_KEY = 'cache';
const AUXILIARY_DOCUMENT_CACHE_TAG_VALUE = 'auxiliary-document';
const AUXILIARY_DOCUMENT_CACHE_DAYS = 30;
const AUXILIARY_DOCUMENT_NONCURRENT_DAYS = 1;

interface S3StackProps extends cdk.StackProps {
  bucketPrefix: string;
  environment: Environment;
}

export class S3Stack extends cdk.Stack {
  public readonly certificatesBucketName: string;
  public readonly documentsBucketName: string;

  public readonly certificatesBucketArn: string;
  public readonly documentsBucketArn: string;

  constructor(scope: Construct, id: string, props: S3StackProps) {
    super(scope, id, props);

    const {bucketPrefix, environment} = props;
    const isProduction = environment === 'prod';

    const certificatesBucket = new s3.Bucket(this, 'CertificatesBucket', {
      bucketName: `${bucketPrefix}-certificates`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: isProduction ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
      lifecycleRules: [
        {
          expiration: cdk.Duration.days(90),
          prefix: 'temp/',
        },
      ],
    });

    const documentsBucket = new s3.Bucket(this, 'DocumentsBucket', {
      bucketName: `${bucketPrefix}-documents`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: isProduction ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
      lifecycleRules: [
        {
          id: 'AuxiliaryDocumentCache',
          expiration: cdk.Duration.days(AUXILIARY_DOCUMENT_CACHE_DAYS),
          noncurrentVersionExpiration: cdk.Duration.days(AUXILIARY_DOCUMENT_NONCURRENT_DAYS),
          tagFilters: {
            [AUXILIARY_DOCUMENT_CACHE_TAG_KEY]: AUXILIARY_DOCUMENT_CACHE_TAG_VALUE,
          },
        },
        {
          transitions: [
            {
              storageClass: s3.StorageClass.INFREQUENT_ACCESS,
              transitionAfter: cdk.Duration.days(90),
            },
          ],
        },
      ],
    });

    this.certificatesBucketName = certificatesBucket.bucketName;
    this.documentsBucketName = documentsBucket.bucketName;

    this.certificatesBucketArn = certificatesBucket.bucketArn;
    this.documentsBucketArn = documentsBucket.bucketArn;

    new cdk.CfnOutput(this, 'CertificatesBucketName', {
      value: this.certificatesBucketName,
      exportName: `${id}-certificates-bucket`,
    });

    new cdk.CfnOutput(this, 'DocumentsBucketName', {
      value: this.documentsBucketName,
      exportName: `${id}-documents-bucket`,
    });
  }
}
