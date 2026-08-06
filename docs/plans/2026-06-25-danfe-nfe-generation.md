# DANF-e (NF-e modelo 55) Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:
> executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add DANF-e (NF-e modelo 55) auxiliary-document rendering (4 variants + full contingency) to the existing
`py_dfe/danfe/` package, dispatched from the same `GerarDanfe` service that already serves DANFC-e (mod 65).

**Architecture:** A model-dispatcher (`document.py`) reads `ide/mod` and routes mod 65 → existing
`danfce.generate_danfce` (untouched), mod 55 → new `nfe55.generate_danfe_nfe`. Mod-55 building, barcode generation, and
the generic HTML→PDF renderer are isolated units. Four Jinja templates (composed from shared macros) cover
retrato/paisagem (fixed A4, multi-page) and simplificado/etiqueta (roll, auto-height).

**Tech Stack:** Python 3.14, lxml (XML parse, reuse `xmlops.builder.parse_xml_bytes`), Jinja2 (templating), WeasyPrint
(HTML→PDF), python-barcode (CODE-128 SVG), segno (existing, NFC-e QR only).

## Global Constraints

- **Errors:** every failure raises `DFeError(status_code, error_code, message)`. Never raise bare `Exception`/
  `ValueError`.
- **Constants:** no magic strings/numbers — all model codes, tpEmis/tpNF values, layout keys, labels, and copy live in
  `py_dfe/constants/danfe.py`.
- **DRY:** reuse `formatters.py`, `render.py`, `xmlops.builder.parse_xml_bytes`, and existing `exceptions.py` codes
  before adding new ones.
- **Secrets:** test fixtures use only synthetic CNPJ/CPF (`12345678000199`, `12345678909`). No real customer data.
- **Dependency floor:** `python-barcode>=0.15.1` (pyproject), pinned `python-barcode==0.15.1` (layer/requirements.txt).
  No new native libraries.
- **NFC-e path is frozen:** `danfce.py`, `qr.py`, and `templates/danfce.html` MUST NOT change behavior. Only `render.py`
  is extended (backward-compatible default).
- **Output shape:** `generate_danfe_nfe` returns `{"pdf_b64": <ascii base64>, "html": [<str>, ...]}` — identical shape
  to `generate_danfce`.
- **Commits:** do NOT auto-commit. Per repo policy, stage changes only (`git add`); the user commits explicitly.
  Commit-message lines below are suggestions for when the user asks. Never add a `Co-Authored-By` trailer.
- **Test runner:** `cd py-dfe && python -m pytest`. WeasyPrint-dependent tests guard with
  `pytest.importorskip("weasyprint")`.

---

### Task 1: Constants + barcode error code

**Files:**

- Modify: `py-dfe/py_dfe/constants/danfe.py` (append)
- Modify: `py-dfe/py_dfe/exceptions.py:18` (add one code constant)
- Test: `py-dfe/tests/unit/test_danfe_constants.py` (append)

**Interfaces:**

- Produces: `MODELO_NFE`, `LAYOUT_RETRATO`, `LAYOUT_PAISAGEM`, `LAYOUT_SIMPLIFICADO`, `LAYOUT_ETIQUETA`,
  `VALID_DANFE_NFE_LAYOUTS`, `DEFAULT_DANFE_NFE_LAYOUT`, `DANFE_NFE_TEMPLATES`, `ROLL_LAYOUTS`, `TP_EMIS_FS`,
  `TP_EMIS_SCAN`, `TP_EMIS_EPEC`, `TP_EMIS_FSDA`, `TP_EMIS_SVC_AN`, `TP_EMIS_SVC_RS`, `TP_EMIS_NORMAL_LIKE`,
  `TP_EMIS_FS_LIKE`, `TP_NF_ENTRADA`, `TP_NF_SAIDA`, `TP_NF_LABELS`, `MOD_FRETE_LABELS`, and the `TEXT_*` copy
  constants. `DANFE_INVALID_BARCODE` in `exceptions.py`.

- [ ] **Step 1: Write the failing test**

Append to `py-dfe/tests/unit/test_danfe_constants.py`:

```python
def test_danfe_nfe_layout_constants():
    from py_dfe.constants import danfe as c
    assert c.MODELO_NFE == "55"
    assert c.DEFAULT_DANFE_NFE_LAYOUT == c.LAYOUT_RETRATO
    assert c.VALID_DANFE_NFE_LAYOUTS == {
        c.LAYOUT_RETRATO, c.LAYOUT_PAISAGEM,
        c.LAYOUT_SIMPLIFICADO, c.LAYOUT_ETIQUETA,
    }
    # Every layout maps to a template; roll layouts are a subset.
    assert set(c.DANFE_NFE_TEMPLATES) == c.VALID_DANFE_NFE_LAYOUTS
    assert c.ROLL_LAYOUTS == {c.LAYOUT_SIMPLIFICADO, c.LAYOUT_ETIQUETA}


def test_danfe_nfe_tpemis_groups():
    from py_dfe.constants import danfe as c
    assert c.TP_EMIS_NORMAL in c.TP_EMIS_NORMAL_LIKE
    assert {c.TP_EMIS_SVC_AN, c.TP_EMIS_SVC_RS, c.TP_EMIS_SCAN} <= c.TP_EMIS_NORMAL_LIKE
    assert c.TP_EMIS_FS_LIKE == {c.TP_EMIS_FS, c.TP_EMIS_FSDA}
    assert c.TP_EMIS_EPEC not in c.TP_EMIS_NORMAL_LIKE
    assert c.TP_EMIS_EPEC not in c.TP_EMIS_FS_LIKE


def test_danfe_invalid_barcode_code():
    from py_dfe.exceptions import DANFE_INVALID_BARCODE
    assert DANFE_INVALID_BARCODE == "danfe invalid barcode"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_constants.py -q`
Expected: FAIL (`AttributeError: module ... has no attribute 'MODELO_NFE'`).

- [ ] **Step 3: Add the error code**

In `py-dfe/py_dfe/exceptions.py`, after line 17 (`DANFE_RENDER_FAILED = 'danfe render failed'`):

```python
DANFE_INVALID_BARCODE = 'danfe invalid barcode'
```

- [ ] **Step 4: Append the constants**

Append to `py-dfe/py_dfe/constants/danfe.py`:

```python
# ---------------------------------------------------------------------------
# DANF-e (NF-e modelo 55) — manual_danfe.md (MOC 7.0 Anexo II)
# ---------------------------------------------------------------------------

# Document model code for NF-e.
MODELO_NFE = "55"

# DANF-e layout variants (the `layout` payload key, mod-55 valid set).
LAYOUT_RETRATO = "retrato"
LAYOUT_PAISAGEM = "paisagem"
LAYOUT_SIMPLIFICADO = "simplificado"
LAYOUT_ETIQUETA = "etiqueta"
VALID_DANFE_NFE_LAYOUTS = frozenset(
    {LAYOUT_RETRATO, LAYOUT_PAISAGEM, LAYOUT_SIMPLIFICADO, LAYOUT_ETIQUETA}
)
DEFAULT_DANFE_NFE_LAYOUT = LAYOUT_RETRATO

# Layout → template filename (under danfe/templates/).
DANFE_NFE_TEMPLATES = {
    LAYOUT_RETRATO: "danfe_retrato.html",
    LAYOUT_PAISAGEM: "danfe_paisagem.html",
    LAYOUT_SIMPLIFICADO: "danfe_simplificado.html",
    LAYOUT_ETIQUETA: "danfe_etiqueta.html",
}
# Roll/auto-height layouts use fit_height=True; A4 layouts use fit_height=False.
ROLL_LAYOUTS = frozenset({LAYOUT_SIMPLIFICADO, LAYOUT_ETIQUETA})

# ide/tpEmis (NF-e; manual §3.9 + MOC Anexo I cap.2). TP_EMIS_NORMAL="1" above.
TP_EMIS_FS = "2"
TP_EMIS_SCAN = "3"          # deprecated; printed like a normal emission
TP_EMIS_EPEC = "4"
TP_EMIS_FSDA = "5"
TP_EMIS_SVC_AN = "6"
TP_EMIS_SVC_RS = "7"
# Normal-like: single chave barcode + protocolo de autorização (§3.9.1).
TP_EMIS_NORMAL_LIKE = frozenset(
    {TP_EMIS_NORMAL, TP_EMIS_SCAN, TP_EMIS_SVC_AN, TP_EMIS_SVC_RS}
)
# FS-like: second "Dados da NF-e" barcode, protocolo suppressed (§3.9.2).
TP_EMIS_FS_LIKE = frozenset({TP_EMIS_FS, TP_EMIS_FSDA})

# ide/tpNF.
TP_NF_ENTRADA = "0"
TP_NF_SAIDA = "1"
TP_NF_LABELS = {TP_NF_ENTRADA: "ENTRADA", TP_NF_SAIDA: "SAÍDA"}

# transp/modFrete (manual §3.1.10).
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
TEXT_NFE_HOMOLOGACAO = "SEM VALOR FISCAL"
TEXT_NFE_CONTINGENCIA = (
    "DANFE EM CONTINGÊNCIA - IMPRESSO EM DECORRÊNCIA DE PROBLEMAS TÉCNICOS"
)
TEXT_PROTOCOLO = "PROTOCOLO DE AUTORIZAÇÃO DE USO"
TEXT_PROTOCOLO_EPEC = "PROTOCOLO DE AUTORIZAÇÃO DO EPEC"
TEXT_CONSULTA_NFE = (
    "Consulta de autenticidade no portal nacional da NF-e "
    "www.nfe.fazenda.gov.br/portal ou no site da Sefaz Autorizadora"
)
TEXT_DADOS_NFE = "DADOS DA NF-E"
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_constants.py -q`
Expected: PASS.

- [ ] **Step 6: Stage**

```bash
git add py-dfe/py_dfe/constants/danfe.py py-dfe/py_dfe/exceptions.py py-dfe/tests/unit/test_danfe_constants.py
# Suggested commit (only when user asks): feat(py-dfe): add DANF-e mod-55 constants and barcode error code
```

---

### Task 2: Formatter extensions

**Files:**

- Modify: `py-dfe/py_dfe/danfe/formatters.py` (append functions)
- Test: `py-dfe/tests/unit/test_danfe_formatters.py` (append)

**Interfaces:**

- Consumes: nothing new.
- Produces: `mask_cpf_cnpj(digits: str) -> str`, `num_nf(value: str) -> str`, `mask_cep(digits: str) -> str`,
  `pct(value: str | float | int | None) -> str`.

- [ ] **Step 1: Write the failing test**

