"""DANF-e (NF-e modelo 55) generation from an authorized NF-e XML."""

from __future__ import annotations

import base64
from decimal import Decimal, InvalidOperation
from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe import formatters as fmt
from py_dfe.danfe.barcode import code128c_data_uri, dados_nfe_code
from py_dfe.danfe.render import htmls_to_pdf, render_html
from py_dfe.exceptions import DANFE_INVALID_XML, DANFE_UNSUPPORTED_MODEL, DFeError
from py_dfe.xmlops.builder import parse_xml_bytes


def generate_danfe_nfe(payload: dict[str, Any]) -> dict[str, Any]:
    """Render a DANF-e (mod 55) from an authorized NF-e XML payload."""
    xml = payload.get("xml")
    if not xml:
        raise DFeError(422, DANFE_INVALID_XML, "Missing 'xml' in body")
    layout = payload.get("layout", c.DEFAULT_DANFE_NFE_LAYOUT)
    if layout not in c.VALID_DANFE_NFE_LAYOUTS:
        layout = c.DEFAULT_DANFE_NFE_LAYOUT
    canceled = bool(payload.get("canceled", False))
    site = payload.get("site") or c.DEFAULT_FOOTER_SITE
    logo = payload.get("logo") or None

    inf_nfe, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    context = build_context(
        inf_nfe, prot, layout=layout, canceled=canceled,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave, site=site, logo=logo,
    )
    template = c.DANFE_NFE_TEMPLATES[layout]
    fit_height = layout in c.ROLL_LAYOUTS
    html = render_html(template, {"ctx": context})
    pdf = htmls_to_pdf([html], fit_height=fit_height)
    return {"pdf_b64": base64.b64encode(pdf).decode("ascii"), "html": [html]}


