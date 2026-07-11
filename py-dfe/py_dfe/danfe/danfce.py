"""DANFC-e (DANFE NFC-e) generation from an authorized NFC-e XML."""

from __future__ import annotations

import base64
from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe import formatters as fmt
from py_dfe.danfe.qr import qr_data_uri
from py_dfe.danfe.render import htmls_to_pdf, render_html
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
    site = payload.get("site") or c.DEFAULT_FOOTER_SITE

    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    context = build_context(
        inf_nfe, prot, layout=layout, canceled=canceled,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave, site=site,
    )

    htmls = [
        render_html(_TEMPLATE, {**context, "via": via})
        for via in context["vias"]
    ]
    pdf = htmls_to_pdf(htmls)
    return {"pdf_b64": base64.b64encode(pdf).decode("ascii"), "html": htmls}


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
    tp_emis: str, tp_amb: str, chave: str, site: str = c.DEFAULT_FOOTER_SITE,
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
        "site": site,
        "gerado_em": fmt.now_br(),
        "vias": vias,
        "text": {
            "doc_auxiliar": c.TEXT_DOC_AUXILIAR,
            "cont_l1": c.TEXT_CONTINGENCIA_L1,
            "cont_l2": c.TEXT_CONTINGENCIA_L2,
            "homologacao": c.TEXT_HOMOLOGACAO,
            "cancelada": c.TEXT_WATERMARK_CANCELADA,
            "gerado_por": c.TEXT_GERADO_POR,
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