Append to `py-dfe/tests/unit/test_danfe_formatters.py`:

```python
def test_mask_cpf_cnpj_picks_format_by_length():
    from py_dfe.danfe.formatters import mask_cpf_cnpj
    assert mask_cpf_cnpj("12345678000199") == "12.345.678/0001-99"
    assert mask_cpf_cnpj("12345678909") == "123.456.789-09"
    assert mask_cpf_cnpj("") == ""
    assert mask_cpf_cnpj("123") == "123"  # unknown length → unchanged


def test_num_nf_groups_nine_digits():
    from py_dfe.danfe.formatters import num_nf
    assert num_nf("1") == "000.000.001"
    assert num_nf("123456789") == "123.456.789"
    assert num_nf("") == ""


def test_mask_cep():
    from py_dfe.danfe.formatters import mask_cep
    assert mask_cep("01000000") == "01000-000"
    assert mask_cep("123") == "123"


def test_pct():
    from py_dfe.danfe.formatters import pct
    assert pct("18.00") == "18,00"
    assert pct(None) == "0,00"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_formatters.py -q`
Expected: FAIL (`ImportError: cannot import name 'mask_cpf_cnpj'`).

- [ ] **Step 3: Append implementations**

Append to `py-dfe/py_dfe/danfe/formatters.py` (reuse `mask_cnpj`/`mask_cpf`/`money_br` already in the file):

```python
def mask_cpf_cnpj(digits: str) -> str:
    """Mask as CNPJ (14 digits) or CPF (11 digits); unchanged otherwise."""
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) == 14:
        return mask_cnpj(d)
    if len(d) == 11:
        return mask_cpf(d)
    return digits or ""


def num_nf(value: str) -> str:
    """NF-e number as '999.999.999' (9 digits, zero-padded)."""
    d = "".join(filter(str.isdigit, str(value or "")))
    if not d:
        return ""
    d = d.zfill(9)
    return f"{d[0:3]}.{d[3:6]}.{d[6:9]}"


def mask_cep(digits: str) -> str:
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) != 8:
        return digits or ""
    return f"{d[0:5]}-{d[5:8]}"


def pct(value: str | float | int | None) -> str:
    """Percentage/aliquota as '18,00' (reuses money_br grouping rules)."""
    return money_br(value)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_formatters.py -q`
Expected: PASS.

- [ ] **Step 5: Stage**

```bash
git add py-dfe/py_dfe/danfe/formatters.py py-dfe/tests/unit/test_danfe_formatters.py
# Suggested commit: feat(py-dfe): add CPF/CNPJ, NF number, CEP and percent formatters
```

---

### Task 3: Barcode module + dependency

**Files:**

- Create: `py-dfe/py_dfe/danfe/barcode.py`
- Modify: `py-dfe/pyproject.toml` (dependencies)
- Modify: `py-dfe/layer/requirements.txt`
- Test: `py-dfe/tests/unit/test_danfe_barcode.py`

**Interfaces:**

- Consumes: `DFeError`, `DANFE_INVALID_BARCODE` (Task 1).
- Produces:
    - `code128c_data_uri(value: str) -> str` — `data:image/svg+xml;base64,...`
    -
  `dados_nfe_code(*, cuf: str, tp_emis: str, doc: str, vnf: str, icms_proprio: bool, icms_st: bool, dia_emissao: str) -> str` —
  36-char string with mod-11 DV.
    - `_mod11_dv(digits: str) -> str` — single-char DV (also used internally).

- [ ] **Step 1: Add the dependency**

In `py-dfe/pyproject.toml`, add to the `dependencies` list (next to `segno`):

```toml
    "python-barcode>=0.15.1",
```

In `py-dfe/layer/requirements.txt`, add:

```
python-barcode==0.15.1
```

Install locally: `cd py-dfe && pip install "python-barcode>=0.15.1"`

- [ ] **Step 2: Write the failing test**

Create `py-dfe/tests/unit/test_danfe_barcode.py`:

```python
import base64

import pytest

from py_dfe.danfe.barcode import code128c_data_uri, dados_nfe_code, _mod11_dv
from py_dfe.exceptions import DFeError

_CHAVE = "35260612345678000199550010000000011000000017"


def test_code128c_data_uri_is_inline_svg():
    uri = code128c_data_uri(_CHAVE)
    assert uri.startswith("data:image/svg+xml;base64,")
    decoded = base64.b64decode(uri.split(",", 1)[1])
    assert b"<svg" in decoded


def test_code128c_rejects_empty():
    with pytest.raises(DFeError) as exc:
        code128c_data_uri("")
    assert exc.value.status_code == 422


def test_code128c_rejects_non_numeric():
    with pytest.raises(DFeError) as exc:
        code128c_data_uri("ABC123")
    assert exc.value.status_code == 422


def test_mod11_dv_matches_known_value():
    # Mod-11 of the 43-digit chave body yields the published DV "7".
    assert _mod11_dv("3526061234567800019955001000000001100000001") == "7"


def test_dados_nfe_code_layout():
    code = dados_nfe_code(
        cuf="35", tp_emis="2", doc="12345678000199", vnf="123.45",
        icms_proprio=True, icms_st=False, dia_emissao="25",
    )
    # cUF(2)+tpEmis(1)+doc(14)+vNF(14)+ICMSp(1)+ICMSs(1)+DD(2)+DV(1) = 36
    assert len(code) == 36
    assert code.startswith("35" + "2" + "12345678000199")
    # vNF right-aligned, zero-padded, no decimal point, centavos kept (12345)
    assert "00000000012345" in code
    assert code[31] == "1"  # ICMSp
    assert code[32] == "0"  # ICMSs
    assert code[33:35] == "25"  # DD
    assert code[-1] == _mod11_dv(code[:-1])
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_barcode.py -q`
Expected: FAIL (`ModuleNotFoundError: ... barcode`).

- [ ] **Step 4: Implement the module**

Create `py-dfe/py_dfe/danfe/barcode.py`:

```python
"""CODE-128 barcode rendering and FS/FS-DA 'Dados da NF-e' code (manual §3.9)."""

from __future__ import annotations

import base64
import io

from py_dfe.exceptions import DANFE_INVALID_BARCODE, DFeError


def _mod11_dv(digits: str) -> str:
    """Chave-style mod-11 check digit (weights 2..9 cycling, right→left)."""
    total = 0
    weight = 2
    for ch in reversed(digits):
        total += int(ch) * weight
        weight = 2 if weight == 9 else weight + 1
    rest = total % 11
    dv = 11 - rest
    if dv >= 10:
        dv = 0
    return str(dv)


def code128c_data_uri(value: str) -> str:
    """Render *value* (digits only) as a CODE-128 SVG inlined as a data: URI."""
    digits = "".join(filter(str.isdigit, value or ""))
    if not value or digits != value:
        raise DFeError(
            422, DANFE_INVALID_BARCODE,
            "Barcode value must be a non-empty numeric string",
        )
    # Imported lazily — keeps module import cheap and Lambda-cold-start lean.
    from barcode import Code128
    from barcode.writer import SVGWriter

    buf = io.BytesIO()
    Code128(digits, writer=SVGWriter()).write(
        buf,
        options={
            "module_width": 0.25,
            "module_height": 8.0,
            "quiet_zone": 2.0,
            "write_text": False,
        },
    )
    b64 = base64.b64encode(buf.getvalue()).decode("ascii")
    return f"data:image/svg+xml;base64,{b64}"


def dados_nfe_code(
    *,
    cuf: str,
    tp_emis: str,
    doc: str,
    vnf: str,
    icms_proprio: bool,
    icms_st: bool,
    dia_emissao: str,
) -> str:
    """Build the 36-char 'Dados da NF-e' content for FS/FS-DA (manual §3.9.2).

    Layout: cUF(2) tpEmis(1) doc(14) vNF(14) ICMSp(1) ICMSs(1) DD(2) DV(1).
    All numeric, right-aligned, zero-padded; DV is chave-style mod-11.
    """
    cuf_f = "".join(filter(str.isdigit, cuf)).zfill(2)[-2:]
    tp_f = "".join(filter(str.isdigit, tp_emis)).zfill(1)[-1:]
    doc_f = "".join(filter(str.isdigit, doc)).zfill(14)[-14:]
    cents = "".join(filter(str.isdigit, vnf))  # vNF already 'centavos in last 2'
    vnf_f = cents.zfill(14)[-14:]
    icmsp_f = "1" if icms_proprio else "2"
    icmss_f = "1" if icms_st else "2"
    dd_f = "".join(filter(str.isdigit, dia_emissao)).zfill(2)[-2:]
    body = cuf_f + tp_f + doc_f + vnf_f + icmsp_f + icmss_f + dd_f
    return body + _mod11_dv(body)
```

> Note: the test's `icms_proprio=True` expects `code[31] == "1"`. Index map: cUF `0:2`, tpEmis `2:3`, doc `3:17`, vNF
> `17:31`, ICMSp `31`, ICMSs `32`, DD `33:35`, DV `35`. The `vnf="123.45"` test strips to digits `12345` → zero-padded
> to
> `00000000012345`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_barcode.py -q`
Expected: PASS. If `test_dados_nfe_code_layout` asserts on `vNF`, confirm the caller passes vNF with centavos already
encoded (Task 6 passes the digits of `vNF` formatted as integer centavos).

- [ ] **Step 6: Stage**

```bash
git add py-dfe/py_dfe/danfe/barcode.py py-dfe/pyproject.toml py-dfe/layer/requirements.txt py-dfe/tests/unit/test_danfe_barcode.py
# Suggested commit: feat(py-dfe): add CODE-128 barcode + FS/FS-DA dados-nfe code
```

---

### Task 4: Render extension (`fit_height` flag)

**Files:**

- Modify: `py-dfe/py_dfe/danfe/render.py:35-55` (`htmls_to_pdf`)
- Test: `py-dfe/tests/unit/test_danfe_render.py` (append)

**Interfaces:**

- Consumes: existing `_render_fitted`, `_content_height_px`.
- Produces: `htmls_to_pdf(pages: list[str], *, fit_height: bool = True) -> bytes` (backward compatible).
  `html_to_pdf(html: str) -> bytes` unchanged.

- [ ] **Step 1: Write the failing test**

Append to `py-dfe/tests/unit/test_danfe_render.py`:

```python
def test_fixed_size_paginates_a4():
    pytest.importorskip("weasyprint")
    from pypdf import PdfReader
    import io as _io
    from py_dfe.danfe.render import htmls_to_pdf

    # Two A4 pages forced via CSS; fit_height=False must keep both.
    html = """<html><head><style>
      @page { size: A4 portrait; margin: 5mm; }
      .pg { page-break-after: always; height: 280mm; }
    </style></head><body>
      <div class="pg">PAGE ONE</div>
      <div class="pg">PAGE TWO</div>
    </body></html>"""
    pdf = htmls_to_pdf([html], fit_height=False)
    assert pdf[:4] == b"%PDF"
    reader = PdfReader(_io.BytesIO(pdf))
    assert len(reader.pages) == 2


