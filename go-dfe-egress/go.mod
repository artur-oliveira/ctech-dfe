module gopkg.aoctech.app/dfe/go-dfe-egress

go 1.27

require (
	github.com/aws/aws-lambda-go v1.55.1
	gopkg.aoctech.app/dfe/go-dfe v0.0.0
)

require (
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.3 // indirect
)

replace gopkg.aoctech.app/dfe/go-dfe => ../go-dfe
