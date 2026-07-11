"""XSD schema validation for fiscal documents."""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
from typing import Any

from lxml import etree

from py_dfe.exceptions import XMLValidationError

_SCHEMA_ROOT = Path(__file__).resolve().parents[2] / "schemas" / "xsds"

_XSD_MAP: dict[tuple[str, str], str] = {

    ("nfe", "NFeAutorizacao"): "PL_010c_NT2022_002v1.30/enviNFe_v4.00.xsd",
    ("nfe", "RecepcaoEvento"): "PL_010c_NT2022_002v1.30/envEvento_v1.00.xsd",

    ("nfce", "NFeAutorizacao"): "PL_010c_NT2022_002v1.30/enviNFe_v4.00.xsd",
    ("nfce", "RecepcaoEvento"): "PL_010c_NT2022_002v1.30/envEvento_v1.00.xsd",

    ("cte", "CTeRecepcaoSinc"): "PL_CTe_400_NT2026.001/cte_v4.00.xsd",
    ("cte", "CTeRecepcaoOS"): "PL_CTe_400_NT2026.001/cteOS_v4.00.xsd",

    ("cte", "CTeRecepcaoEvento"): "PL_CTe_400_NT2026.001/eventoCTe_v4.00.xsd",

    ("mdfe", "MDFeRecepcaoSinc"): "PL_MDFe_300b_NT012025_1.03/mdfe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento"): "PL_MDFe_300b_NT012025_1.03/eventoMDFe_v3.00.xsd",
}

_DET_EVENTO_XSD_MAP: dict[tuple[str, str, str], str] = {
    ("nfe", "RecepcaoEvento", "110110"): "PL_010c_NT2022_002v1.30/e110110_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "110111"): "PL_010c_NT2022_002v1.30/e110111_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "110140"): "PL_010c_NT2022_002v1.30/e110140_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "210200"): "PL_010c_NT2022_002v1.30/e210200_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "210210"): "PL_010c_NT2022_002v1.30/e210210_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "210220"): "PL_010c_NT2022_002v1.30/e210220_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "210240"): "PL_010c_NT2022_002v1.30/e210240_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "111500"): "PL_010c_NT2022_002v1.30/e111500_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "111501"): "PL_010c_NT2022_002v1.30/e111501_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "111502"): "PL_010c_NT2022_002v1.30/e111502_v1.00.xsd",
    ("nfe", "RecepcaoEvento", "111503"): "PL_010c_NT2022_002v1.30/e111503_v1.00.xsd",

    ("nfce", "RecepcaoEvento", "110111"): "PL_010c_NT2022_002v1.30/e110111_v1.00.xsd",
    ("nfce", "RecepcaoEvento", "110112"): "PL_010c_NT2022_002v1.30/e110112_v1.00.xsd",
    ("nfce", "RecepcaoEvento", "110140"): "PL_010c_NT2022_002v1.30/e110140_v1.00.xsd",

    ("cte", "CTeRecepcaoEvento", "110110"): "PL_CTe_400_NT2026.001/evCCeCTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110111"): "PL_CTe_400_NT2026.001/evCancCTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110113"): "PL_CTe_400_NT2026.001/evEPECCTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110160"): "PL_CTe_400_NT2026.001/evRegMultimodal_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110170"): "PL_CTe_400_NT2026.001/evGTV_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110180"): "PL_CTe_400_NT2026.001/evCECTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110181"): "PL_CTe_400_NT2026.001/evCancCECTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110190"): "PL_CTe_400_NT2026.001/evIECTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110191"): "PL_CTe_400_NT2026.001/evCancIECTe_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110300"): "PL_CTe_400_NT2026.001/evVincPgto_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "110301"): "PL_CTe_400_NT2026.001/evCancVincPgto_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "610110"): "PL_CTe_400_NT2026.001/evPrestDesacordo_v4.00.xsd",
    ("cte", "CTeRecepcaoEvento", "610111"): "PL_CTe_400_NT2026.001/evCancPrestDesacordo_v4.00.xsd",

    ("mdfe", "MDFeRecepcaoEvento", "110111"): "PL_MDFe_300b_NT012025_1.03/evCancMDFe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento", "110112"): "PL_MDFe_300b_NT012025_1.03/evEncMDFe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento", "110114"): "PL_MDFe_300b_NT012025_1.03/evIncCondutorMDFe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento", "110115"): "PL_MDFe_300b_NT012025_1.03/evInclusaoDFeMDFe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento", "110116"): "PL_MDFe_300b_NT012025_1.03/evPagtoOperMDFe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento", "110117"): "PL_MDFe_300b_NT012025_1.03/evConfirmaServMDFe_v3.00.xsd",
    ("mdfe", "MDFeRecepcaoEvento", "110118"): "PL_MDFe_300b_NT012025_1.03/evAlteracaoPagtoServMDFe_v3.00.xsd",
}

