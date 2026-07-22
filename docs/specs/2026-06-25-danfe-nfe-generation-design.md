# DANF-e (NF-e modelo 55) Generation — Design Spec

**Date:** 2026-06-25 **Scope:** `py-dfe` — add DANF-e (NF-e mod 55) auxiliary-document rendering to the existing
`danfe/` package, alongside the shipped DANFC-e (mod 65) generator. **Status:** Approved design.

---

## 1. Goal

Generate the **DANFE** (Documento Auxiliar da Nota Fiscal Eletrônica) for **NF-e modelo 55** from an authorized NF-e
XML, fully satisfying `py-dfe/manual_danfe.md` (MOC 7.0 Anexo II). Same invocation contract as the existing render path:
service `GerarDanfe`, no certificate, no SEFAZ communication.

**Variants (all in scope):**

| Variant      | Manual ref              | Paper                    | Sizing               |
|--------------|-------------------------|--------------------------|----------------------|
| Retrato      | §3.8.1, Anexo III.02/03 | A4 portrait (210×297mm)  | Fixed A4, multi-page |
| Paisagem     | §3.8.2, Anexo III.04/05 | A4 landscape (297×210mm) | Fixed A4, multi-page |
| Simplificado | §3.11                   | Roll, width ≥ 55mm       | Auto-height          |
| Etiqueta     | §3.12                   | Roll, width ≥ 55mm       | Auto-height          |

**Contingency (variable-content campos, §3.9 — all in scope):**

| tpEmis | Mode   | Campo 1 / Campo 2 behavior                                             |
|--------|--------|------------------------------------------------------------------------|
| 1      | Normal | §3.9.1 — single chave barcode + protocolo de autorização               |
| 6      | SVC-AN | §3.9.1 — same as normal                                                |
| 7      | SVC-RS | §3.9.1 — same as normal                                                |
| 2      | FS     | §3.9.2 — + 2nd barcode "Dados da NF-e" (36 char), protocolo suppressed |
| 5      | FS-DA  | §3.9.2 — same as FS                                                    |
| 4      | EPEC   | §3.9.3 — protocolo de autorização do EPEC                              |

---

## 2. Architecture

### 2.1 Dispatch (single service, auto by model)

`GerarDanfe` already routes to `generate_danfce` in `services/_nf.py`. Introduce a model-dispatcher so one service
handles both documents.

- New `danfe/document.py::generate_danfe(payload)`:
    1. Parse XML once, read `ide/mod`.
    2. `mod == "65"` → delegate to existing `danfce.generate_danfce` (NFC-e, **unchanged**).
    3. `mod == "55"` → delegate to new `nfe55.generate_danfe_nfe`.
    4. else → `DFeError(422, DANFE_UNSUPPORTED_MODEL, ...)`.
- `services/_nf.py`: change the `SERVICE_GERAR_DANFE` route to call `document.generate_danfe`.

To avoid double XML parse, the dispatcher parses once and passes the parsed payload forward; the NFC-e and NF-e builders
accept either raw payload or a pre-parsed handle. Implementation: dispatcher peeks `mod` via a lightweight parse, then
calls the target with the original `payload` (each generator re-uses the shared `_extract_roots`-style parse).
Simplicity over micro-optimization: a second parse of one document is negligible vs. WeasyPrint render time. **Decision:
parse for `mod` detection in the dispatcher, hand the raw `payload` to the chosen generator.**

### 2.2 Module layout (reuse the `danfe/` package)

```
py_dfe/danfe/
├── __init__.py
├── formatters.py    # REUSE money_br, dt_local, mask_cnpj, mask_cpf, chave_blocks
│                    # ADD: mask_cpf_cnpj, num_nf, mask_cep, pct (aliquota), upper
├── qr.py            # EXISTING (NFC-e QR) — untouched
├── barcode.py       # NEW — CODE-128C SVG data-uri + FS/FS-DA dados-nfe code
├── render.py        # EXTEND — htmls_to_pdf(pages, *, fit_height=True)
├── danfce.py        # EXISTING (NFC-e mod 65) — untouched
├── document.py      # NEW — dispatcher by ide/mod
├── nfe55.py         # NEW — generate_danfe_nfe + build_context (mod 55)
└── templates/
    ├── danfce.html              # EXISTING
    ├── _danfe_macros.html       # NEW — Jinja macros, one per quadro
    ├── danfe_retrato.html       # NEW — A4 portrait
    ├── danfe_paisagem.html      # NEW — A4 landscape
    ├── danfe_simplificado.html  # NEW — roll, §3.11.4 subset
    └── danfe_etiqueta.html      # NEW — roll, §3.12.4 subset
```

