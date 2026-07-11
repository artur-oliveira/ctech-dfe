import base64

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.danfce import generate_danfce
from py_dfe.exceptions import DFeError
from tests.danfe_fixtures import sample_nfe_proc


def test_completo_returns_pdf_and_single_via():
    pytest.importorskip("weasyprint")
    out = generate_danfce({"xml": sample_nfe_proc()})
    assert base64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    assert len(out["html"]) == 1
    html = out["html"][0]
    assert c.TEXT_DOC_AUXILIAR in html
    assert "PRODUTO TESTE 1" in html  # item detail present
    assert "Protocolo" in html
    assert "12.345.678/0001-99" in html  # masked emit CNPJ


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
    assert "135260000000017" not in joined  # protocol suppressed


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
