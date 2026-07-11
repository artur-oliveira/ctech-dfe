# DANFC-e Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an in-house `GerarDanfe` service to `py_dfe` that renders a DANFC-e (DANFE NFC-e) PDF + HTML from an authorized NFC-e XML, with no certificate and no SEFAZ call.

**Architecture:** New generic-first subpackage `py_dfe/danfe/`: shared `render.py` (Jinja2→WeasyPrint), `qr.py` (segno→data-URI), `formatters.py` (BR formatting); DANFC-e-specific `danfce.py` + `templates/danfce.html`. Routed through the existing `_NFServiceClient.call` via a new `GerarDanfe` service key that branches before any SEFAZ work. `LambdaRequest` certificate fields become optional.

**Tech Stack:** Python 3.14, WeasyPrint, Jinja2, segno, lxml (existing), Pydantic v2 (existing), pytest.

## Global Constraints

- Python `>=3.14`; Lambda `provided` runtime via CDK layer.
- All errors MUST be raised as `DFeError(status_code, error_code, message)` — never raw `Exception`/`ValueError`.
- No magic strings/numbers — every key/code/text constant lives in `constants/` (new `constants/danfe.py`).
- DRY: reuse existing `parse_xml_bytes` (`xmlops/builder.py`), `DOC_TYPE_CODE`, `mask`/parse utilities; do not duplicate.
- DANFC-e contains ONLY data from the NFC-e XML (manual mandate).
- **Do NOT `git commit`.** Stage changes only; the user commits manually (project policy).
- Frontend/UI untouched. Cross-project note only: cdk Lambda layer must bundle WeasyPrint native libs (cairo/pango/gdk-pixbuf/glib/gobject + fonts) — flagged, not implemented here.

---

### Task 1: Constants + dependencies

**Files:**
- Create: `py-dfe/py_dfe/constants/danfe.py`
- Modify: `py-dfe/py_dfe/exceptions.py` (add error-code constants)
- Modify: `py-dfe/pyproject.toml` (add runtime deps)
- Modify: `py-dfe/layer/requirements.txt` (add pinned deps)
- Test: `py-dfe/tests/unit/test_danfe_constants.py`

**Interfaces:**
- Produces:
  - `constants/danfe.py`: `SERVICE_GERAR_DANFE = "GerarDanfe"`, `LAYOUT_COMPLETO = "completo"`, `LAYOUT_RESUMIDO = "resumido"`, `VALID_LAYOUTS = frozenset({...})`, `TP_EMIS_NORMAL = "1"`, `TP_EMIS_CONTINGENCIA_OFFLINE = "9"`, `TP_AMB_PRODUCAO = "1"`, `TP_AMB_HOMOLOGACAO = "2"`, `MODELO_NFCE = "65"`, text constants (`TEXT_DOC_AUXILIAR`, `TEXT_CONTINGENCIA_L1`, `TEXT_CONTINGENCIA_L2`, `TEXT_HOMOLOGACAO`, `TEXT_CONSUMIDOR_NAO_IDENTIFICADO`, `VIA_CONSUMIDOR`, `VIA_ESTABELECIMENTO`, `TEXT_WATERMARK_CANCELADA`), `TPAG_LABELS: dict[str, str]`.
  - `exceptions.py`: `DANFE_INVALID_XML`, `DANFE_UNSUPPORTED_MODEL`, `DANFE_MISSING_QRCODE`, `DANFE_RENDER_FAILED`, `CERT_REQUIRED` (str code constants).

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfe_constants.py`:
```python
from py_dfe.constants import danfe as c


def test_service_and_layout_constants():
    assert c.SERVICE_GERAR_DANFE == "GerarDanfe"
    assert c.LAYOUT_COMPLETO == "completo"
    assert c.LAYOUT_RESUMIDO == "resumido"
    assert c.VALID_LAYOUTS == frozenset({"completo", "resumido"})


def test_emission_and_env_constants():
    assert c.TP_EMIS_NORMAL == "1"
    assert c.TP_EMIS_CONTINGENCIA_OFFLINE == "9"
    assert c.TP_AMB_PRODUCAO == "1"
    assert c.TP_AMB_HOMOLOGACAO == "2"
    assert c.MODELO_NFCE == "65"


def test_payment_labels():
    assert c.TPAG_LABELS["01"] == "Dinheiro"
    assert c.TPAG_LABELS["03"] == "Cartão de Crédito"
    assert c.TPAG_LABELS["17"] == "Pagamento Instantâneo (PIX) - Dinâmico"
    assert c.TPAG_LABELS["99"] == "Outros"


def test_error_codes_exist():
    from py_dfe import exceptions as e
    assert e.DANFE_INVALID_XML
    assert e.DANFE_UNSUPPORTED_MODEL
    assert e.DANFE_MISSING_QRCODE
    assert e.DANFE_RENDER_FAILED
    assert e.CERT_REQUIRED
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_constants.py -v`
Expected: FAIL — `ModuleNotFoundError: py_dfe.constants.danfe`.

- [ ] **Step 3: Create `constants/danfe.py`**

```python
"""Constants for auxiliary fiscal document (DANFE) generation."""

# Render service key (routed in _NFServiceClient.call, no SEFAZ call).
SERVICE_GERAR_DANFE = "GerarDanfe"

# DANFC-e layout variants.
LAYOUT_COMPLETO = "completo"
LAYOUT_RESUMIDO = "resumido"
VALID_LAYOUTS = frozenset({LAYOUT_COMPLETO, LAYOUT_RESUMIDO})

# ide/tpEmis (NFC-e). Only "9" (offline contingency) triggers the 2-vias layout.
TP_EMIS_NORMAL = "1"
TP_EMIS_CONTINGENCIA_OFFLINE = "9"

# ide/tpAmb.
TP_AMB_PRODUCAO = "1"
TP_AMB_HOMOLOGACAO = "2"

# Document model code for NFC-e.
MODELO_NFCE = "65"

# Fixed copy (manual_danfce.md).
TEXT_DOC_AUXILIAR = "Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica"
TEXT_CONTINGENCIA_L1 = "EMITIDA EM CONTINGÊNCIA"
TEXT_CONTINGENCIA_L2 = "Pendente de autorização"
TEXT_HOMOLOGACAO = "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL"
TEXT_CONSUMIDOR_NAO_IDENTIFICADO = "CONSUMIDOR NÃO IDENTIFICADO"
VIA_CONSUMIDOR = "Via Consumidor"
VIA_ESTABELECIMENTO = "Via do Estabelecimento"
TEXT_WATERMARK_CANCELADA = "CANCELADA"

