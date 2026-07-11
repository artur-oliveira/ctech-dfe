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


def test_mod11_dv_is_deterministic():
    # Chave-style mod-11 of the 43-digit body (weights 2..9 cycling, right→left).
    assert _mod11_dv("3526061234567800019955001000000001100000001") == "8"


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
    assert code[31] == "1"  # ICMSp present (1=há)
    assert code[32] == "2"  # ICMSs absent (2=não há)
    assert code[33:35] == "25"  # DD
    assert code[-1] == _mod11_dv(code[:-1])