def _extract_roots(xml: str) -> tuple[dict, dict | None, str, str, str]:
    try:
        parsed = parse_xml_bytes(xml.encode("utf-8") if isinstance(xml, str) else xml)
    except etree.XMLSyntaxError as exc:
        raise DFeError(422, DANFE_INVALID_XML, f"Malformed XML: {exc}") from exc

    root = next(iter(parsed.values()))
    nfe = root.get("NFe", root)
    prot = root.get("protNFe") if isinstance(root, dict) else None
    inf_nfe = nfe.get("infNFe") if isinstance(nfe, dict) else None
    if not isinstance(inf_nfe, dict):
        raise DFeError(422, DANFE_INVALID_XML, "infNFe not found")

    ide = inf_nfe.get("ide", {})
    if ide.get("mod") != c.MODELO_NFE:
        raise DFeError(
            422, DANFE_UNSUPPORTED_MODEL,
            f"DANF-e requires model 55, got {ide.get('mod')!r}",
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


def _centavos(value: str | None) -> str:
    """Integer centavos string for the FS/FS-DA barcode (e.g. '123.45'→'12345')."""
    try:
        dec = Decimal(str(value or "0"))
    except (InvalidOperation, ValueError):
        dec = Decimal(0)
    return str(int((dec * 100).to_integral_value()))


def _as_list(node: Any) -> list:
    if node is None:
        return []
    return node if isinstance(node, list) else [node]


def _nonzero(value: Any) -> bool:
    return bool(value) and str(value) not in ("0", "0.00", "")


def _split_infadic(text: str) -> list[str]:
    """Split ';'-delimited infAdic text into non-empty lines (market convention)."""
    return [p.strip() for p in (text or "").split(";") if p.strip()]


def build_context(
    inf_nfe: dict, prot: dict | None, *, layout: str, canceled: bool,
    tp_emis: str, tp_amb: str, chave: str, site: str = c.DEFAULT_FOOTER_SITE,
    logo: str | None = None,
) -> dict[str, Any]:
    is_contingencia = tp_emis not in c.TP_EMIS_NORMAL_LIKE
    is_fs = tp_emis in c.TP_EMIS_FS_LIKE
    is_epec = tp_emis == c.TP_EMIS_EPEC
    is_homologacao = tp_amb == c.TP_AMB_HOMOLOGACAO
    show_protocolo = not is_fs  # FS/FS-DA not yet authorized

    ide = inf_nfe.get("ide", {})
    emit = inf_nfe.get("emit", {})
    ender_e = emit.get("enderEmit", {})
    dest = inf_nfe.get("dest", {}) or {}
    ender_d = dest.get("enderDest", {})

    produtos = []
    for d in _as_list(inf_nfe.get("det")):
        p = d.get("prod", {})
        imp = d.get("imposto", {}) or {}
        icms_grp = next(iter((imp.get("ICMS") or {}).values()), {}) if imp.get("ICMS") else {}
        if not isinstance(icms_grp, dict):
            icms_grp = {}
        ipi_grp = (imp.get("IPI") or {}).get("IPITrib", {}) or {}
        if not isinstance(ipi_grp, dict):
            ipi_grp = {}
        cst = icms_grp.get("CST") or icms_grp.get("CSOSN") or ""
        orig = icms_grp.get("orig", "")
        produtos.append({
            "cProd": p.get("cProd", ""),
            "xProd": p.get("xProd", ""),
            "NCM": p.get("NCM", ""),
            "CFOP": p.get("CFOP", ""),
            "uCom": p.get("uCom", ""),
            "qCom": p.get("qCom", ""),
            "vUnCom": fmt.money_br(p.get("vUnCom")),
            "vProd": fmt.money_br(p.get("vProd")),
            "vDesc": fmt.money_br(p.get("vDesc")),
            "origCST": f"{orig}/{cst}" if orig and cst else cst,
            "vBC": fmt.money_br(icms_grp.get("vBC")),
            "vICMS": fmt.money_br(icms_grp.get("vICMS")),
            "vIPI": fmt.money_br(ipi_grp.get("vIPI")),
            "pICMS": fmt.pct(icms_grp.get("pICMS")),
            "pIPI": fmt.pct(ipi_grp.get("pIPI")),
            "infAdProd": d.get("infAdProd", ""),
        })

    icms = (inf_nfe.get("total", {}) or {}).get("ICMSTot", {})
    totals = {k: fmt.money_br(icms.get(k)) for k in (
        "vBC", "vICMS", "vBCST", "vST", "vII", "vICMSUFRemet", "vFCPUFDest",
        "vProd", "vFrete", "vSeg", "vDesc", "vOutro", "vIPI", "vICMSUFDest",
        "vNF", "vTotTrib",
    )}

    transp = inf_nfe.get("transp", {}) or {}
    transporta = transp.get("transporta", {}) or {}
    veic = transp.get("veicTransp", {}) or {}
    vol = next(iter(_as_list(transp.get("vol"))), {}) or {}
    transp_ctx = {
        "modFrete_label": c.MOD_FRETE_LABELS.get(transp.get("modFrete", ""), transp.get("modFrete", "")),
        "nome": transporta.get("xNome", ""),
        "doc": fmt.mask_cpf_cnpj(transporta.get("CNPJ") or transporta.get("CPF") or ""),
        "ie": transporta.get("IE", ""),
        "ender": transporta.get("xEnder", ""),
        "mun": transporta.get("xMun", ""),
        "uf": transporta.get("UF", ""),
        "placa": veic.get("placa", ""),
        "placa_uf": veic.get("UF", ""),
        "qVol": vol.get("qVol", ""),
        "esp": vol.get("esp", ""),
        "marca": vol.get("marca", ""),
        "pesoB": vol.get("pesoB", ""),
        "pesoL": vol.get("pesoL", ""),
    }

    cobr = inf_nfe.get("cobr", {}) or {}
    fat = cobr.get("fat", {}) or {}
    duplicatas = [{
        "nDup": dp.get("nDup", ""),
        "dVenc": dp.get("dVenc", ""),
        "vDup": fmt.money_br(dp.get("vDup")),
    } for dp in _as_list(cobr.get("dup"))]

    infadic = inf_nfe.get("infAdic", {}) or {}

    protocolo = None
    if show_protocolo and prot:
        ip = prot.get("infProt", {})
        protocolo = {"nProt": ip.get("nProt", ""), "dhRecbto": fmt.dt_local(ip.get("dhRecbto"))}

    dados_nfe_barcode = ""
    dados_nfe_code_str = ""
    if is_fs:
        dia = (ide.get("dhEmi", "") or "")[8:10] or "01"
        cuf = (chave or "")[0:2]
        doc_dest = dest.get("CNPJ") or dest.get("CPF") or "0"
        dados_nfe_code_str = dados_nfe_code(
            cuf=cuf, tp_emis=tp_emis, doc=doc_dest, vnf=_centavos(icms.get("vNF")),
            icms_proprio=_nonzero(icms.get("vICMS")),
            icms_st=_nonzero(icms.get("vST")),
            dia_emissao=dia,
        )
        dados_nfe_barcode = code128c_data_uri(dados_nfe_code_str)

    return {
        "layout": layout,
        "emit": {
            "nome": emit.get("xNome", ""),
            "fantasia": emit.get("xFant", ""),
            "cnpj": fmt.mask_cnpj(emit.get("CNPJ", "")),
            "ie": emit.get("IE", ""),
            "iest": emit.get("IEST", ""),
            "im": emit.get("IM", ""),
            "endereco": _endereco(ender_e),
            "cep": fmt.mask_cep(ender_e.get("CEP", "")),
            "mun": ender_e.get("xMun", ""),
            "uf": ender_e.get("UF", ""),
            "fone": ender_e.get("fone", ""),
            "logo_b64": logo,
        },
        "dest": {
            "nome": dest.get("xNome", ""),
            "doc": fmt.mask_cpf_cnpj(dest.get("CNPJ") or dest.get("CPF") or ""),
            "ie": dest.get("IE", ""),
            "endereco": _endereco(ender_d),
            "cep": fmt.mask_cep(ender_d.get("CEP", "")),
            "mun": ender_d.get("xMun", ""),
            "uf": ender_d.get("UF", ""),
            "fone": ender_d.get("fone", ""),
        },
        "ide": {
            "natOp": ide.get("natOp", ""),
            "tpNF": ide.get("tpNF", ""),
            "tpNF_label": c.TP_NF_LABELS.get(ide.get("tpNF", ""), ""),
            "nNF": fmt.num_nf(ide.get("nNF", "")),
            "serie": ide.get("serie", ""),
            "dhEmi": fmt.dt_local(ide.get("dhEmi")),
            "dhEmi_data": fmt.date_br(ide.get("dhEmi")),
            "dhSaiEnt": fmt.dt_local(ide.get("dhSaiEnt")),
            "dhSaiEnt_data": fmt.date_br(ide.get("dhSaiEnt")),
            "dhSaiEnt_hora": fmt.time_br(ide.get("dhSaiEnt")),
        },
        "produtos": produtos,
        "totals": totals,
        "transp": transp_ctx,
        "fatura": {
            "nFat": fat.get("nFat", ""),
            "vOrig": fmt.money_br(fat.get("vOrig")),
            "vLiq": fmt.money_br(fat.get("vLiq")),
        },
        "duplicatas": duplicatas,
        "chave_fmt": fmt.chave_blocks(chave),
        "chave_raw": "".join(filter(str.isdigit, chave or "")),
        "chave_barcode": code128c_data_uri(chave),
        "protocolo": protocolo,
        "protocolo_label": c.TEXT_PROTOCOLO_EPEC if is_epec else c.TEXT_PROTOCOLO,
        "show_protocolo": show_protocolo,
        "is_contingencia": is_contingencia,
        "is_fs": is_fs,
        "is_epec": is_epec,
        "is_homologacao": is_homologacao,
        "is_cancelada": canceled,
        "dados_nfe_code": dados_nfe_code_str,
        "dados_nfe_barcode": dados_nfe_barcode,
        "msg_fiscal": _split_infadic(infadic.get("infAdFisco", "")),
        "msg_contribuinte": _split_infadic(infadic.get("infCpl", "")),
        "site": site,
        "gerado_em": fmt.now_br(),
        "text": {
            "gerado_por": c.TEXT_GERADO_POR,
            "danfe": c.TEXT_DANFE,
            "danfe_desc": c.TEXT_DANFE_DESC,
            "simplificado": c.TEXT_DANFE_SIMPLIFICADO,
            "etiqueta": c.TEXT_DANFE_ETIQUETA,
            "homologacao": c.TEXT_NFE_HOMOLOGACAO,
            "contingencia": c.TEXT_NFE_CONTINGENCIA,
            "consulta": c.TEXT_CONSULTA_NFE,
            "dados_nfe": c.TEXT_DADOS_NFE,
            "cancelada": c.TEXT_WATERMARK_CANCELADA,
        },
    }


def _endereco(ender: dict) -> str:
    parts = [
        ender.get("xLgr", ""), ender.get("nro", ""), ender.get("xCpl", ""),
        ender.get("xBairro", ""),
    ]
    return ", ".join(p for p in parts if p)
