# go-dfe

In-process Go SEFAZ client (SOAP + XML-DSig + mTLS). A **library**, not a service: `worker`
and `api` import it directly via the root `go.work` and call `dfe.Call` in-process (no Lambda Invoke, no network hop).
It is the **primary** dispatch path; py-dfe is the fallback (see `worker/README.md` §3).

Authoritative rules: [`CLAUDE.md`](CLAUDE.md) · root [`MIGRATION.md`](../MIGRATION.md)
(2026-07-18 entry) · [`DOCS.md`](../DOCS.md) go-dfe section.

---

## 1. Public API

- `dfe.Implements(docType, service string) bool` — `dfe.go:86`. Callers MUST check this before choosing `dfe.Call` vs
  the py-dfe Lambda.
- `dfe.Call(ctx, Request) (Response, error)` — `dfe.go:96`. Single entry point; errors for any `(docType, service)` not
  in `implemented`.
- `Request` / `Response` / `Problem` (`request.go:9-42`) mirror py-dfe's
  `LambdaRequest`/`LambdaResponse`/`Problem` — wire format unchanged. `Response.Body` is a **JSON-encoded string**
  (Lambda Invoke contract preserved).
- `shadow.ShadowCompare(ctx, req, pyDfeStatusCode, pyDfeBody)` — `shadow.go:21`. Runs
  `dfe.Call` in parallel with an already-obtained py-dfe response, compares status+body (structurally), **logs only,
  never affects the caller**, never returns an error. Migration shadow window; py-dfe stays authoritative until the
  window is clean.

## 2. `implemented` map (`dfe.go:33-81`)

Coverage by doc type / SEFAZ service category:

| docType | status | consulta            | envio/autorização      | evento/cancelamento | distribuição |
|---------|--------|---------------------|------------------------|---------------------|--------------|
| `nfe`   | ✓     | ✓ (incl. Cadastro) | ✓                     | ✓ (RecepcaoEvento) | ✓           |
| `nfce`  | ✓     | ✓                  | ✓                     | ✓                  | —            |
| `cte`   | ✓     | ✓                  | ✓ (Sinc/OS/GTVe/Simp) | ✓                  | ✓           |
| `mdfe`  | ✓     | ✓                  | ✓                     | ✓                  | ✓           |

Cancelamento flows through `RecepcaoEvento` (event 110111). The map IS the promotion gate:
removing a key silently falls back to py-dfe.

## 3. mTLS (`internal/certificate/manager.go`)

`certificate.Load(certB64, password)` → `pkcs12.DecodeChain` → leaf cert + RSA key + CA chain (`manager.go:34-53`).
Builds `*http.Client` with the mTLS cert and
`InsecureSkipVerify: true` (`manager.go:78-92`) — **deliberate** (matches py-dfe
`CERT_NONE`); do not "fix". Password comes via `Request.CertificatePassword`, never logged.

## 4. SOAP + XML

- `soap.Builder.Build` / `ParseResult` — SOAP 1.2 (`soap/envelope.go:87,139`).
- `xmlops.BuildXML` / `ParseXML` — dict↔XML, `@attr`/`@xmlns`/`#text` convention (`builder.go:8-23,48,260`).
- `xmlops.Sign` — XML-DSig, **RSA-SHA1 + SHA1 digest + Canonical XML 1.0**
  (`signer.go:34,48-51`). Hand-written because no maintained Go lib covers SEFAZ's legacy requirements. Cross-checked
  byte-identical against upstream `signxml` 5.1.0 (`signer.go:16-28`) — a strong cross-check, **not** the formal
  byte-identical gate.
- `xmlops.BuildProcessedXML` — `nfeProc`/`cteProc`/`mdfeProc`/`procEvento*` for signed services (`processor.go:52`).
  go-dfe deliberately ports the **corrected** 4-value
  `_EVENT["CTeRecepcaoEvento"]` form that py-dfe carries as a latent tuple-collapse bug (`processor.go:21-34`).

## 5. Endpoints (`internal/endpoints/table.go`, `internal/constants/constants.go`)

`endpoints.Resolve(docType, uf, environment, service)` → URL (`table.go:532`);
`endpoints.Authorizer` falls back to `uf` (`table.go:511-520`). Per-doc-type registries with SVRS redirects and **MT (
Mato Grosso) 3-prefix special-casing** (`table.go:382-403`). Environment constants `prod`/`hom`; `DefaultMaxRetries=3`;
`DocTypeCode` 55/65/57/58.

## 6. Known divergences (documented honestly)

- **B5** — signed operations were promoted 2026-07-18 **without** the plan's byte-identical signature gate (no dedicated
  SEFAZ test certificate exists in-repo). The `signxml`
  cross-check is a strong but **not equivalent** gate (`dfe.go:21-32`, `CLAUDE.md` §Signer). Real fiscal risk until the
  formal gate passes.
- **B12** — go-dfe **reproduces** py-dfe's full-XML INFO logging (`internal/services/client.go:116,131,145`); migrating
  to go-dfe does not close the PII leak.
- XSD validation is intentionally absent (`CGO_ENABLED=0` rules out libxml2); `Call` fails loudly if `ValidateSchema` is
  requested for a service that requires it (`client.go:118-120`).
- DANFE/DAMDFE rendering is permanently out of scope — py-dfe is the only render path.

See root [`CONDUCT.md`](../CONDUCT.md) / [`DOCS.md`](../DOCS.md) for the full register.
