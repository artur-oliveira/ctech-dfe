"""DAMDFE (MDF-e modelo 58) generation from an authorized MDF-e XML."""

from __future__ import annotations

import base64
from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe import formatters as fmt
from py_dfe.danfe.barcode import code128c_data_uri
from py_dfe.danfe.qr import qr_data_uri
from py_dfe.danfe.render import htmls_to_pdf, render_html
from py_dfe.exceptions import DANFE_INVALID_XML, DANFE_UNSUPPORTED_MODEL, DFeError
from py_dfe.xmlops.builder import parse_xml_bytes


def generate_damdfe(payload: dict[str, Any]) -> dict[str, Any]:
    """Render a DAMDFE (mod 58) from an authorized MDF-e XML payload."""
    xml = payload.get("xml")
    if not xml:
        raise DFeError(422, DANFE_INVALID_XML, "Missing 'xml' in body")
    layout = payload.get("layout", c.DEFAULT_DAMDFE_LAYOUT)
    if layout not in c.VALID_DAMDFE_LAYOUTS:
        layout = c.DEFAULT_DAMDFE_LAYOUT
    canceled = bool(payload.get("canceled", False))
    site = payload.get("site") or c.DEFAULT_FOOTER_SITE

    inf_mdfe, supl, prot, tp_emis, tp_amb, chave = _extract_roots(xml)
    context = build_context(
        inf_mdfe, supl, prot, layout=layout, canceled=canceled,
        tp_emis=tp_emis, tp_amb=tp_amb, chave=chave, site=site,
    )
    template = c.DAMDFE_TEMPLATES[layout]
    html = render_html(template, {"ctx": context})
    # A4 layouts: honour @page and paginate naturally (long document lists).
    pdf = htmls_to_pdf([html], fit_height=False)
    return {"pdf_b64": base64.b64encode(pdf).decode("ascii"), "html": [html]}


def _extract_roots(xml: str) -> tuple[dict, dict, dict | None, str, str, str]:
    try:
        parsed = parse_xml_bytes(xml.encode("utf-8") if isinstance(xml, str) else xml)
    except etree.XMLSyntaxError as exc:
        raise DFeError(422, DANFE_INVALID_XML, f"Malformed XML: {exc}") from exc

    root = next(iter(parsed.values()))
    # Accept either <mdfeProc> (with MDFe + protMDFe) or a bare <MDFe>.
    mdfe = root.get("MDFe", root) if isinstance(root, dict) else {}
    prot = root.get("protMDFe") if isinstance(root, dict) else None
    inf_mdfe = mdfe.get("infMDFe") if isinstance(mdfe, dict) else None
    if not isinstance(inf_mdfe, dict):
        raise DFeError(422, DANFE_INVALID_XML, "infMDFe not found")
    supl = mdfe.get("infMDFeSupl", {}) if isinstance(mdfe, dict) else {}

    ide = inf_mdfe.get("ide", {})
    if ide.get("mod") != c.MODELO_MDFE:
        raise DFeError(
            422, DANFE_UNSUPPORTED_MODEL,
            f"DAMDFE requires model 58, got {ide.get('mod')!r}",
        )
    tp_emis = ide.get("tpEmis", c.TP_EMIS_MDFE_NORMAL)
    tp_amb = ide.get("tpAmb", c.TP_AMB_PRODUCAO)
    chave = _chave(inf_mdfe, prot)
    return inf_mdfe, supl, prot, tp_emis, tp_amb, chave


def _chave(inf_mdfe: dict, prot: dict | None) -> str:
    if prot:
        ch = (prot.get("infProt") or {}).get("chMDFe")
        if ch:
            return ch
    return (inf_mdfe.get("@Id") or "").replace("MDFe", "")


def _as_list(node: Any) -> list:
    if node is None:
        return []
    return node if isinstance(node, list) else [node]


def _endereco(ender: dict) -> str:
    parts = [
        ender.get("xLgr", ""), ender.get("nro", ""), ender.get("xCpl", ""),
        ender.get("xBairro", ""),
    ]
    return ", ".join(p for p in parts if p)


