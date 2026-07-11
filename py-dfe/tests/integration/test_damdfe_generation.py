import base64
import io

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.mdfe58 import generate_damdfe
from tests.danfe_fixtures import sample_mdfe58_proc

pytestmark = pytest.mark.integration


def _pdf(out):
    raw = base64.b64decode(out["pdf_b64"])
    assert raw[:4] == b"%PDF"
    return raw


@pytest.mark.parametrize("layout", [c.LAYOUT_RETRATO, c.LAYOUT_PAISAGEM])
def test_each_layout_end_to_end(layout):
    pytest.importorskip("weasyprint")
    out = generate_damdfe({"xml": sample_mdfe58_proc(), "layout": layout})
    _pdf(out)


@pytest.mark.parametrize("modal", [
    c.MODAL_RODOVIARIO, c.MODAL_AEREO, c.MODAL_AQUAVIARIO, c.MODAL_FERROVIARIO,
])
def test_each_modal_end_to_end(modal):
    pytest.importorskip("weasyprint")
    out = generate_damdfe({"xml": sample_mdfe58_proc(modal=modal)})
    _pdf(out)


def test_contingencia_end_to_end():
    pytest.importorskip("weasyprint")
    out = generate_damdfe({"xml": sample_mdfe58_proc(tp_emis="2", with_prot=False)})
    raw = _pdf(out)
    assert c.TEXT_MDFE_CONTINGENCIA in out["html"][0]


def test_homologacao_watermark_end_to_end():
    pytest.importorskip("weasyprint")
    out = generate_damdfe({"xml": sample_mdfe58_proc(tp_amb="2")})
    _pdf(out)
    assert c.TEXT_MDFE_HOMOLOGACAO in out["html"][0]


def test_paginates_with_many_documents():
    pytest.importorskip("weasyprint")
    from pypdf import PdfReader
    out = generate_damdfe({"xml": sample_mdfe58_proc(n_docs=120)})
    reader = PdfReader(io.BytesIO(_pdf(out)))
    assert len(reader.pages) >= 2


def test_via_mdfe_service_call_no_certificate():
    """GerarDamdfe routes through MDFeServiceClient without a certificate."""
    pytest.importorskip("weasyprint")
    from py_dfe.services.mdfe import MDFeServiceClient
    client = MDFeServiceClient(None, "SP", "homologacao", validate_schema=False)
    out = client.call(c.SERVICE_GERAR_DAMDFE, {"xml": sample_mdfe58_proc()})
    _pdf(out)
