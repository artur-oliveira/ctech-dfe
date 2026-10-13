module gopkg.aoctech.app/dfe/worker

go 1.27

require (
	github.com/aws/aws-lambda-go v1.55.0
	github.com/aws/aws-sdk-go-v2 v1.47.1
	github.com/aws/aws-sdk-go-v2/config v1.33.5
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.21.5
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.69.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.104.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.109.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.2
	github.com/oklog/ulid/v2 v2.1.2
	gopkg.aoctech.app/dfe/go-dfe v0.0.0-00010101000000-000000000000
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.41.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.0 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.3 // indirect
)

replace gopkg.aoctech.app/dfe/go-dfe => ../go-dfe