def _modal_context(inf_mdfe: dict, modal: str) -> dict[str, Any]:
    """Modal-specific block (one of rodoviário / aéreo / aquaviário / ferroviário)."""
    info = inf_mdfe.get("infModal", {}) or {}
    ctx: dict[str, Any] = {
        "is_rodo": modal == c.MODAL_RODOVIARIO,
        "is_aereo": modal == c.MODAL_AEREO,
        "is_aqua": modal == c.MODAL_AQUAVIARIO,
        "is_ferrov": modal == c.MODAL_FERROVIARIO,
    }

    if modal == c.MODAL_RODOVIARIO:
        rodo = info.get("rodo", {}) or {}
        antt = rodo.get("infANTT", {}) or {}
        ciots = [ci.get("CIOT", "") for ci in _as_list(antt.get("infCIOT"))]
        veic = rodo.get("veicTracao", {}) or {}
        ctx.update({
            "rntrc": antt.get("RNTRC", ""),
            "ciot": ", ".join(filter(None, ciots)),
            "veic": {
                "placa": veic.get("placa", ""),
                "uf": veic.get("UF", ""),
                "renavam": veic.get("RENAVAM", ""),
                "tara": veic.get("tara", ""),
                "capkg": veic.get("capKG", ""),
            },
            "condutores": [
                {"nome": cd.get("xNome", ""), "cpf": fmt.mask_cpf_cnpj(cd.get("CPF", ""))}
                for cd in _as_list(veic.get("condutor"))
            ],
            "reboques": [
                {
                    "placa": rb.get("placa", ""),
                    "uf": rb.get("UF", ""),
                    "renavam": rb.get("RENAVAM", ""),
                    "tara": rb.get("tara", ""),
                }
                for rb in _as_list(rodo.get("veicReboque"))
            ],
        })
    elif modal == c.MODAL_AEREO:
        aereo = info.get("aereo", {}) or {}
        ctx["aereo"] = {
            "nac": aereo.get("nac", ""),
            "matr": aereo.get("matr", ""),
            "nvoo": aereo.get("nVoo", ""),
            "caeremb": aereo.get("cAerEmb", ""),
            "caerdes": aereo.get("cAerDes", ""),
            "dvoo": aereo.get("dVoo", ""),
        }
    elif modal == c.MODAL_AQUAVIARIO:
        aqua = info.get("aquav", {}) or {}
        ctx["aqua"] = {
            "irin": aqua.get("irin", ""),
            "tpemb": aqua.get("tpEmb", ""),
            "cembar": aqua.get("cEmbar", ""),
            "xembar": aqua.get("xEmbar", ""),
            "nviag": aqua.get("nViag", ""),
            "cprtemb": aqua.get("cPrtEmb", ""),
            "cprtdest": aqua.get("cPrtDest", ""),
        }
    elif modal == c.MODAL_FERROVIARIO:
        ferr = info.get("ferrov", {}) or {}
        comp = ferr.get("trem", {}) or {}
        ctx["ferrov"] = {
            "xpref": comp.get("xPref", ""),
            "dhtrem": fmt.dt_local(comp.get("dhTrem")),
            "xori": comp.get("xOri", ""),
            "xdest": comp.get("xDest", ""),
            "qvag": comp.get("qVag", ""),
            "vagoes": [
                {"serie": vg.get("serie", ""), "nvag": vg.get("nVag", "")}
                for vg in _as_list(ferr.get("vag"))
            ],
        }
    return ctx


def _municipios(inf_mdfe: dict) -> list[dict[str, Any]]:
    """One block per discharge municipality, each with its document keys."""
    inf_doc = inf_mdfe.get("infDoc", {}) or {}
    blocks = []
    for md in _as_list(inf_doc.get("infMunDescarga")):
        docs = []
        for nf in _as_list(md.get("infNFe")):
            ch = nf.get("chNFe", "")
            docs.append({"tipo": c.DOC_TIPO_NFE, "chave": ch, "chave_fmt": fmt.chave_blocks(ch)})
        for ct in _as_list(md.get("infCTe")):
            ch = ct.get("chCTe", "")
            docs.append({"tipo": c.DOC_TIPO_CTE, "chave": ch, "chave_fmt": fmt.chave_blocks(ch)})
        for tr in _as_list(md.get("infMDFeTransp")):
            ch = tr.get("chMDFe", "")
            docs.append({"tipo": c.DOC_TIPO_MDFE, "chave": ch, "chave_fmt": fmt.chave_blocks(ch)})
        blocks.append({
            "mun": md.get("xMunDescarga", ""),
            "docs": docs,
            "count": len(docs),
        })
    return blocks


def _seguros(inf_mdfe: dict) -> list[dict[str, Any]]:
    seguros = []
    for sg in _as_list(inf_mdfe.get("seg")):
        resp = sg.get("infResp", {}) or {}
        info = sg.get("infSeg", {}) or {}
        seguros.append({
            "resp": resp.get("respSeg", ""),
            "resp_doc": fmt.mask_cpf_cnpj(resp.get("CNPJ") or resp.get("CPF") or ""),
            "nome": info.get("xSeg", ""),
            "cnpj": fmt.mask_cnpj(info.get("CNPJ", "")) if info.get("CNPJ") else "",
            "apol": sg.get("nApol", ""),
            "averbacoes": ", ".join(filter(None, _as_list(sg.get("nAver")))),
        })
    return seguros


