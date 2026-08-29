# py-dfe

Python Lambda — XML-DSig, SEFAZ SOAP, mTLS, and A1 certificate handling.
This is the **authoritative / fallback** SEFAZ client: the `worker` dispatches
to it via Lambda `InvokeFunction` when go-dfe does not `Implements` the operation
(go-dfe-primary / py-dfe-fallback — see `worker/README.md` §3). Auxiliary documents are
rendered by the Go API with Folio; this Lambda no longer accepts render-only services.

Authoritative rules: [`CLAUDE.md`](CLAUDE.md) · root [`DOCS.md`](../DOCS.md) py-dfe section.
Sibling: [`../go-dfe/README.md`](../go-dfe/README.md).

---

## 1. Public API

- `handler.handler(event, context)` — AWS Lambda entrypoint (`py_dfe/handler.py:26`).
  Accepts raw dict or API-Gateway `{body: str}`; validates `LambdaRequest` (pydantic) →
  `create_service(...)` → `service.call(service, body)` → `LambdaResponse`
  (`handler.py:33-91`).
- Service factory `create_service(doc_type, ...)` → `NFeServiceClient`/`NFCeServiceClient`/
  `CTeServiceClient`/`MDFeServiceClient` (`services/__init__.py:9-29`). The HTTP client is
  `SefazClient.call(service, payload)` (`services/base.py:148`).
- `py_dfe/__init__.py` is empty; the entry is `handler.handler`.

## 2. mTLS + signing

- `CertificateManager(pfx_b64, password)` → `pkcs12.load_key_and_certificates`
  (`certificate/manager.py:34-41`); `ssl_context()` sets `check_hostname=False`,
  `verify_mode=ssl.CERT_NONE` (deliberate — matches go-dfe `InsecureSkipVerify`; do not
  "fix"), writes cert/key to `/tmp` tempfiles deleted on exit (`manager.py:59-93`).
- `sign_xml(...)` → `signxml.XMLSigner` `enveloped`, `rsa-sha1` + `sha1` digest +
  `REC-xml-c14n-20010315` (`xmlops/signer.py:34-39`). **Docstring caveat**: `signer.py:3-5`
  claims SHA-256, but the config is rsa-sha1 — the docstring is inaccurate; runtime matches
  go-dfe. py-dfe's `CTeRecepcaoEvento` processed-XML carries a latent tuple-collapse bug
  (`processor.py:35`) that go-dfe ports correctly.

## 3. SOAP / XML / XSD

- `SOAPEnvelopeBuilder.build` / `extract_body` (`soap/envelope.py:73,132`).
- `to_xml_bytes` / `parse_xml_bytes` (`xmlops/builder.py`) — same `@attr`/`@xmlns`/`#text`
  convention as go-dfe.
- `validate(xml_bytes, doc_type, service)` against bundled XSDs (`xmlops/validator.py:138`);
  `validate_schema` defaults to `False` for `LambdaRequest` but `True` in `SefazClient`
  (`base.py:66`, `models/request.py:45`). go-dfe has no XSD validation.
- Endpoints: `get_endpoint` / `get_authorizer` (`constants/endpoints.py:404,437`); values
  are byte-identical ports of go-dfe's table.

## 4. Known divergences (documented honestly)

- **B12 — PII leak (CONFIRMED).** Full fiscal XML is logged at **INFO** in
  `py_dfe/services/base.py:169` (`raw xml`), `:177` (`soap xml`), `:180` (`received xml`,
  may contain CPF/CNPJ/recipient PII). Root logger is INFO (`logging_config.py:43`). The leak
  is specifically in `base.py`; `handler.py` logs only masked metadata. **go-dfe reproduces
  the same leak** (`go-dfe/internal/services/client.go:116,131,145`) — migrating does not
  close it.
- **B5** — signed-op promotion gate lives on the go-dfe side; py-dfe remains authoritative.
- `CTeRecepcaoEvento` processed-XML latent bug (go-dfe ports the fix).
- `signer.py` docstring (SHA-256) disagrees with actual rsa-sha1 config.

See root [`CONDUCT.md`](../CONDUCT.md) / [`DOCS.md`](../DOCS.md) for the full register
(B4, B5, B12, B14).