def test_fit_height_default_unchanged():
    pytest.importorskip("weasyprint")
    from py_dfe.danfe.render import htmls_to_pdf
    pdf = htmls_to_pdf(["<html><body><p>hi</p></body></html>"])
    assert pdf[:4] == b"%PDF"
```

(If `pypdf` is not already a test dep, add `pypdf` to the dev/test deps in `pyproject.toml`; it is pure-Python.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_render.py::test_fixed_size_paginates_a4 -q`
Expected: FAIL (`htmls_to_pdf() got an unexpected keyword argument 'fit_height'`).

- [ ] **Step 3: Modify `htmls_to_pdf`**

Replace the body of `htmls_to_pdf` in `py-dfe/py_dfe/danfe/render.py` with:

```python
def htmls_to_pdf(pages: list[str], *, fit_height: bool = True) -> bytes:
    """Render HTML strings to one merged PDF.

    fit_height=True  → each page measured and snugged to its content height
                       (thermal / roll: NFC-e, DANFE simplificado/etiqueta).
    fit_height=False → honor each document's own ``@page`` size and let it
                       paginate naturally (fixed A4: DANFE retrato/paisagem).
    """
    try:
        from weasyprint import HTML  # imported lazily (heavy native deps)

        base = str(_TEMPLATE_DIR)
        if fit_height:
            docs = [_render_fitted(HTML, base, html) for html in pages]
        else:
            docs = [HTML(string=html, base_url=base).render() for html in pages]
        if not docs:
            return HTML(string="", base_url=base).write_pdf()
        merged_pages = [page for doc in docs for page in doc.pages]
        return docs[0].copy(merged_pages).write_pdf()
    except DFeError:
        raise
    except Exception as exc:  # noqa: BLE001 - wrap everything as DFeError
        raise DFeError(500, DANFE_RENDER_FAILED, f"PDF render failed: {exc}") from exc
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_render.py -q`
Expected: PASS (both new tests + existing render tests).

- [ ] **Step 5: Stage**

```bash
git add py-dfe/py_dfe/danfe/render.py py-dfe/tests/unit/test_danfe_render.py py-dfe/pyproject.toml
# Suggested commit: feat(py-dfe): add fixed-size multi-page mode to PDF renderer
```

---

### Task 5: Mod-55 test fixture

**Files:**

- Modify: `py-dfe/tests/danfe_fixtures.py` (append `sample_nfe55_proc`)
- Test: `py-dfe/tests/unit/test_danfe_fixtures.py` (append)

**Interfaces:**

- Produces:
  `sample_nfe55_proc(*, tp_emis="1", tp_amb="1", n_items=2, with_transp=True, with_dup=True, with_issqn=False) -> str`.

- [ ] **Step 1: Write the failing test**

Append to `py-dfe/tests/unit/test_danfe_fixtures.py`:

```python
def test_sample_nfe55_proc_is_model_55():
    from tests.danfe_fixtures import sample_nfe55_proc
    xml = sample_nfe55_proc(n_items=3)
    assert "<mod>55</mod>" in xml
    assert xml.count("<det") == 3
    assert "<transp>" in xml
    assert "<dup>" in xml


def test_sample_nfe55_proc_contingency_flags():
    from tests.danfe_fixtures import sample_nfe55_proc
    assert "<tpEmis>2</tpEmis>" in sample_nfe55_proc(tp_emis="2")
    assert "<tpAmb>2</tpAmb>" in sample_nfe55_proc(tp_amb="2")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_fixtures.py -q`
Expected: FAIL (`ImportError: cannot import name 'sample_nfe55_proc'`).

- [ ] **Step 3: Append the fixture**

Append to `py-dfe/tests/danfe_fixtures.py` (reuses module-level `_NS`):

```python
_CHAVE_55 = "35260612345678000199550010000000011000000017"


def _items55(n: int) -> str:
    rows = []
    for i in range(1, n + 1):
        rows.append(
            f"""<det nItem="{i}">
  <prod>
    <cProd>P{i:03d}</cProd>
    <xProd>PRODUTO TESTE {i}</xProd>
    <NCM>61099000</NCM>
    <CFOP>5102</CFOP>
    <uCom>UN</uCom>
    <qCom>2.0000</qCom>
    <vUnCom>10.0000000000</vUnCom>
    <vProd>20.00</vProd>
  </prod>
  <imposto>
    <ICMS>
      <ICMS00>
        <CST>00</CST>
        <vBC>20.00</vBC>
        <pICMS>18.00</pICMS>
        <vICMS>3.60</vICMS>
      </ICMS00>
    </ICMS>
  </imposto>
</det>"""
        )
    return "\n".join(rows)


def sample_nfe55_proc(
    *,
    tp_emis: str = "1",
    tp_amb: str = "1",
    n_items: int = 2,
    with_transp: bool = True,
    with_dup: bool = True,
    with_issqn: bool = False,
) -> str:
    total_prod = f"{20.00 * n_items:.2f}"
    transp = (
        """<transp>
    <modFrete>0</modFrete>
    <transporta>
      <xNome>TRANSPORTADORA TESTE LTDA</xNome>
      <CNPJ>98765432000188</CNPJ>
      <IE>ISENTO</IE>
      <xEnder>RUA DO FRETE, 50</xEnder>
      <xMun>SAO PAULO</xMun>
      <UF>SP</UF>
    </transporta>
    <veicTransp><placa>ABC1D23</placa><UF>SP</UF></veicTransp>
    <vol><qVol>1</qVol><esp>CAIXA</esp><pesoB>1.000</pesoB><pesoL>0.900</pesoL></vol>
  </transp>"""
        if with_transp
        else ""
    )
    cobr = (
        f"""<cobr>
    <fat><nFat>001</nFat><vOrig>{total_prod}</vOrig><vLiq>{total_prod}</vLiq></fat>
    <dup><nDup>001</nDup><dVenc>2026-07-25</dVenc><vDup>{total_prod}</vDup></dup>
  </cobr>"""
        if with_dup
        else ""
    )
    issqn = (
        """<ISSQNtot><vServ>0.00</vServ><vBC>0.00</vBC><vISS>0.00</vISS></ISSQNtot>"""
        if with_issqn
        else ""
    )
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="{_NS}" versao="4.00">
  <NFe>
    <infNFe Id="NFe{_CHAVE_55}" versao="4.00">
      <ide>
        <cUF>35</cUF>
        <natOp>VENDA DE MERCADORIA</natOp>
        <mod>55</mod>
        <serie>1</serie>
        <nNF>1</nNF>
        <dhEmi>2026-06-25T10:30:00-03:00</dhEmi>
        <dhSaiEnt>2026-06-25T10:35:00-03:00</dhSaiEnt>
        <tpNF>1</tpNF>
        <tpAmb>{tp_amb}</tpAmb>
        <tpEmis>{tp_emis}</tpEmis>
      </ide>
      <emit>
        <CNPJ>12345678000199</CNPJ>
        <xNome>EMPRESA TESTE LTDA</xNome>
        <xFant>EMPRESA TESTE</xFant>
        <enderEmit>
          <xLgr>RUA EXEMPLO</xLgr>
          <nro>100</nro>
          <xBairro>CENTRO</xBairro>
          <xMun>SAO PAULO</xMun>
          <UF>SP</UF>
          <CEP>01000000</CEP>
          <fone>1133334444</fone>
        </enderEmit>
        <IE>110042490114</IE>
        <CRT>3</CRT>
      </emit>
      <dest>
        <CNPJ>98765432000188</CNPJ>
        <xNome>CLIENTE TESTE LTDA</xNome>
        <enderDest>
          <xLgr>AV CLIENTE</xLgr>
          <nro>200</nro>
          <xBairro>JARDIM</xBairro>
          <xMun>CAMPINAS</xMun>
          <UF>SP</UF>
          <CEP>13000000</CEP>
        </enderDest>
        <IE>ISENTO</IE>
      </dest>
      {_items55(n_items)}
      <total>
        <ICMSTot>
          <vBC>{total_prod}</vBC>
          <vICMS>{20.00 * n_items * 0.18:.2f}</vICMS>
          <vBCST>0.00</vBCST>
          <vST>0.00</vST>
          <vProd>{total_prod}</vProd>
          <vFrete>0.00</vFrete>
          <vSeg>0.00</vSeg>
          <vDesc>0.00</vDesc>
          <vOutro>0.00</vOutro>
          <vIPI>0.00</vIPI>
          <vNF>{total_prod}</vNF>
          <vTotTrib>5.00</vTotTrib>
        </ICMSTot>
        {issqn}
      </total>
      {transp}
      {cobr}
      <infAdic>
        <infAdFisco>Informacao ao fisco de teste</infAdFisco>
        <infCpl>Documento emitido por ME optante pelo Simples Nacional</infCpl>
      </infAdic>
    </infNFe>
  </NFe>
  <protNFe versao="4.00">
    <infProt>
      <chNFe>{_CHAVE_55}</chNFe>
      <nProt>135260000000099</nProt>
      <dhRecbto>2026-06-25T10:30:05-03:00</dhRecbto>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso da NF-e</xMotivo>
    </infProt>
  </protNFe>
</nfeProc>"""
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_fixtures.py -q`
Expected: PASS.

- [ ] **Step 5: Stage**

```bash
git add py-dfe/tests/danfe_fixtures.py py-dfe/tests/unit/test_danfe_fixtures.py
# Suggested commit: test(py-dfe): add synthetic mod-55 nfeProc fixture
```

---

### Task 6: Mod-55 context builder + generator (`nfe55.py`)

**Files:**

- Create: `py-dfe/py_dfe/danfe/nfe55.py`
- Test: `py-dfe/tests/unit/test_danfe_nfe.py`

**Interfaces:**

- Consumes: `parse_xml_bytes` (`py_dfe.xmlops.builder`), `formatters` (Task 2), `barcode` (Task 3),
  `render.render_html` + `render.htmls_to_pdf` (Task 4), constants (Task 1), `sample_nfe55_proc` (Task 5).
