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
