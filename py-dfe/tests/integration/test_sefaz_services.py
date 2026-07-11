"""Integration tests using respx to mock HTTP at the transport level.

Exercises the full pipeline: JSON → XML → SOAP → (mocked) HTTP → parse → dict.
"""

from __future__ import annotations

from unittest.mock import patch

import httpx
import pytest

from py_dfe.services import create_service
from py_dfe.services.base import SefazClient

_STATUS_RESPONSE = (
    b'<?xml version="1.0" encoding="UTF-8"?>'
    b'<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">'
    b'  <soap12:Body>'
    b'    <nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NfeStatusServico4">'
    b'      <retConsStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">'
    b'        <tpAmb>2</tpAmb><verAplic>SVRS202404231028</verAplic>'
    b'        <cStat>107</cStat><xMotivo>Servico em Operacao.</xMotivo>'
    b'        <cUF>35</cUF><dhRecbto>2024-04-23T10:28:00-03:00</dhRecbto>'
    b'      </retConsStatServ>'
    b'    </nfeResultMsg>'
    b'  </soap12:Body>'
    b'</soap12:Envelope>'
)

_MDFE_STATUS_RESPONSE = (
    b'<?xml version="1.0" encoding="UTF-8"?>'
    b'<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">'
    b'  <soap12:Body>'
    b'    <mdfeResultMsg xmlns="http://www.portalfiscal.inf.br/mdfe/wsdl/MDFeStatusServico">'
    b'      <retConsStatServMDFe versao="3.00" xmlns="http://www.portalfiscal.inf.br/mdfe">'
    b'        <tpAmb>2</tpAmb><cStat>107</cStat><xMotivo>Servico em Operacao.</xMotivo>'
    b'      </retConsStatServMDFe>'
    b'    </mdfeResultMsg>'
    b'  </soap12:Body>'
    b'</soap12:Envelope>'
)

_STATUS_PAYLOAD = {"consStatServ": {
    "@versao": "4.00",
    "@xmlns": "http://www.portalfiscal.inf.br/nfe",
    "tpAmb": "2",
    "cUF": "35",
    "xServ": "STATUS",
}}


def _mock_post(response_content: bytes) -> httpx.Response:
    return httpx.Response(200, content=response_content)


@pytest.mark.integration
class TestNFeServiceIntegration:
    def test_status_service(self, cert_manager):
        """Full pipeline from call() to parsed dict."""
        svc = create_service("nfe", cert_manager, uf="SP", environment="hom")
        with patch.object(httpx.Client, "post", return_value=_mock_post(_STATUS_RESPONSE)):
            result = svc.status_service(uf="35")
        assert result is not None
        assert "retConsStatServ" in str(result) or "nfeResultMsg" in str(result)

    def test_call_generic_status(self, cert_manager):
        svc = create_service("nfe", cert_manager, uf="SP", environment="hom")
        with patch.object(httpx.Client, "post", return_value=_mock_post(_STATUS_RESPONSE)):
            result = svc.call("NfeStatusServico", _STATUS_PAYLOAD)
        assert result is not None

    def test_sp_hits_sp_endpoint(self, cert_manager):
        """Verify SP calls the SP-specific SEFAZ URL."""
        from py_dfe.constants.endpoints import get_endpoint
        url = get_endpoint("nfe", "SP", "hom", "NfeStatusServico")
        assert "fazenda.sp.gov.br" in url

    def test_rj_routes_to_svrs(self, cert_manager):
        from py_dfe.constants.endpoints import get_endpoint
        url = get_endpoint("nfe", "RJ", "hom", "NFeAutorizacao")
        assert "svrs.rs.gov.br" in url

    def test_am_routes_to_am(self, cert_manager):
        from py_dfe.constants.endpoints import get_endpoint
        url = get_endpoint("nfe", "AM", "hom", "NFeAutorizacao")
        assert "sefaz.am.gov.br" in url

    def test_ba_routes_to_ba(self, cert_manager):
        from py_dfe.constants.endpoints import get_endpoint
        url = get_endpoint("nfe", "BA", "hom", "NFeAutorizacao")
        assert "sefaz.ba.gov.br" in url

    def test_homologacao_url_differs_from_producao(self, cert_manager):
        from py_dfe.constants.endpoints import get_endpoint
        prod_url = get_endpoint("nfe", "SP", "prod", "NFeAutorizacao")
        hom_url = get_endpoint("nfe", "SP", "hom", "NFeAutorizacao")
        assert prod_url != hom_url


