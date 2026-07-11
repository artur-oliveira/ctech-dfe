import base64
import io

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.document import generate_danfe
from tests.danfe_fixtures import sample_nfe55_proc

pytestmark = pytest.mark.integration


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