# tPag → label (SEFAZ payment-type table, manual §3.1.3).
TPAG_LABELS = {
    "01": "Dinheiro",
    "02": "Cheque",
    "03": "Cartão de Crédito",
    "04": "Cartão de Débito",
    "05": "Crédito Loja",
    "10": "Vale Alimentação",
    "11": "Vale Refeição",
    "12": "Vale Presente",
    "13": "Vale Combustível",
    "15": "Boleto Bancário",
    "16": "Depósito Bancário",
    "17": "Pagamento Instantâneo (PIX) - Dinâmico",
    "18": "Transferência bancária, Carteira Digital",
    "19": "Programa de fidelidade, Cashback, Crédito Virtual",
    "20": "Pagamento Instantâneo (PIX) - Estático",
    "21": "Crédito em loja",
    "22": "Pagamento Eletrônico não Informado",
    "90": "Sem pagamento",
    "99": "Outros",
}
```

- [ ] **Step 4: Add error-code constants to `exceptions.py`**

Add after the existing code-constant block (after line `UNEXPECTED_ERROR_CODE = 'unexpected error'`):
```python
DANFE_INVALID_XML = 'danfe invalid xml'
DANFE_UNSUPPORTED_MODEL = 'danfe unsupported model'
DANFE_MISSING_QRCODE = 'danfe missing qrcode'
DANFE_RENDER_FAILED = 'danfe render failed'
CERT_REQUIRED = 'certificate required'
```

- [ ] **Step 5: Add dependencies**

`pyproject.toml` — extend `[project].dependencies`:
```toml
dependencies = [
    "httpx>=0.28.1",
    "lxml>=6.1.0",
    "cryptography>=48.0.0",
    "signxml>=4.4.0",
    "pydantic>=2.13.4",
    "weasyprint>=66.0",
    "jinja2>=3.1.4",
    "segno>=1.6.1",
]
```

`layer/requirements.txt` — append pinned lines:
```
jinja2==3.1.4
segno==1.6.1
weasyprint==66.0
```

- [ ] **Step 6: Install deps and run test, verify it passes**

Run: `cd py-dfe && pip install -e . && python -m pytest tests/unit/test_danfe_constants.py -v`
Expected: PASS (4 tests).

- [ ] **Step 7: Stage (do not commit)**

```bash
git add py-dfe/py_dfe/constants/danfe.py py-dfe/py_dfe/exceptions.py py-dfe/pyproject.toml py-dfe/layer/requirements.txt py-dfe/tests/unit/test_danfe_constants.py
```

---

### Task 2: `formatters.py` — BR value/date/document formatting

**Files:**
- Create: `py-dfe/py_dfe/danfe/__init__.py` (empty)
- Create: `py-dfe/py_dfe/danfe/formatters.py`
- Test: `py-dfe/tests/unit/test_danfe_formatters.py`

**Interfaces:**
- Produces (`py_dfe.danfe.formatters`):
  - `money_br(value: str | float | int | None) -> str` — `"1.234,56"`; `None`/`""` → `"0,00"`.
  - `dt_local(iso: str | None) -> str` — ISO-8601 (with offset) → `"dd/mm/yyyy HH:MM:SS"`, respecting the embedded offset (no tz conversion). `None`/`""` → `""`.
  - `mask_cnpj(digits: str) -> str` — 14 digits → `"99.999.999/9999-99"`.
  - `mask_cpf(digits: str) -> str` — 11 digits → `"999.999.999-99"`.
  - `chave_blocks(key: str) -> str` — 44 digits → 11 space-separated 4-digit blocks.

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfe_formatters.py`:
```python
from py_dfe.danfe import formatters as f


def test_money_br():
    assert f.money_br("1234.5") == "1.234,50"
    assert f.money_br("0") == "0,00"
    assert f.money_br(60.9) == "60,90"
    assert f.money_br(None) == "0,00"
    assert f.money_br("") == "0,00"
    assert f.money_br("1000000") == "1.000.000,00"


def test_dt_local():
    assert f.dt_local("2026-06-25T10:30:00-03:00") == "25/06/2026 10:30:00"
    assert f.dt_local("") == ""
    assert f.dt_local(None) == ""


def test_mask_cnpj():
    assert f.mask_cnpj("12345678000199") == "12.345.678/0001-99"


def test_mask_cpf():
    assert f.mask_cpf("12345678909") == "123.456.789-09"


def test_chave_blocks():
    key = "28170800156225000131650110000151341562040824"[:44]
    out = f.chave_blocks(key)
    assert out.count(" ") == 10
    assert all(len(b) == 4 for b in out.split(" "))
    assert out.replace(" ", "") == key
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_formatters.py -v`
Expected: FAIL — `ModuleNotFoundError: py_dfe.danfe`.

- [ ] **Step 3: Implement**

Create empty `py-dfe/py_dfe/danfe/__init__.py`.

`py-dfe/py_dfe/danfe/formatters.py`:
```python
"""Pure Brazilian-locale formatting helpers for DANFE rendering."""

from __future__ import annotations

import datetime
from decimal import Decimal, InvalidOperation


def money_br(value: str | float | int | None) -> str:
    """Format a monetary value as '1.234,56' (dot thousands, comma decimal)."""
    if value is None or value == "":
        value = 0
    try:
        dec = Decimal(str(value))
    except (InvalidOperation, ValueError):
        dec = Decimal(0)
    # Two decimals, US grouping, then swap separators.
    formatted = f"{dec:,.2f}"
    return formatted.replace(",", "_").replace(".", ",").replace("_", ".")


def dt_local(iso: str | None) -> str:
    """ISO-8601 (with offset) → 'dd/mm/yyyy HH:MM:SS', keeping the wall clock."""
    if not iso:
        return ""
    dt = datetime.datetime.fromisoformat(iso)
    return dt.strftime("%d/%m/%Y %H:%M:%S")


def mask_cnpj(digits: str) -> str:
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) != 14:
        return digits or ""
    return f"{d[0:2]}.{d[2:5]}.{d[5:8]}/{d[8:12]}-{d[12:14]}"


def mask_cpf(digits: str) -> str:
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) != 11:
        return digits or ""
    return f"{d[0:3]}.{d[3:6]}.{d[6:9]}-{d[9:11]}"


def chave_blocks(key: str) -> str:
    d = "".join(filter(str.isdigit, key or ""))
    return " ".join(d[i:i + 4] for i in range(0, len(d), 4))
```

- [ ] **Step 4: Run test, verify it passes**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_formatters.py -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Stage (do not commit)**

```bash
git add py-dfe/py_dfe/danfe/__init__.py py-dfe/py_dfe/danfe/formatters.py py-dfe/tests/unit/test_danfe_formatters.py
```

---

### Task 3: `qr.py` — QR Code data-URI

**Files:**
- Create: `py-dfe/py_dfe/danfe/qr.py`
- Test: `py-dfe/tests/unit/test_danfe_qr.py`

