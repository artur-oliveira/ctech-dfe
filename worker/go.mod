module gopkg.aoctech.app/dfe/worker

go 1.27

require (
	github.com/aws/aws-lambda-go v1.55.0
	github.com/aws/aws-sdk-go-v2 v1.47.3
	github.com/aws/aws-sdk-go-v2/config v1.33.5
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.21.5
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.69.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.104.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.109.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.5
	github.com/oklog/ulid/v2 v2.1.2
	gopkg.aoctech.app/api-commons v1.39.0
	gopkg.aoctech.app/dfe/go-dfe v0.0.0-00010101000000-000000000000
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.6 // indirect
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
	github.com/aws/smithy-go v1.28.5 // indirect
	github.com/gofiber/fiber/v3 v3.5.0 // indirect
	github.com/gofiber/schema v1.8.7 // indirect
	github.com/gofiber/utils/v2 v2.5.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/molecule-man/go-brrr v1.1.0 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/valkey-io/valkey-go v1.0.78 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.74.0 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.3 // indirect
)

replace gopkg.aoctech.app/dfe/go-dfe => ../go-dfe
