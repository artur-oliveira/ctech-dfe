"""SOAP 1.2 envelope builder for SEFAZ web services."""

from __future__ import annotations

import base64
import gzip as gz

from lxml import etree

from py_dfe.constants.endpoints import get_authorizer
from py_dfe.constants.enums import (
    DOC_NAMESPACE,
    SCHEMA_VERSION,
    SOAP_ELEMENTS,
    SOAP_HEADER_VERSION_OVERRIDE,
    SOAP_WRAPPED_BODY_OVERRIDES,
    SOAP_WRAPPED_BODY_SERVICES,
    UF_IBGE,
    WSDL_OPERATION_BY_DOCTYPE,
    WSDL_OPERATION_OVERRIDES,
    WSDL_SERVICE_BY_DOCTYPE,
)
from py_dfe.constants.types import XMLElement
from py_dfe.exceptions import SOAPError
from py_dfe.xmlops.builder import create_element, create_subelement

_SOAP12_NS = "http://www.w3.org/2003/05/soap-envelope"
_XSI_NS = "http://www.w3.org/2001/XMLSchema-instance"
_XSD_NS = "http://www.w3.org/2001/XMLSchema"


def _resolve_operation(doc_type: str, uf: str, service: str) -> str:
    """Return the WSDL operation name for the given combination.

    Checks per-authorizer overrides first, then falls back to the global default.
    """
    authorizer = get_authorizer(doc_type, uf)
    override = WSDL_OPERATION_OVERRIDES.get((authorizer, doc_type, service))
    if override:
        return override
    return WSDL_OPERATION_BY_DOCTYPE[doc_type].get(service, "")


class SOAPEnvelopeBuilder:
    """Builder for SEFAZ SOAP 1.2 envelopes.

    Usage
    -----
    >>> payload_bytes = b'<test></test>'
    >>> builder = SOAPEnvelopeBuilder("nfe", "SP", "NFeAutorizacao")
    >>> envelope_bytes = builder.build(payload_bytes)
    """

    def __init__(self, doc_type: str, uf: str, service: str) -> None:
        self._doc_type = doc_type
        self._uf = uf
        self._service = service

        wsdl_name = WSDL_SERVICE_BY_DOCTYPE[doc_type].get(service)
        if wsdl_name is None:
            raise SOAPError(f"Unknown service {service!r} for doc_type {doc_type!r}")

        base_wsdl = f"{DOC_NAMESPACE[doc_type]}/wsdl"
        self._wsdl_ns = f"{base_wsdl}/{wsdl_name}"
        self._authorizer = get_authorizer(doc_type, uf)
        self._operation = _resolve_operation(doc_type, uf, service)
        self._version = SOAP_HEADER_VERSION_OVERRIDE.get(service, SCHEMA_VERSION[doc_type])
        self._cuf = str(UF_IBGE.get(uf, 0))
        elems = SOAP_ELEMENTS[doc_type]
        self._header_elem = elems["header"]
        self._body_elem = elems["body"]

    def build(self, payload_xml: bytes, gzip=False, include_header=False) -> bytes:
        """Wrap *payload_xml* in a SOAP 1.2 envelope and return as bytes."""
        try:
            envelope = self._make_envelope(payload_xml, gzip, include_header)
            return etree.tostring(envelope, xml_declaration=True, encoding="UTF-8")
        except SOAPError:
            raise
        except Exception as exc:
            raise SOAPError(f"Failed to build SOAP envelope: {exc}") from exc

    @property
    def content_type(self) -> str:
        """Content-Type header value including the SOAP action."""
        return (
            f"application/soap+xml; charset=utf-8; "
            f'action="{self._wsdl_ns}/{self._operation}"'
        )

    def _make_envelope(
        self,
        payload_xml: bytes,
        gzip: bool = False,
        include_header: bool = False,
    ) -> XMLElement:
        nsmap = {
            "soap12": _SOAP12_NS,
            "xsi": _XSI_NS,
            "xsd": _XSD_NS,
        }
        envelope = create_element(f"{{{_SOAP12_NS}}}Envelope", nsmap=nsmap)
        ns = self._wsdl_ns
        if include_header:
            header = create_subelement(envelope, f"{{{_SOAP12_NS}}}Header")
            cab = create_subelement(
                header, f"{{{ns}}}{self._header_elem}", nsmap={None: ns}
            )
            create_subelement(cab, f"{{{ns}}}cUF").text = self._cuf
            create_subelement(cab, f"{{{ns}}}versaoDados").text = self._version

        body = create_subelement(envelope, f"{{{_SOAP12_NS}}}Body")
        if self._service in SOAP_WRAPPED_BODY_SERVICES or (self._authorizer, self._service) in SOAP_WRAPPED_BODY_OVERRIDES:
            wrapper = create_subelement(
                body, f"{{{ns}}}{self._operation}", nsmap={None: ns}
            )
            data = create_subelement(wrapper, f"{{{ns}}}{self._body_elem}")
        else:
            data = create_subelement(
                body, f"{{{ns}}}{self._body_elem}", nsmap={None: ns}
            )

        if gzip:
            data.text = base64.b64encode(gz.compress(payload_xml)).decode("utf-8")
        else:
            payload_root = etree.fromstring(payload_xml)
            data.append(payload_root)

        return envelope


def extract_body(soap_response: bytes) -> bytes:
    """Extract the first child element of the SOAP Body and return as bytes."""
    try:
        root = etree.fromstring(soap_response)
        body = root.find(f"{{{_SOAP12_NS}}}Body")
        if body is None:
            raise SOAPError("SOAP Body element not found in response")
        children = list(body)
        if not children:
            raise SOAPError("SOAP Body is empty")
        return etree.tostring(children[0])
    except SOAPError:
        raise
    except Exception as exc:
        raise SOAPError(f"Failed to extract SOAP body: {exc}") from exc
