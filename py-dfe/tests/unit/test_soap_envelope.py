"""Unit tests for SOAP envelope builder."""

import pytest
from lxml import etree

from py_dfe.exceptions import SOAPError
from py_dfe.soap.envelope import SOAPEnvelopeBuilder, extract_body

_SOAP12_NS = "http://www.w3.org/2003/05/soap-envelope"
_SAMPLE_PAYLOAD = b'<consStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><tpAmb>2</tpAmb></consStatServ>'


class TestSOAPEnvelopeBuilder:
    def _builder(self, doc_type="nfe", uf="SP", service="NfeStatusServico"):
        return SOAPEnvelopeBuilder(doc_type, uf, service)

    def test_build_returns_bytes(self):
        result = self._builder().build(_SAMPLE_PAYLOAD)
        assert isinstance(result, bytes)

    def test_envelope_has_soap12_ns(self):
        result = self._builder().build(_SAMPLE_PAYLOAD)
        root = etree.fromstring(result)
        assert root.tag == f"{{{_SOAP12_NS}}}Envelope"

    def test_body_wraps_payload(self):
        result = self._builder().build(_SAMPLE_PAYLOAD)
        root = etree.fromstring(result)
        body = root.find(f"{{{_SOAP12_NS}}}Body")
        assert body is not None
        dados = list(body)[0]
        assert "nfeDadosMsg" in dados.tag
        assert dados.find("{http://www.portalfiscal.inf.br/nfe}consStatServ") is not None

    def test_content_type_contains_action(self):
        builder = self._builder("nfe", "SP", "NFeAutorizacao")
        ct = builder.content_type
        assert "application/soap+xml" in ct
        assert "NFeAutorizacao4" in ct
        assert "nfeAutorizacaoLote" in ct

    def test_content_type_status_service_operation(self):
        builder = self._builder("nfe", "SP", "NfeStatusServico")
        ct = builder.content_type
        assert "nfeStatusServicoNF" in ct


    def test_unknown_service_raises(self):
        with pytest.raises(SOAPError):
            SOAPEnvelopeBuilder("nfe", "SP", "UnknownService")


class TestExtractBody:
    def test_extracts_result_element(self):
        envelope_xml = (
            b'<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">'
            b'  <soap12:Body>'
            b'    <nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NfeStatusServico4">'
            b'      <retConsStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">'
            b'        <cStat>107</cStat>'
            b'      </retConsStatServ>'
            b'    </nfeResultMsg>'
            b'  </soap12:Body>'
            b'</soap12:Envelope>'
        )
        body = extract_body(envelope_xml)
        root = etree.fromstring(body)
        assert "nfeResultMsg" in root.tag

    def test_missing_body_raises(self):
        with pytest.raises(SOAPError):
            extract_body(b"<root><other/></root>")