**Isolation contract per unit:**

- `barcode.py` — pure functions, input str → output SVG data-uri / code str. No I/O, no template knowledge.
- `nfe55.py` — owns mod-55 XML → context-dict mapping. No rendering specifics beyond choosing the template name.
- `render.py` — document-agnostic HTML→PDF. Knows nothing about NF-e/NFC-e semantics.
- `_danfe_macros.html` — presentational only; receives a fully-built context, emits no business logic.

### 2.3 Render extension

`render.py` currently fits each page height to content (thermal). Extend without breaking NFC-e:

```python
def htmls_to_pdf(pages: list[str], *, fit_height: bool = True) -> bytes:
    # fit_height=True  -> existing per-page measure + snug @page + merge (thermal/roll)
    # fit_height=False -> render each doc honoring its own @page, merge all pages as-is
```

- NFC-e (`danfce.py`) and DANF-e **Simplificado/Etiqueta** → `fit_height=True` (roll, auto-height).
- DANF-e **Retrato/Paisagem** → `fit_height=False` (fixed A4 from template `@page`, natural multi-page pagination).

`html_to_pdf(html)` keeps delegating with the default. No change to existing callers.

### 2.4 Multi-page / folhas adicionais (§3.5, §3.10.2)

For retrato/paisagem with overflowing items:

- Repeating header via WeasyPrint **running elements**: `.danfe-header { position: running(danfeHeader) }` +
  `@page { @top-center { content: element(danfeHeader) } }`. Header repeats on every folha with: emit identification,
  "DANFE", DANFE-id box, natureza da operação, chave + barcode (§3.5 minimum set).
- Folha numbering "FOLHA N/T" via CSS `counter(page)` / `counter(pages)`.
- Products table `<thead>` repeats per page automatically.
- Canhoto prints only on the first page (manual: canhoto is page-1 stub).

---

## 3. Constants (`constants/danfe.py`, additive)

```python
# Document model codes.
MODELO_NFE = "55"          # NEW (MODELO_NFCE = "65" exists)

# DANF-e (mod 55) layout variants.
LAYOUT_RETRATO = "retrato"
LAYOUT_PAISAGEM = "paisagem"
LAYOUT_SIMPLIFICADO = "simplificado"
LAYOUT_ETIQUETA = "etiqueta"
VALID_DANFE_NFE_LAYOUTS = frozenset({
    LAYOUT_RETRATO, LAYOUT_PAISAGEM, LAYOUT_SIMPLIFICADO, LAYOUT_ETIQUETA,
})
DEFAULT_DANFE_NFE_LAYOUT = LAYOUT_RETRATO

# Template filenames per layout.
DANFE_NFE_TEMPLATES = {
    LAYOUT_RETRATO: "danfe_retrato.html",
    LAYOUT_PAISAGEM: "danfe_paisagem.html",
    LAYOUT_SIMPLIFICADO: "danfe_simplificado.html",
    LAYOUT_ETIQUETA: "danfe_etiqueta.html",
}
ROLL_LAYOUTS = frozenset({LAYOUT_SIMPLIFICADO, LAYOUT_ETIQUETA})  # fit_height=True

# NF-e ide/tpEmis (manual §3.9 + MOC Anexo I cap.2).
TP_EMIS_NORMAL = "1"             # already exists; keep
TP_EMIS_FS = "2"
TP_EMIS_SCAN = "3"               # deprecated; treated as normal layout
TP_EMIS_EPEC = "4"
TP_EMIS_FSDA = "5"
TP_EMIS_SVC_AN = "6"
TP_EMIS_SVC_RS = "7"
TP_EMIS_NORMAL_LIKE = frozenset({TP_EMIS_NORMAL, TP_EMIS_SCAN, TP_EMIS_SVC_AN, TP_EMIS_SVC_RS})
TP_EMIS_FS_LIKE = frozenset({TP_EMIS_FS, TP_EMIS_FSDA})

# ide/tpNF.
TP_NF_ENTRADA = "0"
TP_NF_SAIDA = "1"
TP_NF_LABELS = {TP_NF_ENTRADA: "ENTRADA", TP_NF_SAIDA: "SAÍDA"}

# Frete / modFrete (manual §3.1.10).
MOD_FRETE_LABELS = {
    "0": "0 - Remetente",
    "1": "1 - Destinatário",
    "2": "2 - Terceiros",
    "3": "3 - Próprio (Remetente)",
    "4": "4 - Próprio (Destinatário)",
    "9": "9 - Sem Frete",
}

# Fixed copy (manual_danfe.md).
TEXT_DANFE = "DANFE"
TEXT_DANFE_DESC = "DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA"
TEXT_DANFE_SIMPLIFICADO = "DANFE Simplificado"
TEXT_DANFE_ETIQUETA = "DANFE Simplificado - Etiqueta"
TEXT_NFE_HOMOLOGACAO = "SEM VALOR FISCAL"                       # §homologação
TEXT_NFE_CONTINGENCIA = "DANFE EM CONTINGÊNCIA - IMPRESSO EM DECORRÊNCIA DE PROBLEMAS TÉCNICOS"
TEXT_PROTOCOLO = "PROTOCOLO DE AUTORIZAÇÃO DE USO"
TEXT_PROTOCOLO_EPEC = "PROTOCOLO DE AUTORIZAÇÃO DO EPEC"
TEXT_CONSULTA_NFE = "Consulta de autenticidade no portal nacional da NF-e www.nfe.fazenda.gov.br/portal ou no site da Sefaz Autorizadora"
TEXT_DADOS_NFE = "DADOS DA NF-E"
TEXT_CONTINUA_VERSO = "CONTINUA NO VERSO"  # only if verso used; not used (single-side)
```