_MODAL_XSD_MAP: dict[tuple[str, str], str] = {

    ("cte", "rodo"): "PL_CTe_400_NT2026.001/cteModalRodoviario_v4.00.xsd",
    ("cte", "aereo"): "PL_CTe_400_NT2026.001/cteModalAereo_v4.00.xsd",
    ("cte", "aquav"): "PL_CTe_400_NT2026.001/cteModalAquaviario_v4.00.xsd",
    ("cte", "duto"): "PL_CTe_400_NT2026.001/cteModalDutoviario_v4.00.xsd",
    ("cte", "ferrov"): "PL_CTe_400_NT2026.001/cteModalFerroviario_v4.00.xsd",
    ("cte", "rodoOS"): "PL_CTe_400_NT2026.001/cteModalRodoviarioOS_v4.00.xsd",
    ("cte", "multimodal"): "PL_CTe_400_NT2026.001/cteMultiModal_v4.00.xsd",

    ("mdfe", "rodo"): "PL_MDFe_300b_NT012025_1.03/mdfeModalRodoviario_v3.00.xsd",
    ("mdfe", "aereo"): "PL_MDFe_300b_NT012025_1.03/mdfeModalAereo_v3.00.xsd",
    ("mdfe", "aquav"): "PL_MDFe_300b_NT012025_1.03/mdfeModalAquaviario_v3.00.xsd",
    ("mdfe", "ferrov"): "PL_MDFe_300b_NT012025_1.03/mdfeModalFerroviario_v3.00.xsd",
}

_DET_EVENTO_SERVICES: frozenset[str] = frozenset({"RecepcaoEvento", "CTeRecepcaoEvento", "MDFeRecepcaoEvento"})

_MODAL_SERVICES: frozenset[str] = frozenset({"CTeRecepcaoSinc", "CTeRecepcaoOS", "MDFeRecepcaoSinc"})

_ALL_EVENTO_SERVICES: frozenset[str] = _DET_EVENTO_SERVICES


@lru_cache(maxsize=32)
def _load_schema(xsd_path: Path) -> etree.XMLSchema:
    schema_doc = etree.parse(str(xsd_path))
    return etree.XMLSchema(schema_doc)


def _run_validation(element: Any, rel_path: str, label: str) -> None:
    xsd_path = _SCHEMA_ROOT / rel_path
    if not xsd_path.exists():
        return
    schema = _load_schema(xsd_path)
    if not schema.validate(element):
        errors = [str(e) for e in schema.error_log]
        raise XMLValidationError(f"XSD validation failed for {label}", errors=errors)


def _extract_tp_evento(doc: Any) -> str | None:
    els = doc.findall(".//{*}tpEvento")
    return els[0].text.strip() if els and els[0].text else None


def _extract_det_evento_child(doc: Any, use_det: bool = False) -> Any | None:
    det_els = doc.findall(".//{*}detEvento")
    if not det_els:
        return None

    if use_det:
        return det_els[0]
    children = list(det_els[0])
    return children[0] if children else None


def _extract_modal_child(doc: Any) -> tuple[Any, str] | tuple[None, None]:
    modal_els = doc.findall(".//{*}infModal")
    if not modal_els:
        return None, None
    children = list(modal_els[0])
    if not children:
        return None, None
    el = children[0]
    return el, etree.QName(el.tag).localname


def validate(xml_bytes: bytes, doc_type: str, service: str) -> None:
    """Validate *xml_bytes* against the XSD(s) for the given doc_type + service.

    Multiple validations may run per call:
    - Main document / event envelope via _XSD_MAP or _EVENT_XSD_MAP.
    - CT-e / MDF-e events: detEvento child via _DET_EVENTO_XSD_MAP.
    - CT-e / MDF-e documents: infModal child via _MODAL_XSD_MAP.

    Raises XMLValidationError on failure. Silent when no schema is mapped.
    """
    doc = etree.fromstring(xml_bytes)
    tp_evento = _extract_tp_evento(doc) if service in _ALL_EVENTO_SERVICES else None

    main_rel = _XSD_MAP.get((doc_type, service))

    if main_rel:
        _run_validation(doc, main_rel, f"{doc_type}/{service}")

    if service in _DET_EVENTO_SERVICES and tp_evento:
        det_rel = _DET_EVENTO_XSD_MAP.get((doc_type, service, tp_evento))
        if det_rel:
            det_child = _extract_det_evento_child(doc, doc_type in ('nfe', 'nfce'))
            if det_child is not None:
                _run_validation(det_child, det_rel, f"{doc_type}/{service}/detEvento/{tp_evento}")

    if service in _MODAL_SERVICES:
        modal_child, modal_name = _extract_modal_child(doc)
        if modal_child is not None and modal_name:
            modal_rel = _MODAL_XSD_MAP.get((doc_type, modal_name))
            if modal_rel:
                _run_validation(modal_child, modal_rel, f"{doc_type}/{service}/infModal/{modal_name}")
