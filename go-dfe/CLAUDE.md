# CLAUDE.md — go-dfe

Go library — in-process SEFAZ SOAP/mTLS communication, incremental replacement for the py-dfe Lambda.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md` (go-dfe section),
`../MIGRATION.md` (2026-07-18 entry), and `../docs/plans/2026-07-17-go-dfe-migration.md`.

---

## Role

A library, not a service: no `cmd/`, no Lambda handler, no HTTP server. `worker`/`api` import it
directly (`gopkg.aoctech.app/dfe/go-dfe`, linked via the root `go.work`) and call `dfe.Call` in the
same goroutine as their own request handling — no network hop, no AWS Lambda Invoke.

**Flow:** `Request → certificate.Load (mTLS) → services.Client.Call → xmlops (build/sign) → soap
(envelope) → endpoints.Resolve → HTTP POST with retry → xmlops.ParseXML → Response`

---

## Directory Structure

```
go-dfe/
├── dfe.go                      # Call(ctx, Request) (Response, error); Implements(docType, service) bool
├── shadow.go                   # ShadowCompare — shared shadow-mode comparison for worker+api seams
├── request.go                  # Request/Response/Problem — mirrors py-dfe's LambdaRequest/LambdaResponse/Problem
├── internal/
│   ├── certificate/manager.go  # PKCS12 → tls.Certificate + http.Client (mTLS, InsecureSkipVerify — deliberate)
│   ├── xmlops/
│   │   ├── builder.go           # dict ↔ XML (@xmlns/@key/#text conventions)
│   │   ├── signer.go            # XML-DSig: rsa-sha1 + sha1 + hand-written Canonical XML 1.0
│   │   ├── processor.go         # "@xml" processed document (nfeProc/procEventoNFe/etc) for signed emission/event services
│   │   └── xsdorder/table.go     # 1:1 port of py-dfe's xsd_order.py
│   ├── soap/envelope.go         # SOAP 1.2 envelope build/parse
│   ├── services/
│   │   ├── config.go             # ServiceConfig per doc_type (signature/validation sets, sign xpath)
│   │   ├── client.go             # SefazClient.Call: payload prep → endpoint resolve → POST+retry → parse
│   │   └── response.go           # per-(authorizer,service) response node-path unwrap + ensure_list normalization
│   ├── endpoints/table.go        # (doc_type, uf, env, service) → URL, incl. SVRS redirects + MT special-casing
│   └── constants/constants.go    # enums, WSDL service/operation tables, retry defaults
```

No `nf.go`/`cte.go`/`mdfe.go` OOP facade *classes* (py-dfe's `NFeServiceClient`/`CTeServiceClient`/etc,
`SefazClient.for_nfe`/`for_cte`) — `services.Client` is already generic over `docType` via
`services.Config`. BUT those facades are not pure indirection: they own real per-(authorizer,service)
response-shape logic (`_RESPONSE_NODE_PATH`, `_ensure_list`) that the Lambda handler's generic
`.call()` dispatch actually depends on for every request — that logic is ported as data tables +
`unwrapResponseNode`/`ensureList` in `response.go`, not skipped. Getting this wrong is silent and
severe: the wrong node path returns a different (or extra-nested) shape than py-dfe's, which both
defeats shadow-mode comparison (constant false divergences) and would corrupt real distribution
processing if ever promoted — see `response.go`'s tests for the AN/SVRS override cases specifically.

---

## Mandatory Workflow

1. Read relevant docs before starting (see above).
2. `rg "..."` — search for existing implementations before creating new code, and check the
   equivalent py-dfe source file first (this package's whole purpose is fidelity to it).
3. Plan → Implement → Run affected tests.
4. Update `../DOCS.md`/`../MIGRATION.md` for architectural changes; `../CONDUCT.md` for new constraints.
5. State cross-project impact (go-dfe ↔ worker ↔ api ↔ py-dfe ↔ cdk).
6. Suggest Conventional Commit.

---

## Engineering Rules

### Fidelity over Go idiom

This package's entire value is behavioral parity with py-dfe for any operation in `dfe.Implements()`.
When porting a py-dfe file (`xsd_order.py`, `endpoints.py`, `signer.py`, …), prefer a direct,
line-by-line-diffable port over a "cleaner" Go restructuring — a data table that looks like it could
be consolidated may encode a real SEFAZ federation quirk (e.g. MT's endpoint special-casing, per-UF
WSDL operation overrides). Do not "clean up" apparent duplication in ported data without checking the
Python source first.

### Constants — no magic strings

SEFAZ service names, doc-type strings, environment strings, WSDL operation/service names, and
endpoint hosts MUST be named constants/maps in `internal/constants` or `internal/endpoints` — never
scattered string literals.

### Certificate handling (MUST NOT simplify)

`internal/certificate/manager.go`'s `InsecureSkipVerify: true` is deliberate (SEFAZ's server
certificate chain is not validated by design) — mirrors py-dfe's `ssl_context()`. Do not "fix" this.

### Signer / C14N (the highest-risk file)

`internal/xmlops/signer.go` hand-implements Canonical XML 1.0 (`REC-xml-c14n-20010315`) because no
maintained Go library does — `goxmldsig` was evaluated and rejected (exclusive C14N + modern
algorithms, wrong fit for SEFAZ's legacy requirements). Any change here needs: the W3C C14N spec
vectors in `signer_test.go` still passing byte-exact, and — before promoting any *signed* operation
into `dfe.Implements()` — the plan's byte-identical gate against a captured py-dfe corpus (a
dedicated test certificate, not yet available in this repo; see `docs/plans/2026-07-17-go-dfe-migration.md`,
"Gate de assinatura"). The current signer implementation has been verified byte-identical against the
real upstream `signxml` library (see `signer.go`'s package doc), which is a strong independent
cross-check but is explicitly **not** the same as that formal gate — do not conflate the two.

### `dfe.Implements()` — the promotion gate

Promoting an operation = adding its `(docType, service)` key to the `implemented` map in `dfe.go`,
after:

- **Unsigned ops** (status/consulta/distribuição): a clean shadow-mode parity window in production
  (see `shadow.go`'s `ShadowCompare`, wired into worker/api's seams — py-dfe stays authoritative,
  divergence only logged, until the window is clean).
- **Signed ops**: the byte-identical signature gate above, in addition to shadow mode.

Reverting = removing the key (falls back to the py-dfe Lambda automatically, no other code change).
Do not add a key to `implemented` without its gate having actually run — this is fiscal software
talking to a government tax authority; a bad promotion produces real rejected/wrong tax documents.

### XSD validation and DANFE — explicitly out of scope

- XSD validation (`Config.RequiresValidation`) is not implemented — `CGO_ENABLED=0` (ARM64 Lambda
  `provided.al2023`, repo-wide rule) rules out libxml2, the only mature option. `services.Client.Call`
  fails loudly (not silently) if a caller requests `ValidateSchema: true` for a service that needs it.
- DANFE/DAMDFE rendering is out of scope permanently (no cert/signature/SOAP/mTLS involved — no
  fiscal/security upside to porting it). py-dfe remains the only path for rendering, indefinitely.

### Go Rules

- `CGO_ENABLED=0 GOARCH=arm64` must build clean (`go-dfe` is linked into `provided.al2023` Lambdas via
  `worker`/`api`, same constraint as those modules).
- No goroutines that outlive a caller's request/invocation — `ShadowCompare` runs synchronously for
  this reason (see `shadow.go`'s doc comment); do not make it fire-and-forget.

---

## Testing

| Change                           | Required                                                             |
|----------------------------------|----------------------------------------------------------------------|
| Data port (xsd_order, endpoints) | Unit test, cross-checked against the py-dfe source values            |
| Signer/C14N                      | Unit test against W3C C14N spec vectors + sign/verify round trip     |
| SefazClient/retry                | Unit test (httptest server, verifies retry-on-5xx / no-retry-on-4xx) |
| `dfe.Implements`/`Call`          | Unit test                                                            |
| Bug fix                          | Reproduce + regression                                               |

Run: `cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./...` (note: `-race` requires
a C toolchain even with `CGO_ENABLED=0` set for the build itself — unavailable in some sandboxes;
run it in CI/wherever a C compiler exists).

---

## Known Constraints

- No dedicated SEFAZ test certificate exists in this repo yet — the byte-identical signature gate and
  any live-homologação integration test are blocked on obtaining one.
- `go.work` at the repo root links `./api ./go-dfe ./worker` for local `go build`/`go test`. CDK's
  Lambda bundling (`cdk/lib/worker-stack.ts`'s `goCode`) tries local `go build` first (sees the
  workspace normally, since it runs with `cwd` inside `worker/`, a `go.work` ancestor); the Docker
  bundling fallback only mounts the single asset directory, so it would NOT see `go.work` or
  `go-dfe`'s source if that fallback path is ever exercised. Not fixed here — the local-bundling path
  is what actually runs in practice (CI's `infra.yml` deploy job installs a real Go toolchain via
  `actions/setup-go` before `cdk deploy`, so `tryBundle` succeeds there too); revisit (see the
  migration plan's fallback note, `replace` directives) if Docker bundling is ever forced (e.g. no
  local Go toolchain).
- `go-dfe` has no `go.sum` of its own — in workspace mode its checksums are recorded in the shared
  `go.work.sum` at the repo root instead. That file must be committed (it is not gitignored) for
  reproducible builds/CI; `worker/go.sum` and `api/go.sum` are untouched and still used normally for
  those modules' own dependencies.
- CI (`.github/workflows/deploy.yml`) has a dedicated `godfe` job/workflow
  (`.github/workflows/godfe.yml`, build+test only, no deploy target — this package ships nothing on
  its own) gated on a `go-dfe/**`/`go.work`/`go.work.sum` path filter. That filter is ALSO added to
  the `worker`/`api` filters (not just a standalone `godfe` filter) so a go-dfe-only change re-runs
  their test suites too, since they consume it in-process — a go-dfe regression must not slip through
  invisible to both. `worker`/`api`'s deploy jobs additionally `needs: godfe`, so a go-dfe test
  failure blocks their deploys the same way a worker failure already blocks api's.

---

## Critical Areas (require analysis before touching)

- `internal/xmlops/signer.go` (C14N/XML-DSig) — see above.
- `internal/xmlops/xsdorder/table.go`, `internal/endpoints/table.go` — data fidelity to py-dfe.
- `dfe.go`'s `implemented` map — the promotion gate.
- `shadow.go` — shared by both worker and api seams; a bug here affects both.

Before touching: identify risks + side effects, verify parity with the equivalent py-dfe behavior.

---

## Completion Checklist

- [ ] `CGO_ENABLED=0 GOARCH=arm64 go build ./...` clean; `go test ./...` passes
- [ ] Ported data/logic cross-checked against the actual py-dfe source (not assumed from memory)
- [ ] No operation added to `dfe.Implements()` without its gate (shadow parity / byte-identical)
- [ ] All constants named (no magic strings)
- [ ] Docs updated (`../DOCS.md`/`../MIGRATION.md` and/or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (go-dfe ↔ worker ↔ api ↔ py-dfe ↔ cdk)