def build_context(
    inf_mdfe: dict, supl: dict, prot: dict | None, *, layout: str, canceled: bool,
    tp_emis: str, tp_amb: str, chave: str, site: str = c.DEFAULT_FOOTER_SITE,
) -> dict[str, Any]:
    is_contingencia = tp_emis == c.TP_EMIS_MDFE_CONTINGENCIA
    is_homologacao = tp_amb == c.TP_AMB_HOMOLOGACAO

    ide = inf_mdfe.get("ide", {}) or {}
    emit = inf_mdfe.get("emit", {}) or {}
    ender_e = emit.get("enderEmit", {}) or {}
    modal = ide.get("modal", c.MODAL_RODOVIARIO)

    carrega = [
        f"{mc.get('xMunCarrega', '')}"
        for mc in _as_list(ide.get("infMunCarrega"))
    ]
    percurso = [p.get("UFPer", "") for p in _as_list(ide.get("infPercurso"))]

    prod_pred = inf_mdfe.get("prodPred", {}) or {}
    tot = inf_mdfe.get("tot", {}) or {}
    infadic = inf_mdfe.get("infAdic", {}) or {}

    protocolo = None
    if prot and not is_contingencia:
        ip = prot.get("infProt", {}) or {}
        protocolo = {
            "nProt": ip.get("nProt", ""),
            "dhRecbto": fmt.dt_local(ip.get("dhRecbto")),
        }

    return {
        "layout": layout,
        "modal": modal,
        "modal_label": c.MODAL_LABELS.get(modal, ""),
        "emit": {
            "nome": emit.get("xNome", ""),
            "fantasia": emit.get("xFant", ""),
            "doc": fmt.mask_cpf_cnpj(emit.get("CNPJ") or emit.get("CPF") or ""),
            "ie": emit.get("IE", ""),
            "endereco": _endereco(ender_e),
            "mun": ender_e.get("xMun", ""),
            "uf": ender_e.get("UF", ""),
            "cep": fmt.mask_cep(ender_e.get("CEP", "")),
            "fone": ender_e.get("fone", ""),
        },
        "ide": {
            "serie": ide.get("serie", ""),
            "nMDF": fmt.num_nf(ide.get("nMDF", "")),
            "dhEmi": fmt.dt_local(ide.get("dhEmi")),
            "ufIni": ide.get("UFIni", ""),
            "ufFim": ide.get("UFFim", ""),
            "tpEmit_label": c.TP_EMIT_MDFE_LABELS.get(ide.get("tpEmit", ""), ""),
            "tpTransp_label": c.TP_TRANSP_MDFE_LABELS.get(ide.get("tpTransp", ""), ""),
        },
        "carrega": carrega,
        "percurso": " ".join(filter(None, percurso)),
        "modal_info": _modal_context(inf_mdfe, modal),
        "prodPred": {
            "tpCarga_label": c.TP_CARGA_LABELS.get(prod_pred.get("tpCarga", ""), ""),
            "xProd": prod_pred.get("xProd", ""),
            "ncm": prod_pred.get("NCM", ""),
        },
        "tot": {
            "qCTe": tot.get("qCTe", "0"),
            "qNFe": tot.get("qNFe", "0"),
            "qMDFe": tot.get("qMDFe", "0"),
            "vCarga": fmt.money_br(tot.get("vCarga")),
            "qCarga": tot.get("qCarga", ""),
            "cUnid_label": c.C_UNID_LABELS.get(tot.get("cUnid", ""), ""),
        },
        "municipios": _municipios(inf_mdfe),
        "seguros": _seguros(inf_mdfe),
        "lacres": [
            lc.get("nLacre", "") for lc in _as_list(inf_mdfe.get("lacres"))
            if lc.get("nLacre")
        ],
        "chave_fmt": fmt.chave_blocks(chave),
        "chave_raw": "".join(filter(str.isdigit, chave or "")),
        "chave_barcode": code128c_data_uri(chave),
        "qr_uri": qr_data_uri(supl.get("qrCodMDFe", "")) if supl.get("qrCodMDFe") else "",
        "protocolo": protocolo,
        "is_contingencia": is_contingencia,
        "is_homologacao": is_homologacao,
        "is_cancelada": canceled,
        "msg_fiscal": infadic.get("infAdFisco", ""),
        "msg_contribuinte": infadic.get("infCpl", ""),
        "site": site,
        "gerado_em": fmt.now_br(),
        "text": {
            "gerado_por": c.TEXT_GERADO_POR,
            "damdfe": c.TEXT_DAMDFE,
            "damdfe_desc": c.TEXT_DAMDFE_DESC,
            "homologacao": c.TEXT_MDFE_HOMOLOGACAO,
            "contingencia": c.TEXT_MDFE_CONTINGENCIA,
            "protocolo": c.TEXT_PROTOCOLO_MDFE,
            "consulta": c.TEXT_DAMDFE_CONSULTA,
            "cancelada": c.TEXT_WATERMARK_CANCELADA,
        },
    }
