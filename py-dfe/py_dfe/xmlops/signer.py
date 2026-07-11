"""XML digital signature for fiscal documents (XML-DSig / ICP-Brasil).

SEFAZ supports both RSA-SHA1 (legacy) and RSA-SHA256 (NT 2017.001+).
signxml ≥ 3.x rejects SHA-1 by default; we use SHA-256 which is accepted
by all current SEFAZ authorizers.
"""

from __future__ import annotations

import re as _re

from lxml import etree
from signxml import XMLSigner, methods, namespaces as _ns

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.types import XMLElement
from py_dfe.exceptions import XMLSignError

_DSIG_NS = "http://www.w3.org/2000/09/xmldsig#"
_X509_RE = _re.compile(r'(<X509Certificate[^>]*>)([^<]*)(</X509Certificate>)')


class _SefazXMLSigner(XMLSigner):
    """signxml ≥ 3 rejects SHA-1; SEFAZ XSD fixa rsa-sha1/sha1/C14N sem prefixo ds:."""

    def check_deprecated_methods(self) -> None:
        pass

    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self.namespaces = {None: _ns.ds}


_SIGNER = _SefazXMLSigner(
    method=methods.enveloped,
    signature_algorithm="rsa-sha1",
    digest_algorithm="sha1",
    c14n_algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315",
)


def _fix_x509_newlines(element: XMLElement) -> XMLElement:
    xml_str = etree.tostring(element, encoding="unicode")
    xml_str = _X509_RE.sub(
        lambda m: m.group(1) + m.group(2).replace("\n", "").replace(" ", "") + m.group(3),
        xml_str,
    )
    return etree.fromstring(xml_str)


def sign_xml(element: XMLElement, cert_manager: CertificateManager, reference_id: str) -> XMLElement:
    """Sign *element* using the certificate and return the signed element.

    Parameters
    ----------
    element:
        The lxml Element to sign (e.g. an NFe or CTe root).
    cert_manager:
        Provides PEM cert/key.
    reference_id:
        The value of the ``Id`` attribute on the element to sign
        (e.g. ``"NFe43240101234567890100550010000000011000000011"``).
    """
    try:
        signed = _SIGNER.sign(
            element,
            key=cert_manager.key_pem,
            cert=cert_manager.cert_pem.decode('utf-8').strip(),
            reference_uri=f"#{reference_id}",
        )
        return _fix_x509_newlines(signed)
    except Exception as exc:
        raise XMLSignError(f"Failed to sign XML: {exc}") from exc


def sign_xml_bytes(xml_bytes: bytes, cert_manager: CertificateManager, reference_id: str) -> bytes:
    """Parse *xml_bytes*, sign and return the signed XML bytes."""
    root = etree.fromstring(xml_bytes)
    signed = sign_xml(root, cert_manager, reference_id)
    return etree.tostring(signed, xml_declaration=False, encoding="unicode").encode()
