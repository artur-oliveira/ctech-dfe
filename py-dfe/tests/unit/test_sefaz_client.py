"""Unit tests for SefazClient - HTTP layer with retry logic."""

from __future__ import annotations

from unittest.mock import MagicMock, patch, call
import time

import httpx
import pytest

from py_dfe.services.base import SefazClient
from py_dfe.exceptions import RetryExhaustedError, SOAPError


_SOAP_RESPONSE = (
    b'<?xml version="1.0" encoding="UTF-8"?>'
    b'<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">'
    b'  <soap12:Body>'
    b'    <nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NfeStatusServico4">'
    b'      <retConsStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">'
    b'        <tpAmb>2</tpAmb><cStat>107</cStat><xMotivo>Servico em Operacao.</xMotivo>'
    b'      </retConsStatServ>'
    b'    </nfeResultMsg>'
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


def _make_client(cert_manager):
    return SefazClient(
        doc_type="nfe",
        uf="SP",
        environment="hom",
        cert_manager=cert_manager,
        validate_schema=False,
        max_retries=2,
    )


class TestSefazClientCall:
    def test_successful_call(self, cert_manager):
        client = _make_client(cert_manager)
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = _SOAP_RESPONSE

        with patch.object(httpx.Client, "post", return_value=mock_response):
            result = client.call("NfeStatusServico", _STATUS_PAYLOAD)

        assert "nfeResultMsg" in result or "retConsStatServ" in str(result)

    def test_http_500_triggers_retry(self, cert_manager):
        client = _make_client(cert_manager)
        fail = MagicMock(status_code=500)
        success = MagicMock(status_code=200, content=_SOAP_RESPONSE)

        with patch.object(httpx.Client, "post", side_effect=[fail, success]):
            with patch.object(SefazClient, "_sleep"):  # skip actual sleep
                result = client.call("NfeStatusServico", _STATUS_PAYLOAD)

        assert result is not None

    def test_exhausts_retries_on_timeout(self, cert_manager):
        client = _make_client(cert_manager)

        with patch.object(
            httpx.Client, "post", side_effect=httpx.TimeoutException("timeout")
        ):
            with patch.object(SefazClient, "_sleep"):
                with pytest.raises(RetryExhaustedError):
                    client.call("NfeStatusServico", _STATUS_PAYLOAD)

    def test_http_4xx_raises_immediately(self, cert_manager):
        client = _make_client(cert_manager)
        mock_response = MagicMock(status_code=400, text="Bad Request")

        with patch.object(httpx.Client, "post", return_value=mock_response):
            with pytest.raises(SOAPError) as exc_info:
                client.call("NfeStatusServico", _STATUS_PAYLOAD)

        assert exc_info.value.status_code == 400

    def test_no_retry_on_network_error_when_max_retries_zero(self, cert_manager):
        client = SefazClient(
            doc_type="nfe", uf="SP", environment="hom",
            cert_manager=cert_manager, max_retries=0,
        )
        with patch.object(
            httpx.Client, "post", side_effect=httpx.NetworkError("down")
        ):
            with pytest.raises(RetryExhaustedError):
                client.call("NfeStatusServico", _STATUS_PAYLOAD)