**Interfaces:**
- Consumes: nothing from prior tasks.
- Produces (`py_dfe.danfe.qr`): `qr_data_uri(payload: str) -> str` — returns `"data:image/png;base64,<...>"`; QR error level **M**, UTF-8 (manual §4.5). Raises `DFeError(422, DANFE_MISSING_QRCODE, ...)` on empty payload.

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfe_qr.py`:
```python
import base64

import pytest

from py_dfe.danfe.qr import qr_data_uri
from py_dfe.exceptions import DFeError


def test_qr_data_uri_shape():
    uri = qr_data_uri("https://example.gov.br/nfce/qrcode?p=abc|2|1|1|HASH")
    assert uri.startswith("data:image/png;base64,")
    raw = base64.b64decode(uri.split(",", 1)[1])
    assert raw[:8] == b"\x89PNG\r\n\x1a\n"  # PNG magic


def test_qr_empty_raises():
    with pytest.raises(DFeError) as exc:
        qr_data_uri("")
    assert exc.value.status_code == 422
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_qr.py -v`
Expected: FAIL — `ModuleNotFoundError`.

- [ ] **Step 3: Implement**

`py-dfe/py_dfe/danfe/qr.py`:
```python
"""QR Code image generation for fiscal auxiliary documents."""

from __future__ import annotations

import base64
import io

import segno

from py_dfe.exceptions import DANFE_MISSING_QRCODE, DFeError

# Manual §4.5: error correction level M, UTF-8.
_QR_ERROR = "m"


def qr_data_uri(payload: str) -> str:
    """Render *payload* as a PNG QR Code embedded as a base64 data-URI."""
    if not payload:
        raise DFeError(422, DANFE_MISSING_QRCODE, "QR Code payload is empty")
    qr = segno.make(payload, error=_QR_ERROR, encoding="utf-8")
    buf = io.BytesIO()
    qr.save(buf, kind="png", scale=4, border=2)
    b64 = base64.b64encode(buf.getvalue()).decode("ascii")
    return f"data:image/png;base64,{b64}"
```

- [ ] **Step 4: Run test, verify it passes**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_qr.py -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Stage (do not commit)**

```bash
git add py-dfe/py_dfe/danfe/qr.py py-dfe/tests/unit/test_danfe_qr.py
```

---

### Task 4: `render.py` — generic Jinja2 → WeasyPrint

**Files:**
- Create: `py-dfe/py_dfe/danfe/render.py`
- Create: `py-dfe/py_dfe/danfe/templates/_probe.html` (tiny template used only by this task's test)
- Test: `py-dfe/tests/unit/test_danfe_render.py`

**Interfaces:**
- Consumes: nothing from prior tasks.
- Produces (`py_dfe.danfe.render`):
  - `render_html(template_name: str, context: dict) -> str` — render a Jinja2 template from `py_dfe/danfe/templates/`.
  - `html_to_pdf(html: str) -> bytes` — WeasyPrint conversion; raises `DFeError(500, DANFE_RENDER_FAILED, ...)` on failure.

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfe_render.py`:
```python
import pytest

from py_dfe.danfe.render import render_html, html_to_pdf


def test_render_html_substitutes_context():
    out = render_html("_probe.html", {"name": "Mundo"})
    assert "Olá Mundo" in out


def test_html_to_pdf_returns_pdf_bytes():
    weasyprint = pytest.importorskip("weasyprint")  # needs native libs
    pdf = html_to_pdf("<html><body><p>x</p></body></html>")
    assert pdf[:4] == b"%PDF"
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_render.py -v`
Expected: FAIL — `ModuleNotFoundError: py_dfe.danfe.render`.

- [ ] **Step 3: Implement**

Create `py-dfe/py_dfe/danfe/templates/_probe.html`:
```html
<p>Olá {{ name }}</p>
```

`py-dfe/py_dfe/danfe/render.py`:
```python
"""Generic HTML(Jinja2) → PDF(WeasyPrint) rendering. Document-agnostic."""

from __future__ import annotations

import pathlib

from jinja2 import Environment, FileSystemLoader, select_autoescape

from py_dfe.exceptions import DANFE_RENDER_FAILED, DFeError

_TEMPLATE_DIR = pathlib.Path(__file__).parent / "templates"

_env = Environment(
    loader=FileSystemLoader(str(_TEMPLATE_DIR)),
    autoescape=select_autoescape(["html", "xml"]),
)


def render_html(template_name: str, context: dict) -> str:
    """Render a Jinja2 template (from the danfe/templates/ dir) to an HTML string."""
    template = _env.get_template(template_name)
    return template.render(**context)


def html_to_pdf(html: str) -> bytes:
    """Convert an HTML string to PDF bytes via WeasyPrint."""
    try:
        from weasyprint import HTML  # imported lazily (heavy native deps)

        return HTML(string=html, base_url=str(_TEMPLATE_DIR)).write_pdf()
    except DFeError:
        raise
    except Exception as exc:  # noqa: BLE001 - wrap everything as DFeError
        raise DFeError(500, DANFE_RENDER_FAILED, f"PDF render failed: {exc}") from exc
```

- [ ] **Step 4: Run test, verify it passes**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_render.py -v`
Expected: PASS (`test_render_html_substitutes_context` PASS; `test_html_to_pdf_returns_pdf_bytes` PASS if WeasyPrint native libs present, else SKIPPED).

- [ ] **Step 5: Stage (do not commit)**

```bash
git add py-dfe/py_dfe/danfe/render.py py-dfe/py_dfe/danfe/templates/_probe.html py-dfe/tests/unit/test_danfe_render.py
```

---

### Task 5: Shared test fixture — sample authorized NFC-e XML

**Files:**
- Create: `py-dfe/tests/danfe_fixtures.py`
- Test: `py-dfe/tests/unit/test_danfe_fixtures.py`

**Interfaces:**
- Produces (`tests.danfe_fixtures`):
  - `sample_nfe_proc(*, tp_emis="1", tp_amb="1", with_dest=True, n_items=2) -> str` — returns a well-formed `<nfeProc>` NFC-e XML string with `protNFe`, `infNFeSupl/qrCode`, `infNFeSupl/urlChave`. Mutating `tp_emis`/`tp_amb` toggles contingency/homologation. `with_dest=False` omits `dest`.

This fixture is consumed by Tasks 6 and 8. It is plain data, no logic to unit-test beyond well-formedness.

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfe_fixtures.py`:
```python
from lxml import etree

from tests.danfe_fixtures import sample_nfe_proc


def test_sample_is_well_formed_and_has_key_nodes():
    xml = sample_nfe_proc()
    root = etree.fromstring(xml.encode("utf-8"))
    ns = {"n": "http://www.portalfiscal.inf.br/nfe"}
    assert root.find(".//n:infNFeSupl/n:qrCode", ns) is not None
    assert root.find(".//n:protNFe", ns) is not None
    assert len(root.findall(".//n:det", ns)) == 2


def test_contingency_toggle():
    xml = sample_nfe_proc(tp_emis="9")
    assert "<tpEmis>9</tpEmis>" in xml
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_fixtures.py -v`
Expected: FAIL — `ModuleNotFoundError: tests.danfe_fixtures`.