- Produces:
    - `generate_danfe_nfe(payload: dict) -> dict` → `{"pdf_b64", "html"}`.
    - `build_context(inf_nfe, prot, *, layout, canceled, tp_emis, tp_amb, chave) -> dict`.
    - `_extract_roots(xml) -> tuple[dict, dict|None, str, str, str]` (validates mod 55).

> This task creates the generator and its context. The HTML templates do not exist yet (Tasks 7-8), so the PDF-rendering
> assertions in this task's tests guard with `importorskip` AND are written to tolerate a missing template by asserting
> on
> `build_context` output (pure, no WeasyPrint). Full end-to-end PDF assertions live in Tasks 7-8 and 9 once templates
> exist.

- [ ] **Step 1: Write the failing test (context only — no template needed)**

Create `py-dfe/tests/unit/test_danfe_nfe.py`:

```python
import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.nfe55 import _extract_roots, build_context
from py_dfe.exceptions import DFeError
from tests.danfe_fixtures import sample_nfe55_proc


def _ctx(**kw):
    xml = sample_nfe55_proc(**kw)
    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    return build_context(
        inf_nfe, prot, layout=c.LAYOUT_RETRATO, canceled=False,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave,
    )


def test_extract_roots_rejects_non_55():
    bad = sample_nfe55_proc().replace("<mod>55</mod>", "<mod>65</mod>")
    with pytest.raises(DFeError) as exc:
        _extract_roots(bad)
    assert exc.value.status_code == 422


def test_context_core_fields():
    ctx = _ctx(n_items=2)
    assert ctx["emit"]["cnpj"] == "12.345.678/0001-99"
    assert ctx["dest"]["nome"] == "CLIENTE TESTE LTDA"
    assert ctx["ide"]["tpNF_label"] == "SAÍDA"
    assert ctx["ide"]["nNF"] == "000.000.001"
    assert len(ctx["items"]) == 2
    assert ctx["items"][0]["NCM"] == "61099000"
    assert ctx["chave_fmt"].count(" ") == 10  # 11 blocks → 10 spaces
    assert ctx["chave_barcode"].startswith("data:image/svg+xml;base64,")
    assert ctx["show_protocolo"] is True
    assert ctx["is_fs"] is False
    assert ctx["transp"]["modFrete_label"] == "0 - Remetente"
    assert ctx["duplicatas"][0]["nDup"] == "001"


def test_context_fs_contingency_emits_second_barcode():
    ctx = _ctx(tp_emis=c.TP_EMIS_FS)
    assert ctx["is_fs"] is True
    assert ctx["show_protocolo"] is False
    assert ctx["dados_nfe_barcode"].startswith("data:image/svg+xml;base64,")
    assert len(ctx["dados_nfe_code"]) == 36


def test_context_epec_keeps_protocolo_label():
    ctx = _ctx(tp_emis=c.TP_EMIS_EPEC)
    assert ctx["is_epec"] is True
    assert ctx["show_protocolo"] is True
    assert ctx["protocolo_label"] == c.TEXT_PROTOCOLO_EPEC


def test_context_homologacao_and_cancelada():
    ctx = _ctx(tp_amb=c.TP_AMB_HOMOLOGACAO)
    assert ctx["is_homologacao"] is True
    xml = sample_nfe55_proc()
    inf_nfe, prot, te, ta, ch = _extract_roots(xml)
    ctx2 = build_context(inf_nfe, prot, layout=c.LAYOUT_RETRATO, canceled=True,
                         tp_emis=te, tp_amb=ta, chave=ch)
    assert ctx2["is_cancelada"] is True
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_nfe.py -q`
Expected: FAIL (`ModuleNotFoundError: ... nfe55`).

- [ ] **Step 3: Implement `nfe55.py`**

Create `py-dfe/py_dfe/danfe/nfe55.py`:

```python
"""DANF-e (NF-e modelo 55) generation from an authorized NF-e XML."""

from __future__ import annotations

import base64
from decimal import Decimal, InvalidOperation
from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe import formatters as fmt
from py_dfe.danfe.barcode import code128c_data_uri, dados_nfe_code
from py_dfe.danfe.render import htmls_to_pdf, render_html
from py_dfe.exceptions import DANFE_INVALID_XML, DANFE_UNSUPPORTED_MODEL, DFeError
from py_dfe.xmlops.builder import parse_xml_bytes


def generate_danfe_nfe(payload: dict[str, Any]) -> dict[str, Any]:
    """Render a DANF-e (mod 55) from an authorized NF-e XML payload."""
    xml = payload.get("xml")
    if not xml:
        raise DFeError(422, DANFE_INVALID_XML, "Missing 'xml' in body")
    layout = payload.get("layout", c.DEFAULT_DANFE_NFE_LAYOUT)
    if layout not in c.VALID_DANFE_NFE_LAYOUTS:
        layout = c.DEFAULT_DANFE_NFE_LAYOUT
    canceled = bool(payload.get("canceled", False))

    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    context = build_context(
        inf_nfe, prot, layout=layout, canceled=canceled,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave,
    )
    template = c.DANFE_NFE_TEMPLATES[layout]
    fit_height = layout in c.ROLL_LAYOUTS
    html = render_html(template, context)
    pdf = htmls_to_pdf([html], fit_height=fit_height)
    return {"pdf_b64": base64.b64encode(pdf).decode("ascii"), "html": [html]}


def _extract_roots(xml: str) -> tuple[dict, dict | None, str, str, str]:
    try:
        parsed = parse_xml_bytes(xml.encode("utf-8") if isinstance(xml, str) else xml)
    except etree.XMLSyntaxError as exc:
        raise DFeError(422, DANFE_INVALID_XML, f"Malformed XML: {exc}") from exc

    root = next(iter(parsed.values()))
    nfe = root.get("NFe", root)
    prot = root.get("protNFe") if isinstance(root, dict) else None
    inf_nfe = nfe.get("infNFe") if isinstance(nfe, dict) else None
    if not isinstance(inf_nfe, dict):
        raise DFeError(422, DANFE_INVALID_XML, "infNFe not found")

    ide = inf_nfe.get("ide", {})
    if ide.get("mod") != c.MODELO_NFE:
        raise DFeError(
            422, DANFE_UNSUPPORTED_MODEL,
            f"DANF-e requires model 55, got {ide.get('mod')!r}",
        )
    tp_emis = ide.get("tpEmis", c.TP_EMIS_NORMAL)
    tp_amb = ide.get("tpAmb", c.TP_AMB_PRODUCAO)
    chave = _chave(inf_nfe, prot)
    return inf_nfe, prot, tp_emis, tp_amb, chave


def _chave(inf_nfe: dict, prot: dict | None) -> str:
    if prot:
        ch = (prot.get("infProt") or {}).get("chNFe")
        if ch:
            return ch
    return (inf_nfe.get("@Id") or "").replace("NFe", "")


def _centavos(value: str | None) -> str:
    """Integer centavos string for the FS/FS-DA barcode (e.g. '123.45'→'12345')."""
    try:
        dec = Decimal(str(value or "0"))
    except (InvalidOperation, ValueError):
        dec = Decimal(0)
    return str(int((dec * 100).to_integral_value()))


def _as_list(node: Any) -> list:
    if node is None:
        return []
    return node if isinstance(node, list) else [node]


def build_context(
    inf_nfe: dict, prot: dict | None, *, layout: str, canceled: bool,
    tp_emis: str, tp_amb: str, chave: str,
) -> dict[str, Any]:
    is_contingencia = tp_emis not in c.TP_EMIS_NORMAL_LIKE
    is_fs = tp_emis in c.TP_EMIS_FS_LIKE
    is_epec = tp_emis == c.TP_EMIS_EPEC
    is_homologacao = tp_amb == c.TP_AMB_HOMOLOGACAO
    show_protocolo = not is_fs  # FS/FS-DA not yet authorized

    ide = inf_nfe.get("ide", {})
    emit = inf_nfe.get("emit", {})
    ender_e = emit.get("enderEmit", {})
    dest = inf_nfe.get("dest", {}) or {}
    ender_d = dest.get("enderDest", {})

    items = []
    for d in _as_list(inf_nfe.get("det")):
        p = d.get("prod", {})
        imp = d.get("imposto", {}) or {}
        icms_grp = next(iter((imp.get("ICMS") or {}).values()), {}) if imp.get("ICMS") else {}
        if not isinstance(icms_grp, dict):
            icms_grp = {}
        items.append({
            "cProd": p.get("cProd", ""),
            "xProd": p.get("xProd", ""),
            "NCM": p.get("NCM", ""),
            "CFOP": p.get("CFOP", ""),
            "uCom": p.get("uCom", ""),
            "qCom": p.get("qCom", ""),
            "vUnCom": fmt.money_br(p.get("vUnCom")),
            "vProd": fmt.money_br(p.get("vProd")),
            "CST": icms_grp.get("CST") or icms_grp.get("CSOSN") or "",
            "vBC": fmt.money_br(icms_grp.get("vBC")),
            "vICMS": fmt.money_br(icms_grp.get("vICMS")),
            "pICMS": fmt.pct(icms_grp.get("pICMS")),
            "infAdProd": d.get("infAdProd", ""),
        })

    icms = (inf_nfe.get("total", {}) or {}).get("ICMSTot", {})
    totals = {k: fmt.money_br(icms.get(v)) for k, v in {
        "vBC": "vBC", "vICMS": "vICMS", "vBCST": "vBCST", "vST": "vST",
        "vProd": "vProd", "vFrete": "vFrete", "vSeg": "vSeg", "vDesc": "vDesc",
        "vOutro": "vOutro", "vIPI": "vIPI", "vNF": "vNF", "vTotTrib": "vTotTrib",
    }.items()}

    transp = inf_nfe.get("transp", {}) or {}
    transporta = transp.get("transporta", {}) or {}
    veic = transp.get("veicTransp", {}) or {}
    vol = next(iter(_as_list(transp.get("vol"))), {}) or {}
    transp_ctx = {
        "modFrete_label": c.MOD_FRETE_LABELS.get(transp.get("modFrete", ""), transp.get("modFrete", "")),
        "nome": transporta.get("xNome", ""),
        "doc": fmt.mask_cpf_cnpj(transporta.get("CNPJ") or transporta.get("CPF") or ""),
        "ie": transporta.get("IE", ""),
        "ender": transporta.get("xEnder", ""),
        "mun": transporta.get("xMun", ""),
        "uf": transporta.get("UF", ""),
        "placa": veic.get("placa", ""),
        "placa_uf": veic.get("UF", ""),
        "qVol": vol.get("qVol", ""),
        "esp": vol.get("esp", ""),
        "marca": vol.get("marca", ""),
        "pesoB": vol.get("pesoB", ""),
        "pesoL": vol.get("pesoL", ""),
    }

    cobr = inf_nfe.get("cobr", {}) or {}
    fat = cobr.get("fat", {}) or {}
    duplicatas = [{
        "nDup": dp.get("nDup", ""),
        "dVenc": dp.get("dVenc", ""),
        "vDup": fmt.money_br(dp.get("vDup")),
    } for dp in _as_list(cobr.get("dup"))]

    infadic = inf_nfe.get("infAdic", {}) or {}

    protocolo = None
    if show_protocolo and prot:
        ip = prot.get("infProt", {})
        protocolo = {"nProt": ip.get("nProt", ""), "dhRecbto": fmt.dt_local(ip.get("dhRecbto"))}

    dados_nfe_barcode = ""
    dados_nfe_code_str = ""
    if is_fs:
        dia = (ide.get("dhEmi", "") or "")[8:10] or "01"
        cuf = (chave or "")[0:2]
        doc_dest = dest.get("CNPJ") or dest.get("CPF") or "0"
        dados_nfe_code_str = dados_nfe_code(
            cuf=cuf, tp_emis=tp_emis, doc=doc_dest, vnf=_centavos(icms.get("vNF")),
            icms_proprio=bool(icms.get("vICMS") and icms.get("vICMS") not in ("0", "0.00")),
            icms_st=bool(icms.get("vST") and icms.get("vST") not in ("0", "0.00")),
            dia_emissao=dia,
        )
        dados_nfe_barcode = code128c_data_uri(dados_nfe_code_str)

    return {
        "layout": layout,
        "emit": {
            "nome": emit.get("xNome", ""),
            "fantasia": emit.get("xFant", ""),
            "cnpj": fmt.mask_cnpj(emit.get("CNPJ", "")),
            "ie": emit.get("IE", ""),
            "iest": emit.get("IEST", ""),
            "endereco": _endereco(ender_e),
            "cep": fmt.mask_cep(ender_e.get("CEP", "")),
            "mun": ender_e.get("xMun", ""),
            "uf": ender_e.get("UF", ""),
            "fone": ender_e.get("fone", ""),
        },
        "dest": {
            "nome": dest.get("xNome", ""),
            "doc": fmt.mask_cpf_cnpj(dest.get("CNPJ") or dest.get("CPF") or ""),
            "ie": dest.get("IE", ""),
            "endereco": _endereco(ender_d),
            "cep": fmt.mask_cep(ender_d.get("CEP", "")),
            "mun": ender_d.get("xMun", ""),
            "uf": ender_d.get("UF", ""),
            "fone": ender_d.get("fone", ""),
        },
        "ide": {
            "natOp": ide.get("natOp", ""),
            "tpNF": ide.get("tpNF", ""),
            "tpNF_label": c.TP_NF_LABELS.get(ide.get("tpNF", ""), ""),
            "nNF": fmt.num_nf(ide.get("nNF", "")),
            "serie": ide.get("serie", ""),
            "dhEmi": fmt.dt_local(ide.get("dhEmi")),
            "dhSaiEnt": fmt.dt_local(ide.get("dhSaiEnt")),
        },
        "items": items,
        "totals": totals,
        "transp": transp_ctx,
        "fatura": {
            "nFat": fat.get("nFat", ""),
            "vOrig": fmt.money_br(fat.get("vOrig")),
            "vLiq": fmt.money_br(fat.get("vLiq")),
        },
        "duplicatas": duplicatas,
        "chave_fmt": fmt.chave_blocks(chave),
        "chave_raw": "".join(filter(str.isdigit, chave or "")),
        "chave_barcode": code128c_data_uri(chave),
        "protocolo": protocolo,
        "protocolo_label": c.TEXT_PROTOCOLO_EPEC if is_epec else c.TEXT_PROTOCOLO,
        "show_protocolo": show_protocolo,
        "is_contingencia": is_contingencia,
        "is_fs": is_fs,
        "is_epec": is_epec,
        "is_homologacao": is_homologacao,
        "is_cancelada": canceled,
        "dados_nfe_code": dados_nfe_code_str,
        "dados_nfe_barcode": dados_nfe_barcode,
        "msg_fiscal": infadic.get("infAdFisco", ""),
        "msg_contribuinte": infadic.get("infCpl", ""),
        "text": {
            "danfe": c.TEXT_DANFE,
            "danfe_desc": c.TEXT_DANFE_DESC,
            "simplificado": c.TEXT_DANFE_SIMPLIFICADO,
            "etiqueta": c.TEXT_DANFE_ETIQUETA,
            "homologacao": c.TEXT_NFE_HOMOLOGACAO,
            "contingencia": c.TEXT_NFE_CONTINGENCIA,
            "consulta": c.TEXT_CONSULTA_NFE,
            "dados_nfe": c.TEXT_DADOS_NFE,
            "cancelada": c.TEXT_WATERMARK_CANCELADA,
        },
    }


def _endereco(ender: dict) -> str:
    parts = [
        ender.get("xLgr", ""), ender.get("nro", ""), ender.get("xCpl", ""),
        ender.get("xBairro", ""),
    ]
    return ", ".join(p for p in parts if p)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_nfe.py -q`