`TEXT_WATERMARK_CANCELADA`, `TP_AMB_*`, `SERVICE_GERAR_DANFE` already exist and are reused.

> Naming note: `TP_EMIS_NORMAL = "1"` already exists for NFC-e and is reused. NFC-e's
> `TP_EMIS_CONTINGENCIA_OFFLINE = "9"` is mod-65-only and stays as-is.

---

## 4. Barcode (`barcode.py`)

Dependency: **`python-barcode`** (pure-Python, SVG output, no Pillow / no native binaries — Lambda-safe).

```python
def code128c_data_uri(value: str) -> str:
    """Render *value* (digits) as a CODE-128 SVG, returned as a data: URI.
    Raises DFeError(422, DANFE_INVALID_BARCODE) if value is empty/non-numeric."""

def dados_nfe_code(*, cuf: str, tp_emis: str, doc: str, vnf: str,
                   icms_proprio: bool, icms_st: bool, dia_emissao: str) -> str:
    """Build the 36-char 'Dados da NF-e' content for FS/FS-DA (manual §3.9.2):
    cUF(2) tpEmis(1) CNPJ/CPF(14) vNF(14) ICMSp(1) ICMSs(1) DD(2) DV(1).
    Right-aligned, zero-padded; DV = mod-103 (same algorithm as chave DV)."""
```

- `python-barcode`'s `Code128` writer emits SVG; for all-numeric data it encodes via Code-C internally (satisfies
  CODE-128C intent). We render to an SVG string (via `BytesIO`, `writer=SVGWriter`) and base64 into a
  `data:image/svg+xml;base64,...` URI for inlining — consistent with how `qr.py` inlines PNG.
- `dados_nfe_code` DV: reuse the mod-11/mod-103 helper. Manual §3.9.2 says "DV calculado de forma igual ao DV da Chave
  de Acesso" — chave DV is **mod-11**. (The §2.1 mod-103 is the *barcode symbology* DV, computed by `python-barcode`
  internally, not by us.) **Decision: `dados_nfe_code` DV = mod-11**, matching chave-DV; barcode symbology DV handled by
  the library.
- New error constant `DANFE_INVALID_BARCODE = "danfe invalid barcode"` in `exceptions.py`.

---

## 5. Context builder (`nfe55.py`)

`generate_danfe_nfe(payload)`:

1. `xml = payload["xml"]` (else `DFeError(422, DANFE_INVALID_XML)`).
2. `layout = payload.get("layout")`; invalid/missing → `DEFAULT_DANFE_NFE_LAYOUT`. Must be in `VALID_DANFE_NFE_LAYOUTS`.
3. `canceled = bool(payload.get("canceled", False))`.
4. `_extract_roots(xml)` → `inf_nfe, prot, tp_emis, tp_amb, chave`; validates `mod == "55"`.
5. `build_context(...)` → context dict.
6. `template = DANFE_NFE_TEMPLATES[layout]`; `fit_height = layout in ROLL_LAYOUTS`.
7. `html = render_html(template, context)`; `pdf = htmls_to_pdf([html], fit_height=fit_height)`.
8. return `{"pdf_b64": base64(pdf), "html": [html]}` (shape identical to NFC-e output).

`build_context` maps the full NF-e XML. Source tags (MOC Anexo I leiaute):

- **emit**: `xNome`, `xFant`, `CNPJ`, `IE`, `IEST`, `CRT`, `enderEmit` (xLgr, nro, xCpl, xBairro, xMun, UF, CEP, fone).
- **dest**: `xNome`, `CNPJ`/`CPF`/`idEstrangeiro`, `IE`, `enderDest` (full), `email`.
- **ide**: `natOp`, `tpNF`, `nNF`, `serie`, `dhEmi`, `dhSaiEnt`, `tpEmis`, `tpAmb`.
- **total/ICMSTot**: `vBC`, `vICMS`, `vBCST`, `vST`, `vProd`, `vFrete`, `vSeg`, `vDesc`, `vOutro`, `vIPI`, `vNF`,
  `vTotTrib`.
- **transp**: `modFrete`, `transporta` (xNome, CNPJ/CPF, IE, xEnder, xMun, UF), `veicTransp` (placa, UF, RNTC), `vol[]`
  (qVol, esp, marca, nVol, pesoB, pesoL).
- **det[]**: per item `prod` (cProd, xProd, NCM, CFOP, uCom, qCom, vUnCom, vDesc, vProd) + `imposto` (ICMS: CST/CSOSN,
  vBC, pICMS, vICMS; IPI: vIPI, pIPI) + `infAdProd`.
- **cobr**: `fat` (nFat, vOrig, vDesc, vLiq), `dup[]` (nDup, dVenc, vDup).
- **ISSQN** (if present): `vServ`, `vBC`, `vISS`, inscrição municipal.
- **infAdic**: `infAdFisco`, `infCpl`, `obsCont[]`.
- **prot/infProt**: `nProt`, `dhRecbto`.

Derived flags:

- `is_contingencia = tp_emis not in TP_EMIS_NORMAL_LIKE`.
- `is_fs = tp_emis in TP_EMIS_FS_LIKE` → render 2nd barcode (`dados_nfe_code` + `code128c_data_uri`).
- `is_epec = tp_emis == TP_EMIS_EPEC` → protocolo labelled EPEC.
- `show_protocolo`: True for normal-like + EPEC; suppressed for FS-like (not yet authorized).
- `is_homologacao = tp_amb == TP_AMB_HOMOLOGACAO` → "SEM VALOR FISCAL".
- `is_cancelada = canceled`.
- `chave_barcode = code128c_data_uri(chave)`.

