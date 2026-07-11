import * as cdk from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import {HttpVersion} from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as acm from 'aws-cdk-lib/aws-certificatemanager';
import {Construct} from 'constructs';
import {Environment} from './types';

interface FrontendStackProps extends cdk.StackProps {
  environment: Environment;
  certificateArn: string;
  // e.g. "app.example.com" - required when using a custom cert
  domainName?: string;
}

/**
 * Bucket + CloudFront must live in the same stack because
 * S3BucketOrigin.withOriginAccessControl() writes a bucket policy that
 * references the distribution ARN - splitting them across stacks creates
 * a CDK dependency cycle.
 */
export class FrontendStack extends cdk.Stack {
  public readonly bucket: s3.Bucket;
  public readonly distribution: cloudfront.Distribution;
  
  constructor(scope: Construct, id: string, props: FrontendStackProps) {
    super(scope, id, props);
    
    const {environment, certificateArn, domainName} = props;
    const isProduction = environment === 'prod';
    
    this.bucket = new s3.Bucket(this, 'Bucket', {
      bucketName: `${environment}-ctech-dfe-frontend`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: isProduction ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
    });
    
    const oac = new cloudfront.S3OriginAccessControl(this, 'OAC', {
      originAccessControlName: `${environment}-ctech-dfe-oac`,
    });
    
    // Rewrites clean URLs to .html files for Next.js static export:
    //   /products      to /products.html
    //   /products/     to /products/index.html
    //   /_next/...js   to pass through (has extension)
    const urlRewrite = new cloudfront.Function(this, 'UrlRewrite', {
      functionName: `${environment}-ctech-dfe-url-rewrite`,
      code: cloudfront.FunctionCode.fromInline(`
function handler(event) {
  var uri = event.request.uri;
  if (uri !== '/' && !/\\.[^/]+$/.test(uri)) {
    event.request.uri = uri.endsWith('/') ? uri + 'index.html' : uri + '.html';
  }
  return event.request;
}
      `),
      runtime: cloudfront.FunctionRuntime.JS_2_0,
    });
    
    this.distribution = new cloudfront.Distribution(this, 'Distribution', {
      comment: `PyDFe Frontend - ${environment}`,
      defaultBehavior: {
        origin: origins.S3BucketOrigin.withOriginAccessControl(this.bucket, {
          originAccessControl: oac,
        }),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
        allowedMethods: cloudfront.AllowedMethods.ALLOW_GET_HEAD_OPTIONS,
        compress: true,
        functionAssociations: [{
          function: urlRewrite,
          eventType: cloudfront.FunctionEventType.VIEWER_REQUEST,
        }],
      },
      httpVersion: HttpVersion.HTTP2_AND_3,
      defaultRootObject: 'index.html',
      errorResponses: [
        {httpStatus: 403, responseHttpStatus: 404, responsePagePath: '/404.html', ttl: cdk.Duration.seconds(0)},
        {httpStatus: 404, responseHttpStatus: 404, responsePagePath: '/404.html', ttl: cdk.Duration.seconds(0)},
      ],
      certificate: domainName
        ? acm.Certificate.fromCertificateArn(this, 'Cert', certificateArn)
        : undefined,
      domainNames: domainName ? [domainName] : undefined,
      priceClass: cloudfront.PriceClass.PRICE_CLASS_100,
      minimumProtocolVersion: cloudfront.SecurityPolicyProtocol.TLS_V1_2_2021,
    });
    
    new cdk.CfnOutput(this, 'BucketName', {value: this.bucket.bucketName, exportName: `${id}-bucket-name`});
    new cdk.CfnOutput(this, 'DistributionId', {value: this.distribution.distributionId, exportName: `${id}-dist-id`});
    new cdk.CfnOutput(this, 'DistributionDomain', {
      value: this.distribution.distributionDomainName,
      exportName: `${id}-dist-domain`
    });
  }
}
