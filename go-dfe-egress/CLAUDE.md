# CLAUDE.md — go-dfe-egress

Thin Lambda (`provided.al2023`, arm64) deployed in **sa-east-1** that executes `dfe.Call` from `../go-dfe`. Every call to
SEFAZ or a municipal NFS-e authority leaves from a Brazilian IP (Teresina drops traffic from outside Brazil).

**Before any task:** read `../docs/specs/2026-10-06-go-dfe-egress-design.md` and `../go-dfe/CLAUDE.md`.

## Contract

Input: `dfe.Request` as JSON (`../go-dfe/request.go`). Output: `dfe.Response`. An error from `dfe.Call` (unsupported
operation) becomes a Lambda function error, which `worker`/`api` treat as a failed call.

## Rules

- No AWS clients, no business/fiscal logic of its own — all of that lives in `go-dfe`.
- **Never log the request body**: it carries the PFX (`certificate_b64`) and its password. Log only `doc_type`,
  `service`, `uf`, status and duration.
- `environment` arrives as `producao`/`homologacao` from worker/api; `handler.go` converts to `prod`/`hom`.
- `uf` is empty for NFS-e (municipal competence); do not reject it.
- Build: `CGO_ENABLED=0 GOARCH=arm64 go build ./...`. Test: `go test ./... -race`.
- Timeout chain: worker/API (150 s) > this Lambda (120 s, `cdk/lib/egress.ts`) > go-dfe worst case (NFS-e: 4 attempts × 20 s + backoff).

## Mandatory Documentation Policy

Every code change must update the matching docs in the same change.