`_consumidor`/`_endereco`-style helpers reused/extended; `dest` is mandatory for mod 55 (no "consumidor não
identificado" path).

---

## 6. Templates

`_danfe_macros.html` — Jinja macros, one per quadro, each taking explicit args from context:

`canhoto`, `emit_box`, `danfe_id_box`, `chave_box`, `natop_box`, `dest_box`, `fatura_box`, `imposto_box`, `transp_box`,
`produtos_table`, `issqn_box`, `infadic_box`, `watermarks`.

- **danfe_retrato.html** — `@page { size: A4 portrait; margin: 5mm }`, running-element header, composes all quadros
  top-to-bottom per Anexo III.02. Font ≥ sizes from §3.7 (DANFE 12pt bold, nº/série 10pt, descrições 8pt, produtos 6pt).
- **danfe_paisagem.html** — `@page { size: A4 landscape; margin: 5mm }`, same macros, two-column arrangement where the
  anexo III.04 splits emit/identificação horizontally.
- **danfe_simplificado.html** — roll width (default 80mm, ≥55mm), §3.11.4 obrigatórios only: emit (Nome, UF, CNPJ, IE),
  ide (tpNF, série, nº, dhEmi), dest (Nome, UF, CNPJ/CPF), itens (descrição, un, qtd, vUnit, vTotal), total (vNF),
  "DANFE Simplificado", chave + barcode, protocolo. Auto-height.
- **danfe_etiqueta.html** — roll width, §3.12.4 obrigatórios only: "DANFE Simplificado - Etiqueta", emit (Nome, UF,
  CNPJ, IE), ide (tpNF, série, nº, dhEmi), dest (Nome, UF, CNPJ/CPF, IE), total (vNF), chave + barcode, protocolo (EPEC
  if applicable). Auto-height. More compact than simplificado.

All templates honor the shared `margin 0.2–0.8cm` lateral rule (§3.6.2) and overflow-safe table layout
(`table-layout: fixed`, colgroup widths) learned from the NFC-e fixes.

---

## 7. Error handling

- All failures raise `DFeError(status_code, code, message)`.
- Reuse: `DANFE_INVALID_XML` (422), `DANFE_UNSUPPORTED_MODEL` (422, now also raised for mod ∉ {55,65}),
  `DANFE_RENDER_FAILED` (500).
- Add: `DANFE_INVALID_BARCODE = "danfe invalid barcode"` (422).

---

## 8. Dependencies

- Add `python-barcode>=0.15.1` to `pyproject.toml`.
- Pin `python-barcode==0.15.1` in `layer/requirements.txt`.
- No new native libraries (WeasyPrint native stack already provisioned for DANFC-e).

---

## 9. Testing

**Unit:**

- `test_danfe_constants.py` — extend: new layout/tpEmis/tpNF/frete constants present and consistent.
- `test_danfe_formatters.py` — extend: `mask_cpf_cnpj`, `num_nf`, `mask_cep`, `pct`.
- `test_danfe_barcode.py` (NEW) — `code128c_data_uri` returns `data:image/svg+xml;base64`; rejects empty/non-numeric;
  `dados_nfe_code` is 36 chars, right-aligned/zero-padded, correct mod-11 DV.
- `test_danfe_document.py` (NEW) — dispatcher: mod 65 → NFC-e output, mod 55 → NF-e output, other →
  `DANFE_UNSUPPORTED_MODEL`.
- `test_danfe_nfe.py` (NEW) — per variant: produces `%PDF`; retrato/paisagem contain all mandatory quadros + masked emit
  CNPJ + chave blocks; simplificado/etiqueta contain only their §3.11.4/§3.12.4 fields; homologação → "SEM VALOR
  FISCAL"; cancelada → watermark; FS/FS-DA → 2nd barcode + no protocolo; EPEC → EPEC protocol label; normal → protocolo
  present.
- `test_danfe_render.py` — extend: `fit_height=False` produces multi-page A4 when content overflows; `fit_height=True`
  unchanged for roll.

**Integration:**

- `tests/integration/test_danfe_nfe_generation.py` (NEW) — end-to-end via `generate_danfe` dispatcher from a synthetic
  authorized mod-55 nfeProc fixture: each variant renders a valid PDF; multi-item input yields >1 A4 page in retrato;
  each contingency mode renders without error.

**Fixtures:**

- `tests/danfe_fixtures.py` — extend with
  `sample_nfe55_proc(*, layout-agnostic, tp_emis="1", tp_amb="1", n_items=2, with_transp=True, with_dup=True, with_issqn=False)`
  returning a synthetic mod-55 nfeProc XML. No real CNPJ/CPF/customer data.

---

## 10. Docs

- `../DOCS.md §3` — extend the GerarDanfe section: now model-dispatched (55 + 65), variant matrix, barcode module,
  `python-barcode` dep.
- `../CONDUCT.md` — extend "DANFC-e rendering" → "DANFE rendering": note CODE-128C via python-barcode (pure-Python),
  fixed-A4 vs roll sizing split, running-element multi-page header.

---

## 11. Cross-project impact

- **worker** — no change: `GerarDanfe` contract unchanged; model auto-detected from XML. Worker may pass an optional
  `layout`.
- **api / cdk** — no change. Lambda layer gains one pure-Python wheel (`python-barcode`); no new native lib.
- **ui** — out of scope.

---

## 12. Out of scope (YAGNI)

- Emit logo rendering (not in XML; manual marks optional).
- Verso usage (§3.4) — single-side only; `CONTINUA NO VERSO` not emitted.
- Formulário de segurança physical stamp area.
- CT-e / MDF-e auxiliary docs (future family members).
