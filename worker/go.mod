module gopkg.aoctech.app/dfe/worker

go 1.27.4

require (
	github.com/aws/aws-lambda-go v1.55.1
	github.com/aws/aws-sdk-go-v2 v1.48.0
	github.com/aws/aws-sdk-go-v2/config v1.33.19
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.21.23
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.74.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.117.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.115.0
	github.com/aws/aws-sdk-go-v2/service/sns v1.48.0
	github.com/aws/aws-sdk-go-v2/service/ssm v1.83.0
	github.com/oklog/ulid/v2 v2.1.2
	gopkg.aoctech.app/api-commons v1.43.0
	gopkg.aoctech.app/dfe/go-dfe v0.0.0-20261118002157-570c8ad87c1e
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.27 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.21.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.48.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.53.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.52.0 // indirect
	github.com/aws/smithy-go v1.28.9 // indirect
	github.com/gofiber/fiber/v3 v3.7.0 // indirect
	github.com/gofiber/schema v1.9.5 // indirect
	github.com/gofiber/utils/v2 v2.6.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.4 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/molecule-man/go-brrr v1.3.1 // indirect
	github.com/philhofer/fwd v1.2.1 // indirect
	github.com/tinylib/msgp v1.6.5 // indirect
	github.com/valkey-io/valkey-go v1.0.80 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.79.0 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/net v0.63.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.4 // indirect
)

replace gopkg.aoctech.app/dfe/go-dfe => ../go-dfe
