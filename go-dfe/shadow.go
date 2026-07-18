package dfe

import (
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
)

// ShadowCompare calls Call for req if Implements(req.DocType, req.Service),
// comparing its result against a caller's already-obtained py-dfe Lambda
// response — used during the migration's shadow-mode window (see
// docs/plans/2026-07-17-go-dfe-migration.md, "Gate de validação por fase").
// It never returns an error and never affects the caller in any way beyond
// a log line; safe to call unconditionally from any seam (worker or api)
// for any request, implemented or not.
//
// Exported here (not duplicated per caller) so worker and api — two
// separate Go modules — share one comparison implementation; each caller
// only needs its own small payload-shape-to-Request adapter.
func ShadowCompare(ctx context.Context, req Request, pyDfeStatusCode int, pyDfeBody string) {
	if !Implements(req.DocType, req.Service) {
		return
	}

	resp, err := Call(ctx, req)
	if err != nil {
		slog.Warn("shadow go-dfe call errored", "doc_type", req.DocType, "service", req.Service, "err", err)
		return
	}

	if resp.StatusCode != pyDfeStatusCode {
		slog.Warn("shadow mode divergence: status code",
			"doc_type", req.DocType, "service", req.Service,
			"py_dfe_status", pyDfeStatusCode, "go_dfe_status", resp.StatusCode)
		return
	}

	if !jsonEqual(pyDfeBody, resp.Body) {
		slog.Warn("shadow mode divergence: response body",
			"doc_type", req.DocType, "service", req.Service)
		return
	}

	slog.Info("shadow mode parity: go-dfe matches py-dfe", "doc_type", req.DocType, "service", req.Service)
}

// jsonEqual reports whether a and b decode to structurally equal JSON,
// ignoring key order and formatting differences between Python's json.dumps
// and Go's json.Marshal — a byte-for-byte comparison would be pure noise.
func jsonEqual(a, b string) bool {
	var av, bv any
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}
