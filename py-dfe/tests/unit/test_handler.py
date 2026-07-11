"""Unit tests for the Lambda handler."""

from __future__ import annotations

import json
from unittest.mock import MagicMock, patch

import pytest

from py_dfe.handler import handler
from py_dfe.exceptions import SOAPError


MOCK_RESPONSE = {"retConsStatServ": {"cStat": "107", "xMotivo": "Serviço em Operação."}}


class TestHandler:
    def test_valid_event_calls_service(self, lambda_event):
        with patch("py_dfe.handler.create_service") as mock_factory:
            mock_svc = MagicMock()
            mock_svc.call.return_value = MOCK_RESPONSE
            mock_factory.return_value = mock_svc

            result = handler(lambda_event, None)

        assert result["statusCode"] == 200
        body = result["body"]
        assert isinstance(body, str)
        assert json.loads(body) == MOCK_RESPONSE

    def test_api_gateway_body_string(self, lambda_event):
        """Handler must unwrap API Gateway proxy integration body."""
        event = {"body": json.dumps(lambda_event)}
        with patch("py_dfe.handler.create_service") as mock_factory:
            mock_svc = MagicMock()
            mock_svc.call.return_value = MOCK_RESPONSE
            mock_factory.return_value = mock_svc

            result = handler(event, None)

        assert result["statusCode"] == 200

    def test_validation_error_returns_500(self, pfx_b64):
        bad_event = {
            "cnpj": "123",  # invalid
            "certificate_b64": pfx_b64,
            "certificate_password": "x",
            "uf": "SP",
            "environment": "hom",
            "doc_type": "nfe",
            "service": "NFeAutorizacao",
            "body": {},
        }
        result = handler(bad_event, None)
        assert result["statusCode"] == 422
        assert isinstance(result["body"], str)
        assert json.loads(result["body"])['title'] == "validation error"

    def test_soap_error_returns_500(self, lambda_event):
        with patch("py_dfe.handler.create_service") as mock_factory:
            mock_svc = MagicMock()
            mock_svc.call.side_effect = SOAPError("Connection failed")
            mock_factory.return_value = mock_svc

            result = handler(lambda_event, None)

        assert result["statusCode"] == 400
        assert isinstance(result["body"], str)
        assert json.loads(result["body"])['title'] == "soap request error"

    def test_unexpected_error_returns_500(self, lambda_event):
        with patch("py_dfe.handler.create_service") as mock_factory:
            mock_svc = MagicMock()
            mock_svc.call.side_effect = RuntimeError("Unexpected")
            mock_factory.return_value = mock_svc

            result = handler(lambda_event, None)

        assert result["statusCode"] == 500
        assert isinstance(result["body"], str)
        assert json.loads(result["body"])['title'] == "unexpected error"
