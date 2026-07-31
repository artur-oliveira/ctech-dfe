# CLAUDE.md — py-dfe

Python Lambda — XML-DSig, SEFAZ SOAP, mTLS, A1 certificate handling. Python 3.14.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §3`.

---

## Role

Stateless Lambda that receives a structured `LambdaRequest`, loads the A1 certificate (PFX),
signs the XML with XML-DSig, and submits via SOAP to SEFAZ using mTLS. Returns `LambdaResponse`.
Does not access DynamoDB, S3, or any other AWS service directly.

**Flow:** `LambdaRequest → CertificateManager → ServiceClient → SEFAZ SOAP → LambdaResponse`

---

## Directory Structure

```
py-dfe/
├── py_dfe/
│   ├── handler.py              # Lambda entrypoint — validates request, routes to service
│   ├── certificate/
│   │   └── manager.py          # PFX load, key extraction, mTLS SSL context
│   ├── constants/
│   │   ├── enums.py            # UF, Environment, DocType enums
│   │   └── endpoints.py        # SEFAZ endpoint registry (prod + hom, per UF)
│   ├── services/
│   │   ├── base.py             # SefazClient — async httpx, retry with backoff
│   │   ├── _nf.py              # Shared NF-e/NFC-e logic
│   │   └── nfe.py / nfce.py / cte.py / mdfe.py
│   ├── soap/
│   │   └── envelope.py         # SOAP envelope builder per service
│   ├── xmlops/
│   │   ├── builder.py          # dict ↔ XML (lxml)
│   │   ├── signer.py           # XML-DSig (signxml)
│   │   └── validator.py        # XSD validation
│   └── model/                  # Pydantic models (LambdaRequest, LambdaResponse)
├── schemas/xsds/               # Bundled XSDs (NF-e, NFC-e, CT-e, MDF-e)
└── tests/
    ├── unit/
    └── integration/
```

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing implementations before creating new code.
3. Plan → Implement → Run affected tests.
4. Update `../DOCS.md §3` for new services/operations; `../CONDUCT.md` for new constraints.
5. State cross-project impact (py-dfe ↔ worker ↔ cdk).
6. Suggest Conventional Commit.

---

## Engineering Rules

### DRY

- Never duplicate SEFAZ logic. Shared NF-e/NFC-e logic lives in `services/_nf.py`.
- XML building helpers, SOAP envelope patterns, and retry logic must be shared — not per-service.
- Before adding any function, search `py_dfe/` for existing implementations.
- SEFAZ endpoint registry lives in `constants/endpoints.py` — never inline endpoint URLs.

### Constants — no magic strings/numbers

- All SEFAZ endpoint URLs are in `constants/endpoints.py`.
- UF codes, environment strings (`producao`/`homologacao`), and doc type literals are in `constants/enums.py`.
- HTTP status codes and SEFAZ `cStat` codes must be named constants — never inline integers.
- Never redeclare these constants in individual service files.

### Error Handling (MUST follow)

- **Always raise `DFeError`** with explicit `status_code`, `code`, and `message`.
- Never raise generic `Exception` or `ValueError` — always use `DFeError`.
- `DFeError` examples:
  ```python
  DFeError(422, "SCHEMA_VALIDATION_FAILED", "XML failed XSD validation")
  DFeError(502, "SEFAZ_UNREACHABLE", "Timeout communicating with SEFAZ")
  DFeError(400, "CERT_EXPIRED", "A1 certificate has expired")
  ```

### Retry Logic (critical)

- **Retry only on network failures** (timeout, connection error, HTTP 5xx from SEFAZ infrastructure).
- **Never retry SEFAZ business rejections** (`cStat` rejection codes) — persist the rejection.
- Retry uses exponential backoff. Max retries from `LambdaRequest.max_retries` (0–10, default 3).

### Certificate Handling (MUST NOT simplify)

- `CertificateManager` in `certificate/manager.py` handles PFX load, private key extraction,
  and mTLS SSL context configuration.
- Do not simplify, inline, or replace `CertificateManager` — the mTLS setup is deliberate.
- Certificate password is never logged.

### mTLS

- All SEFAZ communication uses mTLS via `httpx` with the loaded A1 certificate.
- MT (Mato Grosso) endpoints are special-cased in `constants/endpoints.py` — do not remove.

### XSD Validation

- `validate_schema=False` by default in production (CPU-intensive).
- Bundled XSDs in `schemas/xsds/` — do not fetch from external URLs at runtime.

### Python Style

- `snake_case` for functions and variables; `PascalCase` for classes; `UPPER_CASE` for constants.
- Pydantic v2 for all input/output models (`model/`).
- Async httpx for HTTP — do not use `requests` (sync).

### Secrets

Never commit: PFX certs, certificate passwords, real CNPJ/CPF, real customer data.

---

## Testing

| Change              | Required                              |
|---------------------|---------------------------------------|
| New service/op      | Unit + integration (SEFAZ homolog.)   |
| XML building        | Unit test (compare against XSD)       |
| Certificate logic   | Unit test                             |
| Error handling      | Unit test (all `DFeError` paths)      |
| Retry logic         | Unit test (mock httpx)                |
| Bug fix             | Regression test                       |

**Every core function (XML signing, SEFAZ communication, certificate loading) must have an integration test.**

Run: `pytest tests/unit/` for unit tests.
Run: `pytest tests/integration/` for integration tests (requires SEFAZ homologation access).

---

## Known Constraints

- Lambda runtime: Python 3.14 (`provided` via CDK layer).
- Stateless — no DynamoDB, S3, or Redis access; no state between invocations.
- `validate_schema` is disabled by default in production for performance — do not enable without profiling.
- MT (Mato Grosso) endpoints are special-cased — verify before changing endpoint routing.
- XML structure must follow SEFAZ specification strictly — do not simplify or reformat.

---

## Critical Areas (require analysis before touching)

- `CertificateManager` (mTLS setup)
- `signer.py` (XML-DSig)
- `endpoints.py` (SEFAZ endpoint registry, especially MT special-cases)
- Retry logic in `services/base.py`
- `LambdaRequest` / `LambdaResponse` Pydantic models (contract with worker)

Before touching: identify risks + side effects, verify backward compatibility + SEFAZ regulatory impact.

---

## Completion Checklist

- [ ] `pytest tests/unit/` passes
- [ ] No duplication introduced (searched before creating)
- [ ] All constants in `constants/` (no magic strings/URLs)
- [ ] Errors raised as `DFeError` with explicit status_code/code/message
- [ ] Retries only on network errors (never SEFAZ rejections)
- [ ] `CertificateManager` not simplified or bypassed
- [ ] Docs updated (`../DOCS.md §3` and/or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (py-dfe ↔ worker ↔ cdk)

## Mandatory Documentation Policy

**Every code change MUST be documented.**

There are NO exceptions.

Any modification affecting behavior, architecture, APIs, integrations, configuration, deployment, security, business rules, or developer workflow MUST include the corresponding documentation update in the same change.