Expected: PASS (context tests; PDF tests come in Tasks 7-9).

- [ ] **Step 5: Stage**

```bash
git add py-dfe/py_dfe/danfe/nfe55.py py-dfe/tests/unit/test_danfe_nfe.py
# Suggested commit: feat(py-dfe): add mod-55 DANF-e context builder and generator
```

---

### Task 7: Macros + retrato/paisagem templates (full A4)

**Files:**

- Create: `py-dfe/py_dfe/danfe/templates/_danfe_macros.html`
- Create: `py-dfe/py_dfe/danfe/templates/danfe_retrato.html`
- Create: `py-dfe/py_dfe/danfe/templates/danfe_paisagem.html`
- Test: `py-dfe/tests/unit/test_danfe_nfe.py` (append PDF + HTML assertions)

**Interfaces:**

- Consumes: context from `build_context` (Task 6), `generate_danfe_nfe` (Task 6).
- Produces: rendered HTML containing all mandatory quadros; valid `%PDF` for retrato/paisagem.

- [ ] **Step 1: Write the failing test**

Append to `py-dfe/tests/unit/test_danfe_nfe.py`:

```python
import base64 as _b64
from py_dfe.danfe.nfe55 import generate_danfe_nfe


@pytest.mark.parametrize("layout", [c.LAYOUT_RETRATO, c.LAYOUT_PAISAGEM])
def test_a4_variants_render_pdf_with_all_quadros(layout):
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": layout})
    assert _b64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    html = out["html"][0]
    assert c.TEXT_DANFE in html
    assert c.TEXT_DANFE_DESC in html
    assert "EMPRESA TESTE LTDA" in html        # emit
    assert "CLIENTE TESTE LTDA" in html        # dest
    assert "VENDA DE MERCADORIA" in html       # natOp
    assert "PRODUTO TESTE 1" in html           # produtos
    assert "TRANSPORTADORA TESTE LTDA" in html # transp
    assert "001" in html                       # duplicata
    assert "PROTOCOLO DE AUTORIZAÇÃO DE USO" in html


def test_a4_homologacao_watermark():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(tp_amb="2")})
    assert "SEM VALOR FISCAL" in out["html"][0]


def test_a4_fs_contingency_has_second_barcode_no_protocolo():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(tp_emis=c.TP_EMIS_FS)})
    html = out["html"][0]
    assert "DADOS DA NF-E" in html
    assert "PROTOCOLO DE AUTORIZAÇÃO DE USO" not in html
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_nfe.py -k a4 -q`
Expected: FAIL (`weasyprint ... TemplateNotFound: danfe_retrato.html`).

- [ ] **Step 3: Create `_danfe_macros.html`**

Create `py-dfe/py_dfe/danfe/templates/_danfe_macros.html`:

```html
{% macro watermarks(ctx) %}
  {% if ctx.is_cancelada %}<div class="watermark">{{ ctx.text.cancelada }}</div>{% endif %}
  {% if ctx.is_homologacao %}<div class="watermark">{{ ctx.text.homologacao }}</div>{% endif %}
{% endmacro %}

{% macro header(ctx) %}
<div class="danfe-header">
  <table class="hdr"><tr>
    <td class="emit">
      <div class="b big">{{ ctx.emit.nome }}</div>
      <div>{{ ctx.emit.endereco }}</div>
      <div>{{ ctx.emit.mun }} - {{ ctx.emit.uf }} - CEP {{ ctx.emit.cep }}</div>
      <div>Fone: {{ ctx.emit.fone }}</div>
    </td>
    <td class="idbox center">
      <div class="b xbig">{{ ctx.text.danfe }}</div>
      <div class="small">{{ ctx.text.danfe_desc }}</div>
      <div>{{ ctx.ide.tpNF }} - {{ ctx.ide.tpNF_label }}</div>
      <div class="b">Nº {{ ctx.ide.nNF }}</div>
      <div class="b">SÉRIE {{ ctx.ide.serie }}</div>
      <div>FOLHA <span class="pageno"></span></div>
    </td>
    <td class="keybox center">
      <img class="barcode" src="{{ ctx.chave_barcode }}" alt="chave">
      <div class="small b">{{ ctx.chave_fmt }}</div>
      {% if ctx.is_fs %}
      <div class="small b">{{ ctx.text.dados_nfe }}</div>
      <img class="barcode" src="{{ ctx.dados_nfe_barcode }}" alt="dados nf-e">
      {% endif %}
    </td>
  </tr></table>
  <div class="natop"><span class="lbl">NATUREZA DA OPERAÇÃO</span> {{ ctx.ide.natOp }}</div>
  {% if ctx.show_protocolo and ctx.protocolo %}
  <div class="prot"><span class="lbl">{{ ctx.protocolo_label }}</span>
    {{ ctx.protocolo.nProt }} {{ ctx.protocolo.dhRecbto }}</div>
  {% endif %}
  {% if ctx.is_contingencia %}<div class="banner">{{ ctx.text.contingencia }}</div>{% endif %}
</div>
{% endmacro %}

{% macro dest_box(ctx) %}
<table class="box"><tr><td>
  <span class="lbl">DESTINATÁRIO / REMETENTE</span>
  <div class="b">{{ ctx.dest.nome }}</div>
  <div>CNPJ/CPF: {{ ctx.dest.doc }} &nbsp; IE: {{ ctx.dest.ie }}</div>
  <div>{{ ctx.dest.endereco }} - {{ ctx.dest.mun }}/{{ ctx.dest.uf }} - CEP {{ ctx.dest.cep }}</div>
  <div>EMISSÃO: {{ ctx.ide.dhEmi }} &nbsp; SAÍDA/ENTRADA: {{ ctx.ide.dhSaiEnt }}</div>
</td></tr></table>
{% endmacro %}

{% macro fatura_box(ctx) %}
{% if ctx.duplicatas %}
<table class="box dup"><tr><td><span class="lbl">FATURA / DUPLICATAS</span>
  {% for d in ctx.duplicatas %}<span class="dupitem">{{ d.nDup }} {{ d.dVenc }} R$ {{ d.vDup }}</span>{% endfor %}
</td></tr></table>
{% endif %}
{% endmacro %}

{% macro imposto_box(ctx) %}
<table class="box totals">
  <tr><td>BC ICMS</td><td class="num">{{ ctx.totals.vBC }}</td>
      <td>VALOR ICMS</td><td class="num">{{ ctx.totals.vICMS }}</td>
      <td>BC ICMS ST</td><td class="num">{{ ctx.totals.vBCST }}</td>
      <td>VALOR ICMS ST</td><td class="num">{{ ctx.totals.vST }}</td></tr>
  <tr><td>VALOR PRODUTOS</td><td class="num">{{ ctx.totals.vProd }}</td>
      <td>FRETE</td><td class="num">{{ ctx.totals.vFrete }}</td>
      <td>SEGURO</td><td class="num">{{ ctx.totals.vSeg }}</td>
      <td>DESCONTO</td><td class="num">{{ ctx.totals.vDesc }}</td></tr>
  <tr><td>OUTRAS DESPESAS</td><td class="num">{{ ctx.totals.vOutro }}</td>
      <td>VALOR IPI</td><td class="num">{{ ctx.totals.vIPI }}</td>
      <td class="b">VALOR TOTAL NOTA</td><td class="num b">{{ ctx.totals.vNF }}</td>
      <td>TRIB. APROX.</td><td class="num">{{ ctx.totals.vTotTrib }}</td></tr>
</table>
{% endmacro %}

{% macro transp_box(ctx) %}
<table class="box"><tr><td>
  <span class="lbl">TRANSPORTADOR / VOLUMES TRANSPORTADOS</span>
  <div>{{ ctx.transp.nome }} &nbsp; FRETE: {{ ctx.transp.modFrete_label }}
       &nbsp; PLACA: {{ ctx.transp.placa }}/{{ ctx.transp.placa_uf }}</div>
  <div>CNPJ/CPF: {{ ctx.transp.doc }} &nbsp; IE: {{ ctx.transp.ie }}
       &nbsp; {{ ctx.transp.ender }} - {{ ctx.transp.mun }}/{{ ctx.transp.uf }}</div>
  <div>QTD: {{ ctx.transp.qVol }} &nbsp; ESPÉCIE: {{ ctx.transp.esp }}
       &nbsp; MARCA: {{ ctx.transp.marca }}
       &nbsp; PESO B: {{ ctx.transp.pesoB }} &nbsp; PESO L: {{ ctx.transp.pesoL }}</div>
</td></tr></table>
{% endmacro %}

{% macro produtos_table(ctx) %}
<table class="box items">
  <colgroup>
    <col style="width:9%"><col style="width:31%"><col style="width:8%">
    <col style="width:5%"><col style="width:5%"><col style="width:5%">
    <col style="width:7%"><col style="width:8%"><col style="width:8%">
    <col style="width:8%"><col style="width:6%">
  </colgroup>
  <thead><tr>
    <td>CÓDIGO</td><td>DESCRIÇÃO</td><td>NCM</td><td>CST</td><td>CFOP</td>
    <td>UN</td><td class="num">QTD</td><td class="num">V.UNIT</td>
    <td class="num">V.TOTAL</td><td class="num">BC ICMS</td><td class="num">ALIQ</td>
  </tr></thead>
  <tbody>
  {% for it in ctx.items %}
  <tr>
    <td>{{ it.cProd }}</td><td>{{ it.xProd }}</td><td>{{ it.NCM }}</td>
    <td>{{ it.CST }}</td><td>{{ it.CFOP }}</td><td>{{ it.uCom }}</td>
    <td class="num">{{ it.qCom }}</td><td class="num">{{ it.vUnCom }}</td>
    <td class="num">{{ it.vProd }}</td><td class="num">{{ it.vBC }}</td>
    <td class="num">{{ it.pICMS }}</td>
  </tr>
  {% if it.infAdProd %}<tr><td></td><td colspan="10" class="small">{{ it.infAdProd }}</td></tr>{% endif %}
  {% endfor %}
  </tbody>
</table>
{% endmacro %}

{% macro infadic_box(ctx) %}
<table class="box"><tr>
  <td style="width:70%"><span class="lbl">INFORMAÇÕES COMPLEMENTARES</span>
    <div>{{ ctx.msg_fiscal }}</div><div>{{ ctx.msg_contribuinte }}</div></td>
  <td style="width:30%"><span class="lbl">RESERVADO AO FISCO</span></td>
</tr></table>
{% endmacro %}
```

- [ ] **Step 4: Create `danfe_retrato.html`**

Create `py-dfe/py_dfe/danfe/templates/danfe_retrato.html`:

```html
{% import "_danfe_macros.html" as m %}
<style>
  @page {
    size: A4 portrait;
    margin: 5mm;
    @top-center { content: element(danfeHeader); }
    @bottom-right { content: "FOLHA " counter(page) "/" counter(pages); font-size: 7pt; }
  }
  body { font-family: "Times New Roman", serif; font-size: 8pt; color: #000; margin: 0; }
  .danfe-header { position: running(danfeHeader); }
  .b { font-weight: bold; } .center { text-align: center; }
  .small { font-size: 7pt; } .big { font-size: 12pt; } .xbig { font-size: 16pt; }
  .num { text-align: right; white-space: nowrap; }
  .lbl { font-size: 6pt; font-weight: bold; display: block; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  .box, .box td { border: 0.5pt solid #000; }
  .box td { padding: 1pt 2pt; vertical-align: top; word-wrap: break-word; }
  .hdr td { border: 0.5pt solid #000; padding: 2pt; vertical-align: top; }
  .hdr .emit { width: 45%; } .hdr .idbox { width: 25%; } .hdr .keybox { width: 30%; }
  .barcode { width: 100%; height: 12mm; display: block; }
  .items td { font-size: 6pt; }
  .natop, .prot { border: 0.5pt solid #000; padding: 1pt 2pt; }
  .banner { font-weight: bold; text-align: center; border: 1pt solid #000; margin: 2pt 0; padding: 2pt; }
  .pageno::before { content: counter(page) "/" counter(pages); }
  .dupitem { display: inline-block; margin-right: 8pt; }
  .watermark { position: fixed; top: 40%; left: 0; width: 100%; text-align: center;
    font-size: 48pt; color: rgba(0,0,0,0.12); transform: rotate(-30deg); font-weight: bold; }
</style>
{{ m.watermarks(ctx) }}
{{ m.header(ctx) }}
{{ m.dest_box(ctx) }}
{{ m.fatura_box(ctx) }}
{{ m.imposto_box(ctx) }}
{{ m.transp_box(ctx) }}
{{ m.produtos_table(ctx) }}
{{ m.infadic_box(ctx) }}
```

> The context dict is passed under the name `ctx`. Update `generate_danfe_nfe` Step 3 (Task 6) render call to wrap:
> change `render_html(template, context)` to `render_html(template, {"ctx": context})`. **Apply this one-line change now
> ** (it was deferred from Task 6 so the macros have a single `ctx` root). Re-run Task 6 tests after — `build_context`
> is
> unaffected; only the render wrapper changes.

- [ ] **Step 5: Create `danfe_paisagem.html`**

Create `py-dfe/py_dfe/danfe/templates/danfe_paisagem.html` (identical blocks; landscape page + wider header):

```html
{% import "_danfe_macros.html" as m %}
<style>
  @page {
    size: A4 landscape;
    margin: 5mm;
    @top-center { content: element(danfeHeader); }
    @bottom-right { content: "FOLHA " counter(page) "/" counter(pages); font-size: 7pt; }
  }
  body { font-family: "Times New Roman", serif; font-size: 8pt; color: #000; margin: 0; }
  .danfe-header { position: running(danfeHeader); }
  .b { font-weight: bold; } .center { text-align: center; }
  .small { font-size: 7pt; } .big { font-size: 12pt; } .xbig { font-size: 16pt; }
  .num { text-align: right; white-space: nowrap; }
  .lbl { font-size: 6pt; font-weight: bold; display: block; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  .box, .box td { border: 0.5pt solid #000; }
  .box td { padding: 1pt 2pt; vertical-align: top; word-wrap: break-word; }
  .hdr td { border: 0.5pt solid #000; padding: 2pt; vertical-align: top; }
  .hdr .emit { width: 50%; } .hdr .idbox { width: 20%; } .hdr .keybox { width: 30%; }
  .barcode { width: 100%; height: 12mm; display: block; }
  .items td { font-size: 6pt; }
  .natop, .prot { border: 0.5pt solid #000; padding: 1pt 2pt; }
  .banner { font-weight: bold; text-align: center; border: 1pt solid #000; margin: 2pt 0; padding: 2pt; }
  .pageno::before { content: counter(page) "/" counter(pages); }
  .dupitem { display: inline-block; margin-right: 8pt; }
  .watermark { position: fixed; top: 40%; left: 0; width: 100%; text-align: center;
    font-size: 48pt; color: rgba(0,0,0,0.12); transform: rotate(-20deg); font-weight: bold; }
</style>
{{ m.watermarks(ctx) }}
{{ m.header(ctx) }}
{{ m.dest_box(ctx) }}
{{ m.fatura_box(ctx) }}
{{ m.imposto_box(ctx) }}
{{ m.transp_box(ctx) }}
{{ m.produtos_table(ctx) }}
{{ m.infadic_box(ctx) }}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_nfe.py -q`
Expected: PASS (context + A4 PDF tests). If WeasyPrint warns about unknown `running()`/`element()`, that is non-fatal;
the PDF still renders.

- [ ] **Step 7: Stage**

```bash
git add py-dfe/py_dfe/danfe/templates/_danfe_macros.html py-dfe/py_dfe/danfe/templates/danfe_retrato.html py-dfe/py_dfe/danfe/templates/danfe_paisagem.html py-dfe/py_dfe/danfe/nfe55.py py-dfe/tests/unit/test_danfe_nfe.py
# Suggested commit: feat(py-dfe): add DANF-e retrato/paisagem templates and macros
```

---

### Task 8: Simplificado + etiqueta templates (roll, auto-height)

**Files:**