- [ ] **Step 3: Implement**

`py-dfe/tests/danfe_fixtures.py`:
```python
"""Synthetic authorized NFC-e XML for DANFC-e tests. No real CNPJ/CPF."""

from __future__ import annotations

_NS = "http://www.portalfiscal.inf.br/nfe"
_CHAVE = "35260612345678000199650010000000011000000017"


def _items(n: int) -> str:
    rows = []
    for i in range(1, n + 1):
        rows.append(
            f"""<det nItem="{i}">
  <prod>
    <cProd>P{i:03d}</cProd>
    <xProd>PRODUTO TESTE {i}</xProd>
    <uCom>UN</uCom>
    <qCom>2.0000</qCom>
    <vUnCom>10.0000000000</vUnCom>
    <vProd>20.00</vProd>
  </prod>
</det>"""
        )
    return "\n".join(rows)


def sample_nfe_proc(
    *, tp_emis: str = "1", tp_amb: str = "1", with_dest: bool = True, n_items: int = 2
) -> str:
    dest = (
        """<dest>
    <CPF>12345678909</CPF>
    <xNome>CONSUMIDOR TESTE</xNome>
  </dest>"""
        if with_dest
        else ""
    )
    total_prod = f"{20.00 * n_items:.2f}"
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="{_NS}" versao="4.00">
  <NFe>
    <infNFe Id="NFe{_CHAVE}" versao="4.00">
      <ide>
        <cUF>35</cUF>
        <mod>65</mod>
        <serie>1</serie>
        <nNF>1</nNF>
        <dhEmi>2026-06-25T10:30:00-03:00</dhEmi>
        <tpAmb>{tp_amb}</tpAmb>
        <tpEmis>{tp_emis}</tpEmis>
      </ide>
      <emit>
        <CNPJ>12345678000199</CNPJ>
        <xNome>EMPRESA TESTE LTDA</xNome>
        <enderEmit>
          <xLgr>RUA EXEMPLO</xLgr>
          <nro>100</nro>
          <xBairro>CENTRO</xBairro>
          <xMun>SAO PAULO</xMun>
          <UF>SP</UF>
          <CEP>01000000</CEP>
        </enderEmit>
      </emit>
      {dest}
      {_items(n_items)}
      <total>
        <ICMSTot>
          <vProd>{total_prod}</vProd>
          <vFrete>0.00</vFrete>
          <vSeg>0.00</vSeg>
          <vDesc>0.00</vDesc>
          <vOutro>0.00</vOutro>
          <vNF>{total_prod}</vNF>
          <vTotTrib>5.00</vTotTrib>
        </ICMSTot>
      </total>
      <pag>
        <detPag>
          <tPag>01</tPag>
          <vPag>{total_prod}</vPag>
        </detPag>
        <vTroco>0.00</vTroco>
      </pag>
      <infAdic>
        <infAdFisco>Mensagem fiscal de teste</infAdFisco>
        <infCpl>Obrigado pela preferencia</infCpl>
      </infAdic>
    </infNFe>
    <infNFeSupl>
      <qrCode>https://www.fazenda.sp.gov.br/nfce/qrcode?p={_CHAVE}|2|{tp_amb}|1|ABCDEF1234567890ABCDEF1234567890ABCDEF12</qrCode>
      <urlChave>https://www.fazenda.sp.gov.br/nfce/consulta</urlChave>
    </infNFeSupl>
  </NFe>
  <protNFe versao="4.00">
    <infProt>
      <chNFe>{_CHAVE}</chNFe>
      <nProt>135260000000017</nProt>
      <dhRecbto>2026-06-25T10:30:05-03:00</dhRecbto>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso da NF-e</xMotivo>
    </infProt>
  </protNFe>
</nfeProc>"""
```

- [ ] **Step 4: Run test, verify it passes**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfe_fixtures.py -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Stage (do not commit)**

```bash
git add py-dfe/tests/danfe_fixtures.py py-dfe/tests/unit/test_danfe_fixtures.py
```

---

### Task 6: `danfce.py` — context build + variant logic + entrypoint

**Files:**
- Create: `py-dfe/py_dfe/danfe/danfce.py`
- Create: `py-dfe/py_dfe/danfe/templates/danfce.html`
- Test: `py-dfe/tests/unit/test_danfce.py`

**Interfaces:**
- Consumes: `parse_xml_bytes` (`py_dfe.xmlops.builder`); `formatters.*`; `qr.qr_data_uri`; `render.render_html` + `render.html_to_pdf`; all `constants/danfe.py` names; `exceptions` codes.
- Produces (`py_dfe.danfe.danfce`):
  - `generate_danfce(payload: dict) -> dict` — `{"pdf_b64": str, "html": list[str]}`. Reads `payload["xml"]` (required), `payload.get("layout", LAYOUT_COMPLETO)`, `payload.get("canceled", False)`.
  - `build_context(inf_nfe: dict, prot: dict | None, *, layout: str, canceled: bool, tp_emis: str, tp_amb: str, chave: str) -> dict` — assembles the template context (no rendering).

**Context dict shape produced by `build_context`** (the template in this task consumes exactly these keys):
```
{
  "emit": {"cnpj": str, "nome": str, "endereco": str},
  "show_items": bool,                 # False for resumido
  "items": [{"cProd","xProd","qCom","uCom","vUnCom","vProd"}],
  "totals": {"qtd": int, "vProd","vDesc","vFrete","vSeg","vOutro","vNF": str,
             "has_acrescimo_desconto": bool, "vTotTrib": str,
             "pagamentos": [{"forma","valor"}], "troco": str},
  "chave_fmt": str,                   # 11 blocks
  "url_chave": str,
  "qr_uri": str,
  "consumidor": str,                  # "CONSUMIDOR ..." or "CONSUMIDOR NÃO IDENTIFICADO"
  "ident": {"nNF","serie","dhEmi": str},
  "protocolo": {"nProt","dhRecbto": str} | None,   # None in contingency
  "is_contingencia": bool,
  "is_homologacao": bool,
  "is_cancelada": bool,
  "msg_fiscal": str,                  # infAdFisco
  "msg_contribuinte": str,            # infCpl
  "vias": [{"label": str | None}],    # 1 entry normally; 2 in contingency
}
```

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfce.py`:
```python
import base64

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.danfce import generate_danfce
from py_dfe.exceptions import DFeError
from tests.danfe_fixtures import sample_nfe_proc


def _gen(**kw):
    payload = {"xml": sample_nfe_proc(**kw.pop("fixture", {})), **kw}
    return generate_danfce(payload)


