import base64

import pytest

from py_dfe.constants import danfe as c
from py_dfe.danfe.mdfe58 import build_context, generate_damdfe
from py_dfe.exceptions import DFeError
from tests.danfe_fixtures import sample_mdfe58_proc


# --- routing / extraction ---------------------------------------------------

def test_generate_missing_xml_raises():
    with pytest.raises(DFeError) as exc:
        generate_damdfe({})
    assert exc.value.status_code == 422


def test_generate_wrong_model_raises():
    bad = sample_mdfe58_proc().replace("<mod>58</mod>", "<mod>57</mod>")
    with pytest.raises(DFeError) as exc:
        generate_damdfe({"xml": bad})
    assert exc.value.status_code == 422


def test_generate_invalid_layout_falls_back_to_default():
    pytest.importorskip("weasyprint")
    out = generate_damdfe({"xml": sample_mdfe58_proc(), "layout": "bogus"})
    assert base64.b64decode(out["pdf_b64"])[:4] == b"%PDF"


def test_generate_damd():
    pytest.importorskip("weasyprint")
    out = generate_damdfe({"xml": sample_mdfe58_proc(), "layout": "bogus"})
    assert base64.b64decode(out["pdf_b64"])[:4] == b"%PDF"


# --- context content --------------------------------------------------------

def _ctx(**kw):
    from lxml import etree  # noqa: F401
    from py_dfe.danfe.mdfe58 import _extract_roots
    inf, supl, prot, tpe, tpa, ch = _extract_roots(sample_mdfe58_proc(**kw))
    return build_context(
        inf, supl, prot, layout=c.LAYOUT_RETRATO, canceled=False,
        tp_emis=tpe, tp_amb=tpa, chave=ch,
    )


def test_context_header_and_ide():
    ctx = _ctx()
    assert ctx["emit"]["nome"] == "TRANSPORTADORA TESTE LTDA"
    assert ctx["emit"]["doc"] == "12.345.678/0001-99"
    assert ctx["ide"]["ufIni"] == "SP"
    assert ctx["ide"]["ufFim"] == "RJ"
    assert ctx["ide"]["tpEmit_label"] == c.TP_EMIT_MDFE_LABELS["1"]
    assert ctx["ide"]["tpTransp_label"] == c.TP_TRANSP_MDFE_LABELS["1"]
    assert ctx["modal_label"] == "Rodoviário"
    assert ctx["carrega"] == ["SAO PAULO"]
    assert ctx["percurso"] == "MG"


def test_context_modal_rodoviario():
    mi = _ctx(modal="1")["modal_info"]
    assert mi["is_rodo"] is True
    assert mi["rntrc"] == "12345678"
    assert mi["veic"]["placa"] == "ABC1D23"
    assert mi["condutores"][0]["nome"] == "JOAO MOTORISTA"
    assert mi["condutores"][0]["cpf"] == "123.456.789-09"


@pytest.mark.parametrize("modal,key,flag", [
    ("2", "aereo", "is_aereo"),
    ("3", "aqua", "is_aqua"),
    ("4", "ferrov", "is_ferrov"),
])
def test_context_other_modais(modal, key, flag):
    mi = _ctx(modal=modal)["modal_info"]
    assert mi[flag] is True
    assert key in mi


def test_context_documents_and_totals():
    ctx = _ctx(n_docs=3)
    assert ctx["tot"]["qNFe"] == "3"
    assert ctx["tot"]["cUnid_label"] == "KG"
    muns = ctx["municipios"]
    assert len(muns) == 1
    assert muns[0]["mun"] == "RIO DE JANEIRO"
    assert muns[0]["count"] == 3
    assert all(d["tipo"] == c.DOC_TIPO_NFE for d in muns[0]["docs"])


def test_context_seguro_and_obs():
    ctx = _ctx()
    assert ctx["seguros"][0]["nome"] == "SEGURADORA TESTE"
    assert ctx["seguros"][0]["cnpj"] == "98.765.432/0001-88"
    assert ctx["lacres"] == ["LAC001"]
    assert "fisco" in ctx["msg_fiscal"]


def test_context_protocolo_present_when_authorized():
    ctx = _ctx()
    assert ctx["protocolo"]["nProt"] == "135260000000088"
    assert ctx["is_contingencia"] is False


def test_context_contingencia_suppresses_protocolo():
    ctx = _ctx(tp_emis="2", with_prot=False)
    assert ctx["protocolo"] is None
    assert ctx["is_contingencia"] is True


def test_context_homologacao_flag():
    assert _ctx(tp_amb="2")["is_homologacao"] is True
