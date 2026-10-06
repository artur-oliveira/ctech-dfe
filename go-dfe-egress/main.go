package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
)

func main() {
	lambda.Start(newHandler(dfe.Call))
}
