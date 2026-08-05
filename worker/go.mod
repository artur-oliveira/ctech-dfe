module gopkg.aoctech.app/dfe/worker

go 1.26

require (
	github.com/aws/aws-lambda-go v1.54.0
	github.com/aws/aws-sdk-go-v2 v1.43.3
	github.com/aws/aws-sdk-go-v2/config v1.32.34
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.20.58
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.63.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.101.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.106.4
	github.com/aws/aws-sdk-go-v2/service/sns v1.42.3
	github.com/aws/aws-sdk-go-v2/service/sqs v1.46.3
	github.com/google/uuid v1.6.0
)

require (
	golang.org/x/crypto v0.54.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.3 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.16 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.33 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.35 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.34 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.35 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.3 // indirect
	github.com/aws/smithy-go v1.27.6 // indirect
	gopkg.aoctech.app/dfe/go-dfe v0.0.0
)

replace gopkg.aoctech.app/dfe/go-dfe => ../go-dfe
