"""JSON ↔ XML conversion utilities for fiscal documents.

Convention for JSON → XML:
  - Keys starting with "@" become XML attributes.
  - A key "#text" becomes the element text content.
  - All other keys become child elements.
  - A list value creates multiple sibling elements with the same tag.
"""

from __future__ import annotations

from lxml import etree

from py_dfe.constants.types import XMLElement
from py_dfe.exceptions import XMLBuildError
from py_dfe.xmlops.xsd_order import XSD_ORDER


def dict_to_xml(data: dict, parent: XMLElement | None = None) -> XMLElement:
    """Recursively convert a dict to an lxml Element tree.

    The dict must have exactly ONE top-level key (the root element name).
    """
    if len(data) != 1:
        raise XMLBuildError(
            f"Root dict must have exactly one key, got: {list(data.keys())}"
        )
    root_tag, root_value = next(iter(data.items()))
    root = _build_element(root_tag, root_value, parent)
    return root


def create_element(tag: str, attribs: dict = None, nsmap=None) -> XMLElement:
    return etree.Element(tag, attrib=attribs, nsmap=nsmap)  # noqa


def create_subelement(parent, tag, attribs=None, nsmap=None) -> XMLElement:
    return etree.SubElement(parent, tag, attrib=attribs, nsmap=nsmap)


def _build_element(
        tag: str,
        value: dict | list | str | int | float | None,
        parent: XMLElement | None,
        inherited_ns: str | None = None,
        parent_tag: str | None = None,
) -> XMLElement:
    local_ns = inherited_ns
    attribs: dict[str, str] = {}
    if isinstance(value, dict):
        for k, v in value.items():
            if k == "@xmlns":
                local_ns = str(v)
            elif k.startswith("@"):
                attribs[k[1:]] = str(v)

    qualified_tag = f"{{{local_ns}}}{tag}" if local_ns else tag
    if parent is not None:
        element = create_subelement(parent, qualified_tag, attribs)
    else:
        nsmap = {None: local_ns} if local_ns else {}
        element = create_element(qualified_tag, attribs, nsmap)

    if isinstance(value, dict):
        # Lookup: try the most-specific ancestor-scoped key first, narrowing the
        # path one ancestor at a time, then fall back to the plain "tag".
        # parent_tag holds the ":"-joined ancestor path (e.g. "infMDFe:emit").
        child_order = None
        if parent_tag:
            parts = parent_tag.split(":")
            for i in range(len(parts)):
                child_order = XSD_ORDER.get(f"{':'.join(parts[i:])}:{tag}")
                if child_order:
                    break
        child_order = child_order or XSD_ORDER.get(tag)

        if child_order:
            rank = {name: i for i, name in enumerate(child_order)}
            children = sorted(
                ((k, v) for k, v in value.items() if not k.startswith("@") and k != "#text"),
                key=lambda kv: rank.get(kv[0], len(child_order)),
            )
        else:
            children = [(k, v) for k, v in value.items() if not k.startswith("@") and k != "#text"]

        if "#text" in value:
            element.text = str(value["#text"])
        child_path = f"{parent_tag}:{tag}" if parent_tag else tag
        for k, v in children:
            if isinstance(v, list):
                for item in v:
                    _build_element(k, item, element, local_ns, parent_tag=child_path)
            else:
                _build_element(k, v, element, local_ns, parent_tag=child_path)

    elif isinstance(value, (str, int, float)):
        element.text = str(value)

    return element


def to_xml_bytes(data: dict, xml_declaration: bool = False) -> bytes:
    """Convert a dict payload to UTF-8 XML bytes."""
    try:
        root = dict_to_xml(data)
        return etree.tostring(
            root,
            pretty_print=False,  # noqa
            xml_declaration=xml_declaration,
            encoding="UTF-8" if xml_declaration else None,
        )
    except XMLBuildError:
        raise
    except Exception as exc:
        raise XMLBuildError(f"Failed to build XML: {exc}") from exc


def xml_to_dict(element: XMLElement) -> dict | str:
    """Recursively convert an lxml Element to a plain dict.

    Returns a string when the element has text content and no children.
    """

    attribs = {f"@{k}": v for k, v in element.attrib.items()}
    children: dict[str, list] = {}

    for child in element:
        child_tag = etree.QName(child.tag).localname
        child_value = xml_to_dict(child)
        children.setdefault(child_tag, []).append(child_value)

    flat_children = {
        k: v[0] if len(v) == 1 else v for k, v in children.items()
    }

    text = (element.text or "").strip()

    if not attribs and not flat_children:
        return text

    result: dict = {**attribs, **flat_children}
    if text:
        result["#text"] = text
    return result


def parse_xml_bytes(xml_bytes: bytes) -> dict:
    """Parse XML bytes to a dict with the root element name as the key."""
    root = etree.fromstring(xml_bytes)
    tag = etree.QName(root.tag).localname
    return {tag: xml_to_dict(root)}