- Create: `py-dfe/py_dfe/danfe/templates/danfe_simplificado.html`
- Create: `py-dfe/py_dfe/danfe/templates/danfe_etiqueta.html`
- Test: `py-dfe/tests/unit/test_danfe_nfe.py` (append)

**Interfaces:**

- Consumes: `generate_danfe_nfe`, context, `_danfe_macros.html`.
- Produces: valid `%PDF` for simplificado/etiqueta with only the §3.11.4 / §3.12.4 mandatory fields.

- [ ] **Step 1: Write the failing test**

Append to `py-dfe/tests/unit/test_danfe_nfe.py`:

```python
def test_simplificado_has_mandatory_subset_only():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": c.LAYOUT_SIMPLIFICADO})
    assert _b64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    html = out["html"][0]
    assert c.TEXT_DANFE_SIMPLIFICADO in html
    assert "EMPRESA TESTE LTDA" in html
    assert "PRODUTO TESTE 1" in html            # itens included in simplificado
    assert "RESERVADO AO FISCO" not in html     # quadro omitted
    assert "TRANSPORTADOR" not in html          # quadro omitted


def test_etiqueta_omits_items():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": c.LAYOUT_ETIQUETA})
    assert _b64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    html = out["html"][0]
    assert c.TEXT_DANFE_ETIQUETA in html
    assert "PRODUTO TESTE 1" not in html        # etiqueta has no item list
    assert "CLIENTE TESTE LTDA" in html
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_nfe.py -k "simplificado or etiqueta" -q`
Expected: FAIL (`TemplateNotFound: danfe_simplificado.html`).

- [ ] **Step 3: Create `danfe_simplificado.html`**

Create `py-dfe/py_dfe/danfe/templates/danfe_simplificado.html` (§3.11.4 obrigatórios: emit Nome/UF/CNPJ/IE; ide
tpNF/série/nº/dhEmi; dest Nome/UF/CNPJ-CPF; itens descrição/un/qtd/vUnit/vTotal; total vNF; "DANFE Simplificado";
chave + barcode; protocolo):

```html
{% import "_danfe_macros.html" as m %}
<style>
  @page { size: 80mm 2000mm; margin: 2mm; }
  body { font-family: "Times New Roman", serif; font-size: 7pt; margin: 0; }
  .receipt { width: 100%; box-sizing: border-box; position: relative; overflow: hidden; }
  .center { text-align: center; } .b { font-weight: bold; }
  hr { border: none; border-top: 1px dashed #000; margin: 2px 0; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  .num { text-align: right; white-space: nowrap; }
  .barcode { width: 100%; height: 12mm; display: block; margin: 2px 0; }
  .watermark { position: absolute; top: 35%; left: 0; width: 100%; text-align: center;
    font-size: 14pt; white-space: nowrap; color: rgba(0,0,0,0.18);
    transform: rotate(-25deg); font-weight: bold; }
</style>
<div class="receipt">
  {{ m.watermarks(ctx) }}
  <div class="center b">{{ ctx.text.simplificado }}</div>
  <hr>
  <div class="b">{{ ctx.emit.nome }}</div>
  <div>CNPJ: {{ ctx.emit.cnpj }} - IE: {{ ctx.emit.ie }} - UF: {{ ctx.emit.uf }}</div>
  <hr>
  <div>{{ ctx.ide.tpNF_label }} - Nº {{ ctx.ide.nNF }} SÉRIE {{ ctx.ide.serie }}</div>
  <div>EMISSÃO: {{ ctx.ide.dhEmi }}</div>
  <hr>
  <div class="b">DESTINATÁRIO</div>
  <div>{{ ctx.dest.nome }}</div>
  <div>CNPJ/CPF: {{ ctx.dest.doc }} - UF: {{ ctx.dest.uf }}</div>
  <hr>
  <table>
    <colgroup><col style="width:46%"><col style="width:10%"><col style="width:12%"><col style="width:16%"><col style="width:16%"></colgroup>
    <thead><tr><td>DESCR</td><td>UN</td><td class="num">QTD</td><td class="num">V.UN</td><td class="num">V.TOT</td></tr></thead>
    <tbody>
    {% for it in ctx.items %}
    <tr><td>{{ it.xProd }}</td><td>{{ it.uCom }}</td><td class="num">{{ it.qCom }}</td>
        <td class="num">{{ it.vUnCom }}</td><td class="num">{{ it.vProd }}</td></tr>
    {% endfor %}
    </tbody>
  </table>
  <hr>
  <div class="b">VALOR TOTAL R$ {{ ctx.totals.vNF }}</div>
  <hr>
  <img class="barcode" src="{{ ctx.chave_barcode }}" alt="chave">
  <div class="center b">{{ ctx.chave_fmt }}</div>
  {% if ctx.show_protocolo and ctx.protocolo %}
  <div class="center">{{ ctx.protocolo_label }}: {{ ctx.protocolo.nProt }} {{ ctx.protocolo.dhRecbto }}</div>
  {% endif %}
  {% if ctx.is_contingencia %}<hr><div class="center b">{{ ctx.text.contingencia }}</div>{% endif %}
</div>
```

- [ ] **Step 4: Create `danfe_etiqueta.html`**

Create `py-dfe/py_dfe/danfe/templates/danfe_etiqueta.html` (§3.12.4 obrigatórios: "DANFE Simplificado - Etiqueta"; emit
Nome/UF/CNPJ/IE; ide tpNF/série/nº/dhEmi; dest Nome/UF/CNPJ-CPF/IE; total vNF; chave + barcode; protocolo/EPEC — no item
list):

```html
{% import "_danfe_macros.html" as m %}
<style>
  @page { size: 80mm 1200mm; margin: 2mm; }
  body { font-family: "Times New Roman", serif; font-size: 7pt; margin: 0; }
  .receipt { width: 100%; box-sizing: border-box; position: relative; overflow: hidden; }
  .center { text-align: center; } .b { font-weight: bold; }
  hr { border: none; border-top: 1px dashed #000; margin: 2px 0; }
  .barcode { width: 100%; height: 14mm; display: block; margin: 2px 0; }
  .watermark { position: absolute; top: 35%; left: 0; width: 100%; text-align: center;
    font-size: 14pt; white-space: nowrap; color: rgba(0,0,0,0.18);
    transform: rotate(-25deg); font-weight: bold; }
</style>
<div class="receipt">
  {{ m.watermarks(ctx) }}
  <div class="center b">{{ ctx.text.etiqueta }}</div>
  <hr>
  <div class="b">{{ ctx.emit.nome }}</div>
  <div>CNPJ: {{ ctx.emit.cnpj }} - IE: {{ ctx.emit.ie }} - UF: {{ ctx.emit.uf }}</div>
  <hr>
  <div>{{ ctx.ide.tpNF_label }} - Nº {{ ctx.ide.nNF }} SÉRIE {{ ctx.ide.serie }} - {{ ctx.ide.dhEmi }}</div>
  <hr>
  <div class="b">DESTINATÁRIO</div>
  <div>{{ ctx.dest.nome }}</div>
  <div>CNPJ/CPF: {{ ctx.dest.doc }} - IE: {{ ctx.dest.ie }} - UF: {{ ctx.dest.uf }}</div>
  <hr>
  <div class="b">VALOR TOTAL R$ {{ ctx.totals.vNF }}</div>
  <hr>
  <img class="barcode" src="{{ ctx.chave_barcode }}" alt="chave">
  <div class="center b">{{ ctx.chave_fmt }}</div>
  {% if ctx.show_protocolo and ctx.protocolo %}
  <div class="center">{{ ctx.protocolo_label }}: {{ ctx.protocolo.nProt }} {{ ctx.protocolo.dhRecbto }}</div>
  {% endif %}
</div>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_nfe.py -q`
Expected: PASS (all variants).

- [ ] **Step 6: Stage**

```bash
git add py-dfe/py_dfe/danfe/templates/danfe_simplificado.html py-dfe/py_dfe/danfe/templates/danfe_etiqueta.html py-dfe/tests/unit/test_danfe_nfe.py
# Suggested commit: feat(py-dfe): add DANF-e simplificado/etiqueta roll templates
```

---

### Task 9: Dispatcher + service wiring

**Files:**

- Create: `py-dfe/py_dfe/danfe/document.py`
- Modify: `py-dfe/py_dfe/services/_nf.py:8` (import) and `:66-67` (route)
- Test: `py-dfe/tests/unit/test_danfe_document.py`
- Test: `py-dfe/tests/unit/test_danfce_routing.py` (verify still green)

**Interfaces:**

- Consumes: `generate_danfce` (existing), `generate_danfe_nfe` (Task 6), `parse_xml_bytes`, constants.
- Produces: `generate_danfe(payload: dict) -> dict` (dispatch by `ide/mod`).

- [ ] **Step 1: Write the failing test**

Create `py-dfe/tests/unit/test_danfe_document.py`:

```python
import base64

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.document import generate_danfe
from py_dfe.exceptions import DFeError
from tests.danfe_fixtures import sample_nfe_proc, sample_nfe55_proc


def test_dispatch_mod65_uses_nfce():
    pytest.importorskip("weasyprint")
    out = generate_danfe({"xml": sample_nfe_proc()})
    assert base64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    # NFC-e auxiliary-document title is present.
    assert "Consumidor" in out["html"][0] or "NFC-e" in out["html"][0]


def test_dispatch_mod55_uses_nfe():
    pytest.importorskip("weasyprint")
    out = generate_danfe({"xml": sample_nfe55_proc(), "layout": c.LAYOUT_RETRATO})
    assert base64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    assert c.TEXT_DANFE_DESC in out["html"][0]


def test_dispatch_unsupported_model_raises():
    bad = sample_nfe55_proc().replace("<mod>55</mod>", "<mod>57</mod>")
    with pytest.raises(DFeError) as exc:
        generate_danfe({"xml": bad})
    assert exc.value.status_code == 422


def test_dispatch_missing_xml_raises():
    with pytest.raises(DFeError) as exc:
        generate_danfe({})
    assert exc.value.status_code == 422
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_document.py -q`
Expected: FAIL (`ModuleNotFoundError: ... document`).

- [ ] **Step 3: Implement `document.py`**

Create `py-dfe/py_dfe/danfe/document.py`:

