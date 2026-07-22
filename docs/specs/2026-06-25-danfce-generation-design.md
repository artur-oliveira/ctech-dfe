# DANFC-e Generation in py-dfe — Design Spec

**Date:** 2026-06-25 **Project:** py-dfe **Status:** Approved (design)

## 1. Goal

Add an in-house document-rendering capability to `py_dfe` that produces the auxiliary fiscal document **DANFC-e** (DANFE
NFC-e) from an authorized NFC-e XML. This will make possible the auxiliar document issuingwith a local, deterministic
renderer.

This is the first of a family of auxiliary documents (DANFC-e, DANF-e, DACT-e, DAMDF-e). The render pipeline is built
**generic-first** so the remaining three reuse it. **Only DANFC-e is in scope for this spec.**

The feature must satisfy the full `py-dfe/manual_danfce.md` (Manual de Padrões DANFE NFC-e, v5.1, dez/2019) and
implement all three variants: **Completo**, **Resumido**, **Contingência Offline (2 vias)**.

## 2. Constraints

- HTML→PDF, compatible with AWS Lambda (Python 3.14, `provided` runtime via CDK layer).
- No digital certificate, no SEFAZ communication — pure local rendering.
- DANFC-e must contain **only** data present in the NFC-e XML (manual mandate), except return-authorization data already
  embedded in the XML (`protNFe`).
- Errors raised as `DFeError` only (status_code, code, message).
- All string keys / codes are named constants (no magic strings).

## 3. Decisions (from brainstorming)

| Decision          | Choice                                                                                                             |
|-------------------|--------------------------------------------------------------------------------------------------------------------|
| HTML→PDF engine   | **WeasyPrint** — supports physical units (`@page` in mm), exact 56mm/25mm sizing required by manual                |
| QR image          | **segno** (pure-python, no Pillow dep), level **M**, UTF-8, emitted as data-URI PNG                                |
| Templating        | **Jinja2**                                                                                                         |
| Output            | `{"pdf_b64": "<base64>", "html": ["<via>", ...]}` — PDF **and** HTML                                               |
| Input             | Authorized NFC-e XML string; all data read from XML                                                                |
| Variant selection | `layout` explicit; contingência auto from `ide/tpEmis`; homologação auto from `tpAmb`; cancelada via explicit flag |

## 4. Architecture

New subpackage `py_dfe/danfe/`, generic-first:

```
py_dfe/danfe/
├── __init__.py
├── render.py          # GENERIC: Jinja2 HTML → WeasyPrint PDF. Doc-agnostic.
├── qr.py              # GENERIC: QR string → data-URI PNG (segno, level M, UTF-8)
├── formatters.py      # GENERIC: BR money/date(UTC→local)/CNPJ/CPF mask, chave 11-block
├── danfce.py          # DANFC-e specific: XML → context dict, variant logic
└── templates/
    └── danfce.html    # 58mm thermal layout, 9 divisões, Jinja2
```

Future docs (DANF-e/DACT-e/DAMDF-e) add only `danfe.py`/`dacte.py`/`damdfe.py`
plus a template; they reuse `render.py`, `qr.py`, `formatters.py`.

### Unit responsibilities

- **`render.py`** — `render_pdf(template_name, context) -> (pdf_bytes, html_str)`. Loads Jinja2 env from `templates/`,
  renders HTML, runs WeasyPrint to PDF. Knows nothing about NFC-e.
- **`qr.py`** — `qr_data_uri(payload: str) -> str`. segno QR (error level M, UTF-8 encoding), PNG base64 data-URI for
  inline `<img src>`.
- **`formatters.py`** — pure functions:
    - `money_br(value) -> "1.234,56"` (comma decimal, dot thousands — manual §3.1.2)
    - `dt_local(utc_iso) -> str` (UTC → local time — manual §3.1.7)
    - `mask_cnpj` `99.999.999/9999-99`, `mask_cpf` `999.999.999-99`
    - `chave_blocks(key) -> "1234 5678 ..."` (11 blocks of 4 digits — manual §3.1.4)