def test_completo_returns_pdf_and_single_via():
    weasyprint = pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc()})
    assert base64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    assert len(out["html"]) == 1
    html = out["html"][0]
    assert c.TEXT_DOC_AUXILIAR in html
    assert "PRODUTO TESTE 1" in html      # item detail present
    assert "Protocolo" in html
    assert "12.345.678/0001-99" in html   # masked emit CNPJ


def test_resumido_omits_items():
    pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc(), "layout": c.LAYOUT_RESUMIDO})
    assert "PRODUTO TESTE 1" not in out["html"][0]


def test_contingencia_two_vias_no_protocol():
    pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc(tp_emis=c.TP_EMIS_CONTINGENCIA_OFFLINE)})
    assert len(out["html"]) == 2
    joined = "".join(out["html"])
    assert c.TEXT_CONTINGENCIA_L1 in joined
    assert c.VIA_ESTABELECIMENTO in out["html"][1]
    assert "135260000000017" not in joined   # protocol suppressed


def test_homologacao_banner():
    pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc(tp_amb=c.TP_AMB_HOMOLOGACAO)})
    assert c.TEXT_HOMOLOGACAO in out["html"][0]


def test_cancelada_watermark():
    pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc(), "canceled": True})
    assert c.TEXT_WATERMARK_CANCELADA in out["html"][0]


def test_consumidor_nao_identificado():
    pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc(with_dest=False)})
    assert c.TEXT_CONSUMIDOR_NAO_IDENTIFICADO in out["html"][0]


def test_unsupported_model_raises():
    bad = sample_nfe_proc().replace("<mod>65</mod>", "<mod>55</mod>")
    with pytest.raises(DFeError) as exc:
        generate_danfce({"xml": bad})
    assert exc.value.status_code == 422
    assert exc.value.error_code  # DANFE_UNSUPPORTED_MODEL


def test_invalid_xml_raises():
    with pytest.raises(DFeError) as exc:
        generate_danfce({"xml": "<not-xml"})
    assert exc.value.status_code == 422


def test_missing_xml_key_raises():
    with pytest.raises(DFeError) as exc:
        generate_danfce({})
    assert exc.value.status_code == 422
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfce.py -v`
Expected: FAIL — `ModuleNotFoundError: py_dfe.danfe.danfce`.

- [ ] **Step 3: Implement `danfce.py`**

`py-dfe/py_dfe/danfe/danfce.py`:
```python
"""DANFC-e (DANFE NFC-e) generation from an authorized NFC-e XML."""

from __future__ import annotations

import base64
from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe import formatters as fmt
from py_dfe.danfe.qr import qr_data_uri
from py_dfe.danfe.render import html_to_pdf, render_html
from py_dfe.exceptions import (
    DANFE_INVALID_XML,
    DANFE_UNSUPPORTED_MODEL,
    DFeError,
)
from py_dfe.xmlops.builder import parse_xml_bytes

_TEMPLATE = "danfce.html"


def generate_danfce(payload: dict[str, Any]) -> dict[str, Any]:
    """Render a DANFC-e from an authorized NFC-e XML payload."""
    xml = payload.get("xml")
    if not xml:
        raise DFeError(422, DANFE_INVALID_XML, "Missing 'xml' in body")
    layout = payload.get("layout", c.LAYOUT_COMPLETO)
    if layout not in c.VALID_LAYOUTS:
        layout = c.LAYOUT_COMPLETO
    canceled = bool(payload.get("canceled", False))

    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    context = build_context(
        inf_nfe, prot, layout=layout, canceled=canceled,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave,
    )

    htmls = [
        render_html(_TEMPLATE, {**context, "via": via})
        for via in context["vias"]
    ]
    combined = _combine_pages(htmls)
    pdf = html_to_pdf(combined)
    return {"pdf_b64": base64.b64encode(pdf).decode("ascii"), "html": htmls}


def _combine_pages(htmls: list[str]) -> str:
    """Join per-via HTML fragments into one document, one page each."""
    body = '<div style="page-break-after: always;">'.join(htmls)
    return f"<html><head><meta charset='utf-8'></head><body>{body}</body></html>"


