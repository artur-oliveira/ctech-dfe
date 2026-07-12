import * as cdk from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import {HttpVersion} from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as acm from 'aws-cdk-lib/aws-certificatemanager';
import {Construct} from 'constructs';
import {Environment} from './types';

// Paths forwarded to the API origin. Everything else falls through to S3.
const API_PATH_PATTERNS = ['/v1.0/*'];

// nginx on the API instances uses proxy_read_timeout 60s — match it so
// CloudFront does not give up before the origin does.
const API_ORIGIN_READ_TIMEOUT = cdk.Duration.seconds(60);
const API_ORIGIN_KEEPALIVE_TIMEOUT = cdk.Duration.seconds(60);

interface FrontendStackProps extends cdk.StackProps {
  environment: Environment;
  certificateArn: string;
  // e.g. "app.example.com" - required when using a custom cert
  domainName?: string;
  // Public API host on the shared ALB, e.g. "dfe-api.aoctech.app".
  // Used as the API origin: ALL_VIEWER_EXCEPT_HOST_HEADER makes CloudFront send
  // this as the Host header, which is what the ALB listener rule matches on.
  apiDomainName: string;
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
  public readonly routeStore: cloudfront.KeyValueStore;

  constructor(scope: Construct, id: string, props: FrontendStackProps) {
    super(scope, id, props);

    const {environment, certificateArn, domainName, apiDomainName} = props;
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

    // One key per route emitted by the static export, written by the frontend
    // workflow right after it syncs out/ to S3 — so the route list can never
    // drift from the objects actually in the bucket.
    this.routeStore = new cloudfront.KeyValueStore(this, 'RouteStore', {
      keyValueStoreName: `${environment}-ctech-dfe-routes`,
    });

    // Rewrites clean URLs to .html files for Next.js static export:
    //   /products      to /products.html
    //   /products/     to /products.html
    //   /_next/...js   to pass through (has extension)
    //
    // Unknown routes are rewritten to /404.html here rather than through the
    // distribution's errorResponses, because those apply to every behavior and
    // would replace the API's RFC 7807 Problem JSON bodies on 403/404.
    const urlRewrite = new cloudfront.Function(this, 'UrlRewrite', {
      functionName: `${environment}-ctech-dfe-url-rewrite`,
      code: cloudfront.FunctionCode.fromInline(`
import cf from 'cloudfront';

const kvs = cf.kvs();

async function handler(event) {
  var uri = event.request.uri;
  if (uri === '/' || /\\.[^/]+$/.test(uri)) {
    return event.request;
  }
  var route = uri.endsWith('/') ? uri.slice(0, -1) : uri;
  event.request.uri = (await kvs.exists(route)) ? route + '.html' : '/404.html';
  return event.request;
}
      `),
      runtime: cloudfront.FunctionRuntime.JS_2_0,
      keyValueStore: this.routeStore,
    });

    const apiOrigin = new origins.HttpOrigin(apiDomainName, {
      protocolPolicy: cloudfront.OriginProtocolPolicy.HTTPS_ONLY,
      readTimeout: API_ORIGIN_READ_TIMEOUT,
      keepaliveTimeout: API_ORIGIN_KEEPALIVE_TIMEOUT,
    });

    // No caching and no URL rewrite: the API behavior forwards everything the
    // viewer sent (Authorization, query string, body, WebSocket upgrade) except
    // the Host header, which CloudFront replaces with apiDomainName.
    const apiBehavior: cloudfront.BehaviorOptions = {
      origin: apiOrigin,
      viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.HTTPS_ONLY,
      cachePolicy: cloudfront.CachePolicy.CACHING_DISABLED,
      originRequestPolicy: cloudfront.OriginRequestPolicy.ALL_VIEWER_EXCEPT_HOST_HEADER,
      allowedMethods: cloudfront.AllowedMethods.ALLOW_ALL,
      compress: true,
    };

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
      additionalBehaviors: Object.fromEntries(
        API_PATH_PATTERNS.map((pattern) => [pattern, apiBehavior]),
      ),
      httpVersion: HttpVersion.HTTP2_AND_3,
      defaultRootObject: 'index.html',
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
    // Read by .github/workflows/frontend.yml to publish the route manifest.
    new cdk.CfnOutput(this, 'RouteStoreArn', {
      value: this.routeStore.keyValueStoreArn,
      exportName: `${id}-route-store-arn`
    });
  }
}
