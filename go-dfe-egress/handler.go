// Package main is the go-dfe-egress Lambda: it runs dfe.Call in sa-east-1 so
// every call to SEFAZ / a municipal authority originates from a Brazilian IP.
// It touches no AWS service and never logs the request body (it carries the
// PFX and its password).
package main

import (
	"context"
	"log/slog"
	"time"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
)

const (
	envProdLong = "producao"
	envHomLong  = "homologacao"
	envProd     = "prod"
	envHom      = "hom"

	logMsgCall    = "egress call"
	logFieldType  = "doc_type"
	logFieldSvc   = "service"
	logFieldUF    = "uf"
	logFieldCode  = "status_code"
	logFieldMS    = "duration_ms"
	logFieldError = "error"
)

// callFunc is dfe.Call; injectable for tests.
type callFunc func(context.Context, dfe.Request) (dfe.Response, error)

// normalizeEnvironment accepts the long forms worker/api always sent
// ("producao"/"homologacao") and returns the form dfe.Call expects.
func normalizeEnvironment(env string) string {
	switch env {
	case envProdLong:
		return envProd
	case envHomLong:
		return envHom
	default:
		return env
	}
}

func newHandler(call callFunc) func(context.Context, dfe.Request) (dfe.Response, error) {
	return func(ctx context.Context, req dfe.Request) (dfe.Response, error) {
		req.Environment = normalizeEnvironment(req.Environment)
		start := time.Now()
		resp, err := call(ctx, req)
		attrs := []any{
			logFieldType, req.DocType, logFieldSvc, req.Service, logFieldUF, req.UF,
			logFieldCode, resp.StatusCode, logFieldMS, time.Since(start).Milliseconds(),
		}
		if err != nil {
			attrs = append(attrs, logFieldError, err.Error())
		}
		slog.Info(logMsgCall, attrs...)
		return resp, err
	}
}