def _extract_roots(xml: str) -> tuple[dict, dict | None, str, str, str]:
    try:
        parsed = parse_xml_bytes(xml.encode("utf-8") if isinstance(xml, str) else xml)
    except etree.XMLSyntaxError as exc:
        raise DFeError(422, DANFE_INVALID_XML, f"Malformed XML: {exc}") from exc

    root = next(iter(parsed.values()))
    # Accept either <nfeProc> (with NFe + protNFe) or a bare <NFe>.
    nfe = root.get("NFe", root)
    prot = root.get("protNFe") if isinstance(root, dict) else None
    inf_nfe = nfe.get("infNFe") if isinstance(nfe, dict) else None
    if not isinstance(inf_nfe, dict):
        raise DFeError(422, DANFE_INVALID_XML, "infNFe not found")
    inf_nfe = {**inf_nfe, "_infNFeSupl": nfe.get("infNFeSupl", {})}

    ide = inf_nfe.get("ide", {})
    if ide.get("mod") != c.MODELO_NFCE:
        raise DFeError(
            422, DANFE_UNSUPPORTED_MODEL,
            f"DANFC-e requires model 65, got {ide.get('mod')!r}",
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


def build_context(
    inf_nfe: dict, prot: dict | None, *, layout: str, canceled: bool,
    tp_emis: str, tp_amb: str, chave: str,
) -> dict[str, Any]:
    is_contingencia = tp_emis == c.TP_EMIS_CONTINGENCIA_OFFLINE
    is_homologacao = tp_amb == c.TP_AMB_HOMOLOGACAO
    supl = inf_nfe.get("_infNFeSupl", {})

    emit = inf_nfe.get("emit", {})
    ender = emit.get("enderEmit", {})

    det = inf_nfe.get("det", [])
    if isinstance(det, dict):
        det = [det]
    items = [
        {
            "cProd": p.get("cProd", ""),
            "xProd": p.get("xProd", ""),
            "qCom": p.get("qCom", ""),
            "uCom": p.get("uCom", ""),
            "vUnCom": fmt.money_br(p.get("vUnCom")),
            "vProd": fmt.money_br(p.get("vProd")),
        }
        for d in det
        for p in [d.get("prod", {})]
    ]

    icms = (inf_nfe.get("total", {}) or {}).get("ICMSTot", {})
    pag = inf_nfe.get("pag", {})
    detpag = pag.get("detPag", [])
    if isinstance(detpag, dict):
        detpag = [detpag]
    pagamentos = [
        {
            "forma": c.TPAG_LABELS.get(dp.get("tPag", ""), dp.get("tPag", "")),
            "valor": fmt.money_br(dp.get("vPag")),
        }
        for dp in detpag
    ]
    acrescimo_desconto = any(
        (icms.get(k) or "0") not in ("0", "0.00", "")
        for k in ("vFrete", "vSeg", "vOutro", "vDesc")
    )

    totals = {
        "qtd": len(det),
        "vProd": fmt.money_br(icms.get("vProd")),
        "vDesc": fmt.money_br(icms.get("vDesc")),
        "vFrete": fmt.money_br(icms.get("vFrete")),
        "vSeg": fmt.money_br(icms.get("vSeg")),
        "vOutro": fmt.money_br(icms.get("vOutro")),
        "vNF": fmt.money_br(icms.get("vNF")),
        "vTotTrib": fmt.money_br(icms.get("vTotTrib")),
        "has_acrescimo_desconto": acrescimo_desconto,
        "pagamentos": pagamentos,
        "troco": fmt.money_br(pag.get("vTroco")),
    }

    ide = inf_nfe.get("ide", {})
    protocolo = None
    if prot and not is_contingencia:
        ip = prot.get("infProt", {})
        protocolo = {"nProt": ip.get("nProt", ""), "dhRecbto": fmt.dt_local(ip.get("dhRecbto"))}

    infadic = inf_nfe.get("infAdic", {}) or {}

    vias = [{"label": None}]
    if is_contingencia:
        vias = [{"label": c.VIA_CONSUMIDOR}, {"label": c.VIA_ESTABELECIMENTO}]

    return {
        "emit": {
            "cnpj": fmt.mask_cnpj(emit.get("CNPJ", "")),
            "nome": emit.get("xNome", ""),
            "endereco": _endereco(ender),
        },
        "show_items": layout == c.LAYOUT_COMPLETO,
        "items": items,
        "totals": totals,
        "chave_fmt": fmt.chave_blocks(chave),
        "url_chave": supl.get("urlChave", ""),
        "qr_uri": qr_data_uri(supl.get("qrCode", "")),
        "consumidor": _consumidor(inf_nfe.get("dest")),
        "ident": {
            "nNF": ide.get("nNF", ""),
            "serie": ide.get("serie", ""),
            "dhEmi": fmt.dt_local(ide.get("dhEmi")),
        },
        "protocolo": protocolo,
        "is_contingencia": is_contingencia,
        "is_homologacao": is_homologacao,
        "is_cancelada": canceled,
        "msg_fiscal": infadic.get("infAdFisco", ""),
        "msg_contribuinte": infadic.get("infCpl", ""),
        "vias": vias,
        "text": {
            "doc_auxiliar": c.TEXT_DOC_AUXILIAR,
            "cont_l1": c.TEXT_CONTINGENCIA_L1,
            "cont_l2": c.TEXT_CONTINGENCIA_L2,
            "homologacao": c.TEXT_HOMOLOGACAO,
            "cancelada": c.TEXT_WATERMARK_CANCELADA,
        },
    }


def _endereco(ender: dict) -> str:
    parts = [
        ender.get("xLgr", ""), ender.get("nro", ""), ender.get("xBairro", ""),
        ender.get("xMun", ""), ender.get("UF", ""),
    ]
    return ", ".join(p for p in parts if p)


def _consumidor(dest: dict | None) -> str:
    if not dest:
        return c.TEXT_CONSUMIDOR_NAO_IDENTIFICADO
    if dest.get("CNPJ"):
        return f"CONSUMIDOR CNPJ: {fmt.mask_cnpj(dest['CNPJ'])}"
    if dest.get("CPF"):
        return f"CONSUMIDOR CPF: {fmt.mask_cpf(dest['CPF'])}"
    if dest.get("idEstrangeiro"):
        return f"CONSUMIDOR Id. Estrangeiro: {dest['idEstrangeiro']}"
    return c.TEXT_CONSUMIDOR_NAO_IDENTIFICADO
```

- [ ] **Step 4: Implement the template `danfce.html`**

`py-dfe/py_dfe/danfe/templates/danfce.html`:
```html
<style>
  @page { size: 58mm auto; margin: 2mm; }
  body { font-family: "DejaVu Sans", sans-serif; font-size: 7pt; color: #000; }
  .receipt { width: 54mm; position: relative; }
  .center { text-align: center; }
  .b { font-weight: bold; }
  hr { border: none; border-top: 1px dashed #000; margin: 2px 0; }
  table { width: 100%; border-collapse: collapse; }
  td { vertical-align: top; }
  .right { text-align: right; }
  .qr { width: 25mm; height: 25mm; display: block; margin: 2px auto; }
  .banner { font-weight: bold; text-align: center; margin: 3px 0; }
  .watermark {
    position: absolute; top: 40%; left: 0; width: 100%;
    text-align: center; font-size: 28pt; color: rgba(0,0,0,0.15);
    transform: rotate(-25deg); font-weight: bold;
  }
</style>
<div class="receipt">
  {% if is_cancelada %}<div class="watermark">{{ text.cancelada }}</div>{% endif %}
  {% if is_homologacao %}<div class="watermark">HOMOLOGAÇÃO</div>{% endif %}

  {# Divisão I — Cabeçalho #}
  <div class="center b">{{ emit.nome }}</div>
  <div class="center">CNPJ: {{ emit.cnpj }}</div>
  <div class="center">{{ emit.endereco }}</div>
  <div class="center b">{{ text.doc_auxiliar }}</div>
  <hr>

  {% if is_contingencia %}
  <div class="banner">{{ text.cont_l1 }}<br>{{ text.cont_l2 }}</div><hr>
  {% endif %}

  {# Divisão II — Itens (completo only) #}
  {% if show_items %}
  <table>
    <thead><tr><td>Cód</td><td>Descr</td><td class="right">Qtd</td><td>Un</td><td class="right">VlUn</td><td class="right">VlTot</td></tr></thead>
    <tbody>
      {% for it in items %}
      <tr><td>{{ it.cProd }}</td><td>{{ it.xProd }}</td><td class="right">{{ it.qCom }}</td><td>{{ it.uCom }}</td><td class="right">{{ it.vUnCom }}</td><td class="right">{{ it.vProd }}</td></tr>
      {% endfor %}
    </tbody>
  </table>
  <hr>
  {% endif %}

  {# Divisão III — Totais #}
  <table>
    <tr><td>Qtde. total de itens</td><td class="right">{{ totals.qtd }}</td></tr>
    <tr><td>Valor total R$</td><td class="right">{{ totals.vProd }}</td></tr>
    {% if totals.has_acrescimo_desconto %}
    <tr><td>Descontos R$</td><td class="right">{{ totals.vDesc }}</td></tr>
    <tr><td>Acréscimos R$</td><td class="right">{{ totals.vFrete }}</td></tr>
    <tr><td class="b">Valor a pagar R$</td><td class="right b">{{ totals.vNF }}</td></tr>
    {% else %}
    <tr><td class="b">Valor a pagar R$</td><td class="right b">{{ totals.vNF }}</td></tr>
    {% endif %}
    {% for p in totals.pagamentos %}
    <tr><td>{{ p.forma }}</td><td class="right">{{ p.valor }}</td></tr>
    {% endfor %}
    <tr><td>Troco R$</td><td class="right">{{ totals.troco }}</td></tr>
  </table>
  <hr>

  {# Divisão IV — Consulta via chave #}
  <div class="center">Consulte pela Chave de Acesso em</div>
  <div class="center">{{ url_chave }}</div>
  <div class="center">{{ chave_fmt }}</div>
  <hr>

  {# Divisão VI — Consumidor #}
  <div class="center">{{ consumidor }}</div>
  <hr>

  {# Divisão VII — Identificação + Protocolo #}
  <div class="center">NFC-e nº {{ ident.nNF }} Série {{ ident.serie }} {{ ident.dhEmi }}
    {%- if via.label %} - {{ via.label }}{% endif %}</div>
  {% if protocolo %}
  <div class="center">Protocolo de autorização: {{ protocolo.nProt }} {{ protocolo.dhRecbto }}</div>
  {% endif %}
  {% if is_contingencia %}
  <div class="banner">{{ text.cont_l1 }}<br>{{ text.cont_l2 }}</div>
  {% endif %}
  {% if is_homologacao %}
  <div class="banner">{{ text.homologacao }}</div>
  {% endif %}
  <hr>

  {# Divisão V — QR Code #}
  <img class="qr" src="{{ qr_uri }}" alt="QR Code">
  <hr>

  {# Divisão VIII — Mensagem fiscal #}
  {% if msg_fiscal %}<div>{{ msg_fiscal }}</div><hr>{% endif %}

  {# Divisão IX — Mensagem do contribuinte #}
  {% if msg_contribuinte %}<div>{{ msg_contribuinte }}</div>{% endif %}
</div>
```

- [ ] **Step 5: Run tests, verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfce.py -v`
Expected: PASS — error-path tests (`test_unsupported_model_raises`, `test_invalid_xml_raises`, `test_missing_xml_key_raises`) PASS unconditionally; render tests PASS if WeasyPrint native libs present, else SKIPPED.

- [ ] **Step 6: Stage (do not commit)**

```bash
git add py-dfe/py_dfe/danfe/danfce.py py-dfe/py_dfe/danfe/templates/danfce.html py-dfe/tests/unit/test_danfce.py
```

---

### Task 7: Routing — optional certificate + `GerarDanfe` branch

**Files:**
- Modify: `py-dfe/py_dfe/models/request.py:37-39` (certificate fields optional)
- Modify: `py-dfe/py_dfe/handler.py:42-67` (conditional cert + CERT_REQUIRED guard + log guard)
- Modify: `py-dfe/py_dfe/services/_nf.py:62-82` (branch in `call`)
- Test: `py-dfe/tests/unit/test_danfce_routing.py`
- Test (modify): existing `py-dfe/tests/unit/test_handler.py` (ensure still green)

**Interfaces:**
- Consumes: `generate_danfce` (Task 6); `SERVICE_GERAR_DANFE` (Task 1).
- Produces: a `LambdaRequest` accepting absent certificate; `handler` that routes `GerarDanfe` without a certificate and rejects cert-less SEFAZ services with `DFeError(400, CERT_REQUIRED, ...)`.

- [ ] **Step 1: Write the failing test**

`py-dfe/tests/unit/test_danfce_routing.py`:
```python
import json

import pytest

from py_dfe.constants import danfe as c
from py_dfe.handler import handler
from py_dfe.models.request import LambdaRequest
from tests.danfe_fixtures import sample_nfe_proc


def test_request_accepts_missing_certificate():
    req = LambdaRequest.model_validate({
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": c.SERVICE_GERAR_DANFE,
        "body": {"xml": "<x/>"},
    })
    assert req.certificate_b64 is None


def test_handler_routes_gerardanfe_without_cert():
    pytest.importorskip("weasyprint")
    event = {
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": c.SERVICE_GERAR_DANFE,
        "body": {"xml": sample_nfe_proc(), "layout": c.LAYOUT_COMPLETO},
    }
    resp = handler(event, None)
    assert resp["statusCode"] == 200
    body = json.loads(resp["body"])
    assert "pdf_b64" in body and body["html"]


def test_handler_rejects_sefaz_service_without_cert():
    event = {
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": "NfeStatusServico", "body": {},
    }
    resp = handler(event, None)
    assert resp["statusCode"] == 400  # CERT_REQUIRED
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfce_routing.py -v`
Expected: FAIL — `LambdaRequest` still requires `certificate_b64` (ValidationError).

- [ ] **Step 3: Make certificate fields optional**

`models/request.py` — replace lines 37-39:
```python
    cnpj: str = Field(..., pattern=r"^\d{14}$")
    certificate_b64: str | None = Field(default=None)
    certificate_password: str | None = Field(default=None)
```

- [ ] **Step 4: Update the handler**

`handler.py` — replace the log block + service construction (lines 42-68) with:
```python
    logger.info(
        "Request: doc_type=%s service=%s uf=%s environment=%s cnpj=%s "
        "validate_schema=%s max_retries=%s cert_b64_len=%d body_keys=%s",
        req.doc_type,
        req.service,
        req.uf,
        req.environment,
        _mask_cnpj(req.cnpj),
        req.validate_schema,
        req.max_retries,
        len(req.certificate_b64) if req.certificate_b64 else 0,
        list(req.body.keys())
        if isinstance(req.body, dict)
        else type(req.body).__name__,
    )

    try:
        cert_manager = None
        if req.certificate_b64:
            cert_manager = CertificateManager(
                req.certificate_b64, req.certificate_password
            )
        elif req.service != SERVICE_GERAR_DANFE:
            raise DFeError(
                400, CERT_REQUIRED,
                f"Service {req.service!r} requires a certificate",
            )
        service = create_service(
            doc_type=req.doc_type,
            cert_manager=cert_manager,
            uf=req.uf,
            environment=req.environment,
            validate_schema=req.validate_schema,
            max_retries=req.max_retries,
        )
        result = service.call(req.service, req.body)
```
Add imports at the top of `handler.py`:
```python
from py_dfe.constants.danfe import SERVICE_GERAR_DANFE
from py_dfe.exceptions import CERT_REQUIRED
```
(extend the existing `from py_dfe.exceptions import (...)` block to include `CERT_REQUIRED`, and the existing `DFeError` import already present).

- [ ] **Step 5: Branch in `_NFServiceClient.call`**

`services/_nf.py` — add import near the top:
```python
from py_dfe.constants.danfe import SERVICE_GERAR_DANFE
from py_dfe.danfe.danfce import generate_danfce
```
Then at the very start of `call` (before `authorizer = get_authorizer(...)`):
```python
    def call(self, service: str, payload: dict[str, Any]) -> dict[str, Any]:
        """Generic call for any NF-e service."""
        if service == SERVICE_GERAR_DANFE:
            return generate_danfce(payload)
        authorizer = get_authorizer(self._client.doc_type, self._client.uf)
        ...
```

- [ ] **Step 6: Run tests, verify they pass**

Run: `cd py-dfe && python -m pytest tests/unit/test_danfce_routing.py tests/unit/test_handler.py -v`
Expected: PASS — `test_request_accepts_missing_certificate` and `test_handler_rejects_sefaz_service_without_cert` PASS unconditionally; `test_handler_routes_gerardanfe_without_cert` PASS if WeasyPrint present, else SKIPPED. Existing `test_handler.py` still green.

- [ ] **Step 7: Stage (do not commit)**

```bash
git add py-dfe/py_dfe/models/request.py py-dfe/py_dfe/handler.py py-dfe/py_dfe/services/_nf.py py-dfe/tests/unit/test_danfce_routing.py
```

---

### Task 8: Integration test + documentation

**Files:**
- Create: `py-dfe/tests/integration/test_danfce_generation.py`
- Modify: `DOCS.md` (§3 — new render service)
- Modify: `CONDUCT.md` (WeasyPrint native-lib constraint)

**Interfaces:**
- Consumes: `handler` (Task 7); `sample_nfe_proc` (Task 5); constants (Task 1).

- [ ] **Step 1: Write the integration test**

`py-dfe/tests/integration/test_danfce_generation.py`:
```python
"""Integration: full DANFC-e render through the Lambda handler.

Skipped when WeasyPrint native libraries are unavailable.
"""

from __future__ import annotations

import base64
import json

import pytest

from py_dfe.constants import danfe as c
from py_dfe.handler import handler
from tests.danfe_fixtures import sample_nfe_proc

pytestmark = pytest.mark.integration


def _event(**body):
    return {
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": c.SERVICE_GERAR_DANFE, "body": body,
    }


def setup_module(_):
    pytest.importorskip("weasyprint")


def test_full_completo_pdf_and_html():
    resp = handler(_event(xml=sample_nfe_proc(), layout=c.LAYOUT_COMPLETO), None)
    assert resp["statusCode"] == 200
    body = json.loads(resp["body"])
    assert base64.b64decode(body["pdf_b64"])[:4] == b"%PDF"
    assert len(body["html"]) == 1
    html = body["html"][0]
    for needle in (c.TEXT_DOC_AUXILIAR, "PRODUTO TESTE 1", "data:image/png;base64,", "Protocolo"):
        assert needle in html


def test_full_contingencia_two_vias():
    resp = handler(_event(xml=sample_nfe_proc(tp_emis=c.TP_EMIS_CONTINGENCIA_OFFLINE)), None)
    body = json.loads(resp["body"])
    assert len(body["html"]) == 2
    assert c.VIA_ESTABELECIMENTO in body["html"][1]
    assert c.TEXT_CONTINGENCIA_L1 in "".join(body["html"])


def test_full_homologacao_and_resumido():
    resp = handler(_event(xml=sample_nfe_proc(tp_amb=c.TP_AMB_HOMOLOGACAO), layout=c.LAYOUT_RESUMIDO), None)
    body = json.loads(resp["body"])
    html = body["html"][0]
    assert c.TEXT_HOMOLOGACAO in html
    assert "PRODUTO TESTE 1" not in html
```

- [ ] **Step 2: Run the integration test**

Run: `cd py-dfe && python -m pytest tests/integration/test_danfce_generation.py -v`
Expected: PASS if WeasyPrint native libs present, else SKIPPED (no SEFAZ/cert needed — these run without `TEST_CERT_PATH`).

- [ ] **Step 3: Run the full unit suite (regression)**

Run: `cd py-dfe && python -m pytest tests/unit -v`
Expected: All PASS (render-dependent ones SKIPPED if WeasyPrint native libs missing). No existing test broken.

- [ ] **Step 4: Update `DOCS.md` §3**

Add a row/subsection documenting the new render service: `doc_type="nfce"`, `service="GerarDanfe"`, no certificate, body `{xml, layout?, canceled?}`, returns `{pdf_b64, html[]}`. Variants: completo/resumido (`layout`), contingência auto (`tpEmis=9`, 2 vias), homologação auto (`tpAmb=2`), cancelada (`canceled`). Note QR/data read only from XML.

- [ ] **Step 5: Update `CONDUCT.md`**

Add a constraint entry: "py-dfe DANFC-e rendering uses WeasyPrint, which requires native libraries (cairo, pango, gdk-pixbuf, glib, gobject, fonts) bundled in the Lambda layer/image. CDK layer build MUST include them. Render/QR are pure-local — no certificate, no SEFAZ."

- [ ] **Step 6: Stage (do not commit)**

```bash
git add py-dfe/tests/integration/test_danfce_generation.py DOCS.md CONDUCT.md
```

---

## Cross-project impact

- **py-dfe:** all new code (this plan).
- **cdk:** Lambda layer MUST bundle WeasyPrint native libraries — flagged in `CONDUCT.md`; layer build change owned by cdk, out of scope here.
- **api / worker:** `api/internal/services/nfes/danfe.go` currently calls `consultadanfe.com`. Swapping it to invoke the `GerarDanfe` py-dfe service is a follow-up (not in scope).
- **ui:** unchanged (existing DANFE download button keeps working via the api path).

## Self-Review

- **Spec coverage:** §4 architecture → Tasks 2-6; §5 routing → Tasks 1,7; §6 extraction → Task 6 `build_context`; §7 all variants (completo/resumido/contingência-2vias/homologação/cancelada) → Task 6 tests + template; §8 output shape → Task 6 `generate_danfce`; §9 paper/QR sizing → Task 6 template `@page`/`.qr`; §10 deps + cross-project → Task 1 + Task 8 docs; §11 error handling → Tasks 1,3,6,7 (all DFeError codes covered); §12 testing → Tasks 2-8.
- **Placeholder scan:** none — every code/test step contains full content; docs steps (8.4/8.5) specify exact text to add.
- **Type consistency:** `generate_danfce(payload)->{pdf_b64,html[]}` consistent across Tasks 6,7,8; `qr_data_uri`, `render_html`, `html_to_pdf`, `money_br`/`dt_local`/`mask_cnpj`/`mask_cpf`/`chave_blocks`, `sample_nfe_proc(tp_emis,tp_amb,with_dest,n_items)`, `SERVICE_GERAR_DANFE` all used with matching signatures across tasks.
- **Manual coverage:** Divisões I-IX all present in `danfce.html`; contingency 2-via + "Via do Estabelecimento" + suppressed protocol (manual §3.1.7/§3.1.8); homologação banner (§3.1.8); QR level M/UTF-8 (§4.5); 58mm/2mm margins + 25mm QR (§3.3/§3.4); chave 11 blocks (§3.1.4); money comma-decimal (§3.1.2).
