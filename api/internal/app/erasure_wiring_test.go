package app

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/fx"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Dark launch: without ERASURE_QUEUE_URL nothing erasure-related runs, not
// even the write lock (its table may not exist yet).
func TestErasureIsOffWithoutAQueue(t *testing.T) {
	if s := newErasureStore(&awsclient.Clients{}, &config.Config{}); s != nil {
		t.Fatal("store built without ERASURE_QUEUE_URL")
	}
	if blockedOf(nil) != nil {
		t.Fatal("a nil store must leave the lock off")
	}
	if svc := newErasureService(nil, nil, nil, nil, nil, &awsclient.Clients{}, &config.Config{}, nil); svc != nil {
		t.Fatal("purge service built without a store")
	}
}

func TestErasureIsOnWithAQueue(t *testing.T) {
	clients := &awsclient.Clients{DynamoDB: dynamodb.New(dynamodb.Options{Region: "us-east-1"})}
	s := newErasureStore(clients, &config.Config{ErasureQueueURL: "https://sqs.example/q", TablePrefix: "dev_dfe"})
	if s == nil || blockedOf(s) == nil {
		t.Fatal("store or lock missing with ERASURE_QUEUE_URL set")
	}
}

// The purge must not borrow the reach credential (ruling R11): with only
// ACCOUNT_CLIENT_* set, the subject lookup is unconfigured and fails.
func TestErasureUsesItsOwnCredential(t *testing.T) {
	reachOnly := &config.Config{CtechURL: "https://accounts.example", AccountClientID: "dfe-reach", AccountClientSecret: "s"}
	if newErasureSubject(reachOnly, nil) != nil {
		t.Fatal("the subject lookup was built from ACCOUNT_CLIENT_*")
	}
	own := &config.Config{CtechURL: "https://accounts.example", ErasureClientID: "dfe-erasure", ErasureClientSecret: "s"}
	if newErasureSubject(own, nil) == nil {
		t.Fatal("the subject lookup ignored ERASURE_CLIENT_*")
	}
}

func TestModuleGraphResolves(t *testing.T) {
	if err := fx.ValidateApp(Module); err != nil {
		t.Fatal(err)
	}
}
