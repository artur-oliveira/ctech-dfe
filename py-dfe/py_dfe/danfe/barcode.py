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