- **`danfce.py`** —
    - `generate_danfce(payload: dict) -> dict` (the entrypoint reached from `call`)
    - `build_context(nfe_proc: dict, *, layout, canceled) -> dict`
    - variant resolution + 2-vias assembly.

## 5. Entrypoint / routing

- New constant `SERVICE_GERAR_DANFE = "GerarDanfe"` (in `constants/`).
- `LambdaRequest`: `certificate_b64` / `certificate_password` become **optional**
  (`str | None = None`). Removes the `min_length=10` hard requirement at model level.
- `handler.py`: build `CertificateManager` **only when** a certificate is present. For render services (`GerarDanfe`)
  cert is absent → skip. Cert-required SEFAZ services validate presence in the handler and raise
  `DFeError(400, "CERT_REQUIRED", ...)` when missing.
- `SefazClient` SSL context must stay **lazy** (built on the first real SEFAZ call), so `create_service` succeeds with
  `cert_manager=None`. Verify during implementation that `__init__` does not eagerly load the cert; if it does, defer
  it.
- `_NFServiceClient.call` branches before any SEFAZ work:
  ```python
  if service == SERVICE_GERAR_DANFE:
      return generate_danfce(payload)
  ```

### Request shape

```json
{
  "doc_type": "nfce",
  "service": "GerarDanfe",
  "uf": "SP",
  "environment": "producao",
  "body": {
    "xml": "<nfeProc>...</nfeProc>",
    "layout": "completo",
    "canceled": false
  }
}
```

`layout` ∈ {`completo` (default), `resumido`}. `canceled` default `false`.

## 6. Data extraction (from `nfeProc` / `NFe` XML via existing `parse_xml_bytes`)

| Division                   | Source tags                                                                                                                                    |
|----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| I — Cabeçalho              | `emit/CNPJ`, `emit/xNome`, `emit/enderEmit/*` (no país); fixed text "Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica"               |
| II — Itens (Completo only) | `det[]/prod`: `cProd`, `xProd`, `qCom`, `uCom`, `vUnCom`, `vProd`                                                                              |
| III — Totais               | `total/ICMSTot`: `vProd`, `vDesc`, `vFrete`, `vSeg`, `vOutro`, `vNF`; `pag/detPag`: `tPag`, `vPag`; `pag/vTroco`; item count = number of `det` |
| IV — Consulta chave        | `infNFeSupl/urlChave` + chave em 11 blocos                                                                                                     |
| V — QR Code                | `infNFeSupl/qrCode` → QR image                                                                                                                 |
| VI — Consumidor            | `dest/CNPJ`\|`CPF`\|`idEstrangeiro`, `dest/xNome`, `dest/enderDest`; else "CONSUMIDOR NÃO IDENTIFICADO"                                        |
| VII — Ident + Protocolo    | `ide/nNF`, `ide/serie`, `ide/dhEmi` (→local); `protNFe/infProt/nProt` + `dhRecbto` (→local). Suppressed in contingência                        |
| VIII — Mensagem fiscal     | `infAdic/infAdFisco`; contingência / homologação banners                                                                                       |
| IX — Mensagem contribuinte | `infAdic/infCpl`                                                                                                                               |

Chave de acesso: from `protNFe/infProt/chNFe`, fallback `infNFe@Id` stripped of
`NFe` prefix.

`tPag` codes mapped to labels (Dinheiro, Cheque, Cartão Crédito, … — manual §3.1.3) via a constant map.

## 7. Variants

| Variant                   | Trigger                                 | Effect                                                                                                                                                                                                                                                              |
|---------------------------|-----------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Completo**              | `layout="completo"` (default)           | All 9 divisões including Divisão II                                                                                                                                                                                                                                 |
| **Resumido**              | `layout="resumido"`                     | Divisão II omitted (no item detail) — manual §3.1.2                                                                                                                                                                                                                 |
| **Contingência (2 vias)** | auto: `ide/tpEmis == 9` (NFC-e offline) | Protocolo suppressed; banner "EMITIDA EM CONTINGÊNCIA Pendente de autorização" below div I **and** below div VII (manual §3.1.8); **two vias** — Via Consumidor + Via Estabelecimento; 2nd via prints "Via do Estabelecimento" beside emission date (manual §3.1.8) |
| **Homologação**           | auto: `tpAmb == 2`                      | Centered upper-case banner "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL" (manual §3.1.8) + watermark                                                                                                                                                      |
| **Cancelada**             | `body.canceled == true`                 | Watermark "CANCELADA"                                                                                                                                                                                                                                               |

