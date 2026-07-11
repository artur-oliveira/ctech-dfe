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
    assert len(ctx["produtos"]) == 2
    assert ctx["produtos"][0]["NCM"] == "61099000"
    assert ctx["chave_fmt"].count(" ") == 10  # 11 blocks → 10 spaces
    assert ctx["chave_barcode"].startswith("data:image/svg+xml;base64,")
    assert ctx["show_protocolo"] is True
    assert ctx["is_fs"] is False
    assert ctx["transp"]["modFrete_label"] == "0 - Remetente"
    assert ctx["duplicatas"][0]["nDup"] == "001"
    assert ctx["emit"]["im"] == ""
    assert ctx["emit"]["logo_b64"] is None
    assert ctx["produtos"][0]["origCST"] == "00"  # no <orig> in fixture → CST only
    assert ctx["produtos"][0]["vDesc"] == "0,00"
    assert ctx["produtos"][0]["vIPI"] == "0,00"
    assert ctx["produtos"][0]["pIPI"] == "0,00"
    assert ctx["ide"]["dhEmi_data"] == "25/06/2026"
    assert ctx["ide"]["dhSaiEnt_data"] == "25/06/2026"
    assert ctx["ide"]["dhSaiEnt_hora"] == "10:35:00"
    for key in ("vII", "vICMSUFRemet", "vFCPUFDest", "vICMSUFDest"):
        assert ctx["totals"][key] == "0,00"  # absent in fixture → default zero


def test_context_logo_passthrough():
    xml = sample_nfe55_proc()
    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    ctx = build_context(
        inf_nfe, prot, layout=c.LAYOUT_RETRATO, canceled=False,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave,
        logo="data:image/png;base64,iVBORw0KGgo=",
    )
    assert ctx["emit"]["logo_b64"] == "data:image/png;base64,iVBORw0KGgo="


def test_context_orig_and_im_when_present():
    xml = sample_nfe55_proc().replace(
        "<CST>00</CST>", "<orig>1</orig><CST>00</CST>",
    ).replace(
        "<IE>110042490114</IE>", "<IE>110042490114</IE><IM>987654</IM>",
    )
    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    ctx = build_context(
        inf_nfe, prot, layout=c.LAYOUT_RETRATO, canceled=False,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave,
    )
    assert ctx["emit"]["im"] == "987654"
    assert ctx["produtos"][0]["origCST"] == "1/00"


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


# --- Rendering (templates) ---------------------------------------------------

import base64 as _b64  # noqa: E402

from py_dfe.danfe.nfe55 import generate_danfe_nfe  # noqa: E402


@pytest.mark.parametrize("layout", [c.LAYOUT_RETRATO, c.LAYOUT_PAISAGEM])
def test_a4_variants_render_pdf_with_all_quadros(layout):
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": layout})
    assert _b64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    html = out["html"][0]
    assert c.TEXT_DANFE in html
    assert c.TEXT_DANFE_DESC in html
    assert "EMPRESA TESTE LTDA" in html  # emit
    assert "CLIENTE TESTE LTDA" in html  # dest
    assert "VENDA DE MERCADORIA" in html  # natOp
    assert "PRODUTO TESTE 1" in html  # produtos
    assert "TRANSPORTADORA TESTE LTDA" in html  # transp
    assert "001" in html  # duplicata
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


def test_simplificado_has_mandatory_subset_only():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": c.LAYOUT_SIMPLIFICADO})
    assert _b64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    html = out["html"][0]
    assert c.TEXT_DANFE_SIMPLIFICADO in html
    assert "EMPRESA TESTE LTDA" in html
    assert "PRODUTO TESTE 1" in html  # itens included in simplificado
    assert "RESERVADO AO FISCO" not in html  # quadro omitted
    assert "TRANSPORTADOR" not in html  # quadro omitted


def test_etiqueta_omits_items():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": c.LAYOUT_ETIQUETA})
    assert _b64.b64decode(out["pdf_b64"])[:4] == b"%PDF"
    html = out["html"][0]
    assert c.TEXT_DANFE_ETIQUETA in html
    assert "PRODUTO TESTE 1" not in html  # etiqueta has no item list
    assert "CLIENTE TESTE LTDA" in html


def test_a4_emitente_without_logo_centers_data():
    pytest.importorskip("weasyprint")
    out = generate_danfe_nfe({"xml": sample_nfe55_proc(), "layout": c.LAYOUT_RETRATO})
    html = out["html"][0]
    assert 'class="emit-data center"' in html
    assert 'class="logo-col"' not in html


def test_a4_emitente_with_logo_reserves_space():
    pytest.importorskip("weasyprint")
    logo = "data:image/png;base64,iVBORw0KGgo="
    out = generate_danfe_nfe(
        {"xml": sample_nfe55_proc(), "layout": c.LAYOUT_RETRATO, "logo": logo}
    )
    html = out["html"][0]
    assert logo in html
    assert "logo-col" in html
    assert 'class="emit-data center"' not in html
