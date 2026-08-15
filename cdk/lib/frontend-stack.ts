import * as cdk from 'aws-cdk-lib';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as s3 from 'aws-cdk-lib/aws-s3';
import {createNextjsStaticFrontend} from '@aoctech/cdk';
import {Construct} from 'constructs';
import {Environment} from './types';

const API_PATH_PATTERNS = ['/v1.0/*', '/.well-known/*'];
const DOCS_PATH_PATTERNS = ['/docs', '/openapi.json', '/openapi.yaml'];
const ELEMENTS_CDN = 'https://unpkg.com';

interface FrontendStackProps extends cdk.StackProps {
  environment: Environment;
  certificateArn: string;
  domainName?: string;
  apiDomainName: string;
  authDomainName: string;
  authApiDomainName: string;
  extraConnectSrc: string[];
}

export class FrontendStack extends cdk.Stack {
  public readonly bucket: s3.Bucket;
  public readonly distribution: cloudfront.Distribution;
  public readonly routeStore: cloudfront.KeyValueStore;

  constructor(scope: Construct, id: string, props: FrontendStackProps) {
    super(scope, id, props);

    const connectSrc = [
      props.apiDomainName,
      props.authDomainName,
      props.authApiDomainName,
      ...props.extraConnectSrc,
    ].map((host) => `https://${host}`);

    const {bucket, distribution, routeStore} = createNextjsStaticFrontend(this, {
      environment: props.environment,
      serviceName: 'ctech-dfe',
      bucketName: `${props.environment}-ctech-dfe-frontend`,
      routeStoreName: `${props.environment}-ctech-dfe-routes`,
      apiDomainName: props.apiDomainName,
      apiPathPatterns: API_PATH_PATTERNS,
      connectSrc,
      domainName: props.domainName,
      certificateArn: props.domainName ? props.certificateArn : undefined,
      distributionComment: `PyDFe Frontend - ${props.environment}`,
      securityHeadersPolicyName: `${props.environment}-CtechDfe-security-headers`,
      additionalBehaviors: ({apiBehavior}) => {
        const docsHeadersPolicy = new cloudfront.ResponseHeadersPolicy(this, 'DocsSecurityHeaders', {
          responseHeadersPolicyName: `${props.environment}-CtechDfe-docs-security-headers`,
          securityHeadersBehavior: {
            contentTypeOptions: {override: true},
            frameOptions: {frameOption: cloudfront.HeadersFrameOption.DENY, override: true},
            strictTransportSecurity: {
              accessControlMaxAge: cdk.Duration.days(730),
              includeSubdomains: true,
              preload: true,
              override: true,
            },
            referrerPolicy: {
              referrerPolicy: cloudfront.HeadersReferrerPolicy.STRICT_ORIGIN_WHEN_CROSS_ORIGIN,
              override: true,
            },
            contentSecurityPolicy: {
              contentSecurityPolicy: [
                "default-src 'self'",
                "base-uri 'self'",
                "object-src 'none'",
                "frame-ancestors 'none'",
                `img-src 'self' data: ${ELEMENTS_CDN}`,
                `font-src 'self' data: ${ELEMENTS_CDN}`,
                `style-src 'self' 'unsafe-inline' ${ELEMENTS_CDN}`,
                `script-src 'self' 'unsafe-inline' ${ELEMENTS_CDN}`,
                "connect-src 'self'",
              ].join('; '),
              override: true,
            },
          },
        });
        return Object.fromEntries(
          DOCS_PATH_PATTERNS.map((pattern) => [
            pattern,
            {...apiBehavior, responseHeadersPolicy: docsHeadersPolicy},
          ]),
        );
      },
      outputExportNamePrefix: id,
    });

    this.bucket = bucket;
    this.distribution = distribution;
    this.routeStore = routeStore;
  }
}