Variants compose: `resumido` may also be contingência / homologação / cancelada.

`tpEmis` constants: `1` normal, `9` contingência offline NFC-e. Only `9` triggers the contingência DANFC-e layout (2
vias).

## 8. Output

```json
{ "pdf_b64": "<base64 single PDF>", "html": ["<via1 html>", "<via2 html>?"] }
```

- Normal / completo / resumido: single-page PDF, `html` length 1.
- Contingência: single PDF with **2 pages** (Via Consumidor + Via Estabelecimento),
  `html` length 2.

## 9. Paper / layout (manual §3.3, §3.4)

- `@page { size: 58mm auto; margin: 2mm; }` (min width 56mm + 2mm margins).
- QR image min 25mm × 25mm (22mm content + ~3mm quiet zone). QR positioned per manual figura 4/5 (lateral or centered) —
  implementation picks centered for the thermal width.
- Monospace-friendly small font; values right-aligned.

## 10. New dependencies + cross-project impact

- **py-dfe `pyproject.toml`**: add `weasyprint`, `jinja2`, `segno`.
- **py-dfe `layer/requirements.txt`**: add pinned `weasyprint`, `jinja2`, `segno`
  (this is the deployed Lambda layer dep list).
- **cdk**: WeasyPrint requires native libraries (`cairo`, `pango`, `gdk-pixbuf`,
  `glib`, `gobject`, fonts) in the Lambda layer / image. The CDK layer build must bundle them. **Flag to cdk owner — out
  of scope for this spec's code, but a hard runtime dependency.**
- **api / worker**: `danfe.go` currently calls `consultadanfe.com`. Swapping it to invoke `py_dfe` `GerarDanfe` is **out
  of scope** here; noted as a follow-up.

## 11. Error handling (`DFeError` only)

| status | code                      | when                                      |
|--------|---------------------------|-------------------------------------------|
| 422    | `DANFE_INVALID_XML`       | XML unparseable / missing required nodes  |
| 422    | `DANFE_UNSUPPORTED_MODEL` | `ide/mod != 65`                           |
| 422    | `DANFE_MISSING_QRCODE`    | `infNFeSupl/qrCode` absent                |
| 400    | `CERT_REQUIRED`           | SEFAZ service invoked without certificate |
| 500    | `DANFE_RENDER_FAILED`     | WeasyPrint / template failure             |

## 12. Testing

**Unit:**

- `formatters`: `money_br`, `dt_local` (UTC→local), `mask_cnpj`/`mask_cpf`,
  `chave_blocks`.
- `qr`: data-URI shape, payload round-trip decode (level M).
- `build_context`: field extraction from a sample XML, consumidor identified vs not, payment-form mapping, item count.
- variant resolution: layout completo/resumido, tpEmis=9 → 2 vias, tpAmb=2 → homolog banner, canceled flag → watermark.
- error paths: each `DFeError` above.

**Integration:**

- Render a sample authorized NFC-e XML → assert PDF starts with `%PDF`, HTML contains all mandatory divisão fields, QR
  `<img>` present.
- Contingência sample (`tpEmis=9`) → 2-page PDF, `html` length 2, banner +
  "Via do Estabelecimento" present, protocolo absent.
- Homologação sample → banner present. Cancelada flag → watermark present.

## 13. Out of scope

- DANF-e / DACT-e / DAMDF-e (future, reuse generic render).
- Swapping `danfe.go` / worker to call py_dfe.
- CDK layer native-lib bundling (flagged, owned by cdk).
- QR hash / CSC generation — QR URL is read from XML, not recomputed.
