"""Build processed DFe XML documents (nfeProc, cteProc, mdfeProc, procEvento*)."""

from __future__ import annotations

import copy
import logging
from typing import Any

from lxml import etree

logger = logging.getLogger(__name__)

_NS: dict[str, str] = {
    "nfe": "http://www.portalfiscal.inf.br/nfe",
    "nfce": "http://www.portalfiscal.inf.br/nfe",
    "cte": "http://www.portalfiscal.inf.br/cte",
    "mdfe": "http://www.portalfiscal.inf.br/mdfe",
}

# service_name: (proc_root_tag, doc_local_tag, prot_local_tag, versao)
_EMISSION: dict[str, tuple[str, str, str, str]] = {
    "NFeAutorizacao": ("nfeProc", "NFe", "protNFe", "4.00"),
    "NfceAutorizacao": ("nfeProc", "NFe", "protNFe", "4.00"),
    "CTeRecepcaoSinc": ("cteProc", "CTe", "protCTe", "4.00"),
    "CTeRecepcaoOS": ("cteOSProc", "CTeOS", "protCTe", "4.00"),
    "CTeRecepcaoGTVe": ("GTVeProc", "GTVe", "protCTe", "4.00"),
    "CTeRecepcaoSimp": ("cteSimpProc", "CTeSimp", "protCTe", "4.00"),
    "MDFeRecepcaoSinc": ("mdfeProc", "MDFe", "protMDFe", "3.00"),
}

# service_name: (proc_root_tag, versao)
# event elements are always renamed to 'evento'/'retEvento' in the proc document
_EVENT: dict[str, tuple[str, str]] = {
    "RecepcaoEvento": ("procEventoNFe", "1.00", "evento", "retEvento"),
    "CTeRecepcaoEvento": ("procEventoCTe", "4.00" "eventoCTe", "retEventoCTe"),
    "MDFeRecepcaoEvento": ("procEventoMDFe", "3.00", "eventoMDFe", "retEventoMDFe"),
}


def build_processed_xml(
        doc_type: str,
        service: str,
        request_xml: bytes,
        response_xml: bytes,
) -> str | None:
    """Return the processed XML string, or None if the service has no processed form."""
    try:
        ns = _NS.get(doc_type)
        if not ns:
            return None
        if service in _EMISSION:
            return _build_emission(ns, service, request_xml, response_xml)
        if service in _EVENT:
            return _build_events(ns, service, request_xml, response_xml)
    except Exception:
        logger.debug(
            "Could not build processed XML for %s/%s", doc_type, service, exc_info=True
        )
    return None


# ---------------------------------------------------------------------------
# Emission: nfeProc, cteProc, mdfeProc, etc.
# ---------------------------------------------------------------------------


def _build_emission(
        ns: str,
        service: str,
        request_xml: bytes,
        response_xml: bytes,
) -> str | None:
    proc_tag, doc_tag, prot_tag, versao = _EMISSION[service]

    req_root = etree.fromstring(request_xml)
    resp_root = etree.fromstring(response_xml)

    doc_el = _first_by_local(req_root, doc_tag)
    prot_el = _first_by_local(resp_root, prot_tag)

    if doc_el is None or prot_el is None:
        return None

    proc = etree.Element(f"{{{ns}}}{proc_tag}", nsmap={None: ns})
    proc.set("versao", versao)
    proc.append(_deep_copy(doc_el))
    proc.append(_deep_copy(prot_el))

    return etree.tostring(proc, encoding="unicode", xml_declaration=False)


# ---------------------------------------------------------------------------
# Events: procEventoNFe, procEventoCTe, procEventoMDFe
# ---------------------------------------------------------------------------


def _build_events(
        ns: str,
        service: str,
        request_xml: bytes,
        response_xml: bytes,
) -> str | None:
    proc_tag, versao, event_tag, ret_event_tag = _EVENT[service]

    req_root = etree.fromstring(request_xml)
    resp_root = etree.fromstring(response_xml)

    # Events in request may be called 'evento', 'eventoCTe', 'eventoMDFe', etc.
    events = _all_by_local(req_root, event_tag)
    if not events:
        # fallback: whole request root IS the event (single event, no envelope)
        events = [req_root]

    # Response may wrap under retEnvEvento/retEvento or retEvento directly
    ret_events = _all_by_local(resp_root, ret_event_tag)
    if not ret_events:
        return None

    results: list[str] = []
    for ev, ret_ev in zip(events, ret_events):
        proc = etree.Element(f"{{{ns}}}{proc_tag}", nsmap={None: ns})
        proc.set("versao", versao)
        # Always tag child as 'evento' (rename if needed)
        proc.append(_as_local_tag(ev, "evento", ns))
        # Always tag child as 'retEvento' (rename if needed)
        proc.append(_as_local_tag(ret_ev, "retEvento", ns))
        results.append(etree.tostring(proc, encoding="unicode", xml_declaration=False))

    if not results:
        return None
    if len(results) == 1:
        return results[0]
    # Multiple events: JSON list of XML strings
    import json

    return json.dumps(results)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _all_by_local(root: etree.Element, local_tag: str) -> list[etree.Element]:
    """Return all descendants (including root) with the given local tag."""
    if etree.QName(root.tag).localname == local_tag:
        return [root]
    return [el for el in root.iter() if etree.QName(el.tag).localname == local_tag]


def _first_by_local(root: etree.Element, local_tag: str) -> etree.Element | None:
    els = _all_by_local(root, local_tag)
    return els[0] if els else None


def _deep_copy(el: etree.Element) -> etree.Element:
    return copy.deepcopy(el)


def _as_local_tag(el: etree.Element, local_tag: str, ns: str) -> etree.Element:
    """Return a deep copy of el, with its root tag renamed to local_tag in ns."""
    new_el = copy.deepcopy(el)
    if etree.QName(new_el.tag).localname != local_tag:
        new_el.tag = f"{{{ns}}}{local_tag}"
    return new_el