@pytest.mark.integration
class TestMDFeServiceIntegration:
    def test_status_service(self, cert_manager):
        svc = create_service("mdfe", cert_manager, uf="SP", environment="hom")
        with patch.object(httpx.Client, "post", return_value=_mock_post(_MDFE_STATUS_RESPONSE)):
            result = svc.status_service()
        assert result is not None

    def test_all_states_hit_svrs(self):
        from py_dfe.constants.endpoints import get_endpoint
        for uf in ["SP", "RJ", "MG", "RS", "GO", "AM"]:
            url = get_endpoint("mdfe", uf, "prod", "MDFeRecepcaoSinc")
            assert "mdfe.svrs.rs.gov.br" in url, f"MDF-e {uf} should route to SVRS"


@pytest.mark.integration
class TestCTeServiceIntegration:
    def test_status_service_mg(self, cert_manager):
        svc = create_service("cte", cert_manager, uf="MG", environment="hom")

        payload = {"consStatServCTe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/cte",
            "tpAmb": "2",
            "cUF": "31",
            "xServ": "STATUS",
        }}
        with patch.object(httpx.Client, "post", return_value=_mock_post(_STATUS_RESPONSE)):
            result = svc.call("CTeStatusServico", payload)
        assert result is not None

    def test_mg_routes_to_mg(self):
        from py_dfe.constants.endpoints import get_endpoint
        url = get_endpoint("cte", "MG", "prod", "CTeRecepcaoSinc")
        assert "cte.fazenda.mg.gov.br" in url

    def test_go_routes_to_svrs(self):
        from py_dfe.constants.endpoints import get_endpoint
        url = get_endpoint("cte", "GO", "prod", "CTeRecepcaoSinc")
        assert "svrs.rs.gov.br" in url


@pytest.mark.integration
class TestRetryIntegration:
    def test_retries_on_503_then_succeeds(self, cert_manager):
        svc = create_service("nfe", cert_manager, uf="SP", environment="hom", max_retries=2)
        responses = [
            httpx.Response(503, content=b"Service Unavailable"),
            httpx.Response(200, content=_STATUS_RESPONSE),
        ]
        with patch.object(httpx.Client, "post", side_effect=responses):
            with patch.object(SefazClient, "_sleep"):
                result = svc.call("NfeStatusServico", _STATUS_PAYLOAD)
        assert result is not None

    def test_exhausts_all_retries(self, cert_manager):
        from py_dfe.exceptions import RetryExhaustedError
        svc = create_service("nfe", cert_manager, uf="SP", environment="hom", max_retries=1)
        with patch.object(httpx.Client, "post", side_effect=httpx.TimeoutException("timeout")):
            with patch.object(SefazClient, "_sleep"):
                with pytest.raises(RetryExhaustedError):
                    svc.call("NfeStatusServico", _STATUS_PAYLOAD)

    def test_no_retry_on_400(self, cert_manager):
        from py_dfe.exceptions import SOAPError
        svc = create_service("nfe", cert_manager, uf="SP", environment="hom", max_retries=3)

        call_count = 0

        def mock_post(*a, **kw):
            nonlocal call_count
            call_count += 1
            return httpx.Response(400, text="Bad Request")

        with patch.object(httpx.Client, "post", side_effect=mock_post):
            with pytest.raises(SOAPError):
                svc.call("NfeStatusServico", _STATUS_PAYLOAD)

        assert call_count == 1, "4xx should not be retried"