```python
"""GerarDanfe dispatcher — routes by NF-e model (mod 55 / 65)."""

from __future__ import annotations

from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe.danfce import generate_danfce
from py_dfe.danfe.nfe55 import generate_danfe_nfe
from py_dfe.exceptions import DANFE_INVALID_XML, DANFE_UNSUPPORTED_MODEL, DFeError
from py_dfe.xmlops.builder import parse_xml_bytes


def generate_danfe(payload: dict[str, Any]) -> dict[str, Any]:
    """Render the correct auxiliary document for the XML's model code."""
    xml = payload.get("xml")
    if not xml:
        raise DFeError(422, DANFE_INVALID_XML, "Missing 'xml' in body")
    mod = _peek_mod(xml)
    if mod == c.MODELO_NFCE:
        return generate_danfce(payload)
    if mod == c.MODELO_NFE:
        return generate_danfe_nfe(payload)
    raise DFeError(
        422, DANFE_UNSUPPORTED_MODEL,
        f"DANFE supports models 55 and 65, got {mod!r}",
    )


def _peek_mod(xml: str) -> str | None:
    try:
        parsed = parse_xml_bytes(xml.encode("utf-8") if isinstance(xml, str) else xml)
    except etree.XMLSyntaxError as exc:
        raise DFeError(422, DANFE_INVALID_XML, f"Malformed XML: {exc}") from exc
    root = next(iter(parsed.values()))
    nfe = root.get("NFe", root) if isinstance(root, dict) else {}
    inf_nfe = nfe.get("infNFe", {}) if isinstance(nfe, dict) else {}
    if not isinstance(inf_nfe, dict):
        raise DFeError(422, DANFE_INVALID_XML, "infNFe not found")
    return (inf_nfe.get("ide", {}) or {}).get("mod")
```

- [ ] **Step 4: Wire the service**

In `py-dfe/py_dfe/services/_nf.py`, change the import on line 8:

```python
from py_dfe.danfe.document import generate_danfe
```

and the route inside `call` (lines 66-67):

```python
        if service == SERVICE_GERAR_DANFE:
            return generate_danfe(payload)
```

- [ ] **Step 5: Run tests to verify they pass**

Run:
`cd py-dfe && python -m pytest tests/unit/test_danfe_document.py tests/unit/test_danfce_routing.py tests/unit/test_danfce.py -q`
Expected: PASS (dispatcher + existing NFC-e routing/behaviour intact).

- [ ] **Step 6: Stage**

```bash
git add py-dfe/py_dfe/danfe/document.py py-dfe/py_dfe/services/_nf.py py-dfe/tests/unit/test_danfe_document.py
# Suggested commit: feat(py-dfe): dispatch GerarDanfe by NF-e model (55/65)
```

---

### Task 10: Integration test (end-to-end, all variants + contingency)

**Files:**

- Create: `py-dfe/tests/integration/test_danfe_nfe_generation.py`

**Interfaces:**

- Consumes: `generate_danfe` dispatcher, `sample_nfe55_proc`.

- [ ] **Step 1: Write the test**

Create `py-dfe/tests/integration/test_danfe_nfe_generation.py`:

```python
import base64
import io

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.document import generate_danfe
from tests.danfe_fixtures import sample_nfe55_proc

pytestmark = pytest.mark.usefixtures()


def _pdf(out):
    raw = base64.b64decode(out["pdf_b64"])
    assert raw[:4] == b"%PDF"
    return raw


@pytest.mark.parametrize("layout", [
    c.LAYOUT_RETRATO, c.LAYOUT_PAISAGEM, c.LAYOUT_SIMPLIFICADO, c.LAYOUT_ETIQUETA,
])
def test_each_variant_end_to_end(layout):
    pytest.importorskip("weasyprint")
    out = generate_danfe({"xml": sample_nfe55_proc(n_items=2), "layout": layout})
    _pdf(out)


@pytest.mark.parametrize("tp_emis", [
    c.TP_EMIS_NORMAL, c.TP_EMIS_SVC_AN, c.TP_EMIS_FS,
    c.TP_EMIS_FSDA, c.TP_EMIS_EPEC,
])
def test_each_contingency_mode_end_to_end(tp_emis):
    pytest.importorskip("weasyprint")
    out = generate_danfe({"xml": sample_nfe55_proc(tp_emis=tp_emis)})
    _pdf(out)


def test_retrato_paginates_with_many_items():
    pytest.importorskip("weasyprint")
    from pypdf import PdfReader
    out = generate_danfe({"xml": sample_nfe55_proc(n_items=80), "layout": c.LAYOUT_RETRATO})
    reader = PdfReader(io.BytesIO(_pdf(out)))
    assert len(reader.pages) >= 2
```

- [ ] **Step 2: Run the test**

Run: `cd py-dfe && python -m pytest tests/integration/test_danfe_nfe_generation.py -q`
Expected: PASS. If `test_retrato_paginates_with_many_items` yields 1 page, increase `n_items` until overflow (the A4
body must exceed one page) — 80 items is comfortably beyond one A4.

- [ ] **Step 3: Run the full DANFE suite**

Run:
`cd py-dfe && python -m pytest tests/unit/test_danfe_constants.py tests/unit/test_danfe_formatters.py tests/unit/test_danfe_barcode.py tests/unit/test_danfe_render.py tests/unit/test_danfe_fixtures.py tests/unit/test_danfe_nfe.py tests/unit/test_danfe_document.py tests/unit/test_danfce.py tests/unit/test_danfce_routing.py tests/integration/test_danfe_nfe_generation.py tests/integration/test_danfce_generation.py -q`
Expected: PASS (DANF-e + DANFC-e, no regressions).

- [ ] **Step 4: Stage**

```bash
git add py-dfe/tests/integration/test_danfe_nfe_generation.py
# Suggested commit: test(py-dfe): end-to-end DANF-e variants and contingency modes
```

---

### Task 11: Documentation

**Files:**

- Modify: `DOCS.md` (§3 GerarDanfe section)
- Modify: `CONDUCT.md` (DANFE rendering section)

**Interfaces:** none (docs only).

- [ ] **Step 1: Update `DOCS.md` §3**

In the existing GerarDanfe subsection of `DOCS.md §3`, replace the DANFC-e-only description with the model-dispatched
version. Add:

```markdown
#### GerarDanfe (auxiliary documents)

`GerarDanfe` renders the auxiliary fiscal document from an authorized XML —
no certificate, no SEFAZ call. `danfe/document.py::generate_danfe` dispatches
by `ide/mod`:

- **mod 65 → DANFC-e** (`danfe/danfce.py`): thermal receipt, QR Code (segno),
  variants completo/resumido, contingência offline (2 vias).
- **mod 55 → DANF-e** (`danfe/nfe55.py`): NF-e DANFE, CODE-128 barcode of the
  44-digit chave (`danfe/barcode.py`, python-barcode). Variants via `layout`:
  `retrato` (default), `paisagem` (fixed A4, multi-page), `simplificado`,
  `etiqueta` (roll ≥55mm, auto-height). Contingency by `tpEmis`: normal/SVC
  (chave barcode + protocolo), FS/FS-DA (second "Dados da NF-e" barcode,
  protocolo suppressed), EPEC (protocolo do EPEC).

Payload: `{"xml": <str>, "layout"?: <str>, "canceled"?: <bool>}`.
Returns `{"pdf_b64": <base64>, "html": [<str>, ...]}`.
Modules: `formatters.py` (BR locale), `barcode.py` (CODE-128 + FS/FS-DA code),
`qr.py` (NFC-e QR), `render.py` (HTML→PDF, `fit_height` flag).
Dependencies: weasyprint, jinja2, segno, python-barcode.
```

- [ ] **Step 2: Update `CONDUCT.md`**

In the existing "DANFC-e rendering" section of `CONDUCT.md`, broaden the heading to "DANFE rendering" and add:

```markdown
- DANF-e (mod 55) uses CODE-128 barcodes via **python-barcode** (pure-Python,
  SVG, no native binary) — distinct from NFC-e's QR (segno).
- Two sizing modes in `render.py::htmls_to_pdf(fit_height=...)`: roll/auto-height
  (NFC-e, DANFE simplificado/etiqueta) vs fixed A4 multi-page (retrato/paisagem).
- Multi-page DANFE repeats its header via WeasyPrint running elements
  (`position: running()` + `@top-center { content: element(...) }`) and numbers
  folhas with CSS `counter(page)/counter(pages)`.
- `GerarDanfe` is model-dispatched (`danfe/document.py`): never branch on model
  inside a renderer; add the branch in the dispatcher.
```

- [ ] **Step 3: Stage**

```bash
git add DOCS.md CONDUCT.md
# Suggested commit: docs: document model-dispatched GerarDanfe (DANF-e + DANFC-e)
```

---

## Self-Review

**Spec coverage:**

| Spec section                                 | Task                                           |
|----------------------------------------------|------------------------------------------------|
| §2.1 Dispatch by mod                         | Task 9                                         |
| §2.2 Module layout                           | Tasks 3,6,7,8,9                                |
| §2.3 Render `fit_height`                     | Task 4                                         |
| §2.4 Multi-page / running header             | Task 7 (templates) + Task 10 (pagination test) |
| §3 Constants                                 | Task 1                                         |
| §4 Barcode (CODE-128 + FS/FS-DA mod-11 DV)   | Task 3                                         |
| §5 Context builder (all tags + flags)        | Task 6                                         |
| §6 Templates (macros + 4 variants)           | Tasks 7,8                                      |
| §7 Errors (DANFE_INVALID_BARCODE)            | Tasks 1,3                                      |
| §8 python-barcode dep                        | Task 3                                         |
| §9 Tests (unit + integration + fixtures)     | Tasks 1,2,3,4,5,6,7,8,9,10                     |
| §10 Docs                                     | Task 11                                        |
| §11 Cross-project (no worker/api/cdk change) | Task 9 (contract preserved)                    |

**Type consistency:** `generate_danfe` (dispatcher, Task 9) ≠ `generate_danfe_nfe` (mod-55 generator, Task 6) ≠
`generate_danfce` (existing NFC-e). `htmls_to_pdf(pages, *, fit_height=True)` signature is identical in Tasks 4, 6.
`code128c_data_uri` / `dados_nfe_code` / `_mod11_dv` signatures match between Task 3 (def) and Task 6 (use). Context
root name `ctx` is consistent across Task 6 render wrapper and Tasks 7-8 templates (the
`render_html(template, {"ctx": context})` wrap is applied in Task 7 Step 4).

**Placeholder scan:** no TBD/TODO; every code step has complete code; commit messages are explicit; the one deferred
edit (render wrapper `{"ctx": context}`) is called out in Task 7 Step 4 with the exact change.

**Note on dependencies:** `pypdf` is used by render/integration pagination tests (Tasks 4, 10). If absent, add `pypdf`
to the test/dev dependencies in `pyproject.toml` (pure-Python). Mentioned in Task 4 Step 1.
