"""GerarDanfe dispatcher — routes by NF-e model (mod 55 / 65)."""

from __future__ import annotations

from typing import Any

from lxml import etree

from py_dfe.constants import danfe as c
from py_dfe.danfe.danfce import generate_danfce
from py_dfe.danfe.nfe55 import generate_danfe_nfe
from py_dfe.exceptions import DANFE_INVALID_XML, DANFE_UNSUPPORTED_MODEL, DFeError
from py_dfe.xmlops.builder import parse_xml_bytes


def generate_danfe(payload: dict[str, Any]) -> dict[str, Any]:
    """Render the correct auxiliary document for the XML's model code."""
    xml = payload.get("xml")
    if not xml:
        raise DFeError(422, DANFE_INVALID_XML, "Missing 'xml' in body")
    mod = _peek_mod(xml)
    if mod == c.MODELO_NFCE:
        return generate_danfce(payload)
    if mod == c.MODELO_NFE:
        return generate_danfe_nfe(payload)
    raise DFeError(
        422, DANFE_UNSUPPORTED_MODEL,
        f"DANFE supports models 55 and 65, got {mod!r}",
    )


def _peek_mod(xml: str) -> str | None:
    try:
        parsed = parse_xml_bytes(xml.encode("utf-8") if isinstance(xml, str) else xml)
    except etree.XMLSyntaxError as exc:
        raise DFeError(422, DANFE_INVALID_XML, f"Malformed XML: {exc}") from exc
    root = next(iter(parsed.values()))
    nfe = root.get("NFe", root) if isinstance(root, dict) else {}
    inf_nfe = nfe.get("infNFe", {}) if isinstance(nfe, dict) else {}
    if not isinstance(inf_nfe, dict):
        raise DFeError(422, DANFE_INVALID_XML, "infNFe not found")
    return (inf_nfe.get("ide", {}) or {}).get("mod")
