"""Unit tests for Pydantic request/response models."""

import pytest
from pydantic import ValidationError

from py_dfe.models.request import LambdaRequest, LambdaResponse


class TestLambdaRequest:
    def _valid(self, pfx_b64, **overrides):
        base = {
            "cnpj": "12345678000195",
            "certificate_b64": pfx_b64,
            "certificate_password": "senha",
            "uf": "SP",
            "environment": "hom",
            "doc_type": "nfe",
            "service": "NFeAutorizacao",
            "body": {"root": {}},
        }
        base.update(overrides)
        return LambdaRequest.model_validate(base)

    def test_valid_request(self, pfx_b64):
        req = self._valid(pfx_b64)
        assert req.cnpj == "12345678000195"
        assert req.uf == "SP"

    def test_uf_uppercased(self, pfx_b64):
        req = self._valid(pfx_b64, uf="sp")
        assert req.uf == "SP"

    def test_doc_type_lowercased(self, pfx_b64):
        req = self._valid(pfx_b64, doc_type="NFe")
        assert req.doc_type == "nfe"

    def test_invalid_cnpj_length(self, pfx_b64):
        with pytest.raises(ValidationError):
            self._valid(pfx_b64, cnpj="123")

    def test_invalid_ambiente(self, pfx_b64):
        with pytest.raises(ValidationError):
            self._valid(pfx_b64, environment="invalid")

    def test_invalid_doc_type(self, pfx_b64):
        with pytest.raises(ValidationError):
            self._valid(pfx_b64, doc_type="boleto")

    def test_max_retries_default(self, pfx_b64):
        req = self._valid(pfx_b64)
        assert req.max_retries == 3

    def test_max_retries_bounded(self, pfx_b64):
        with pytest.raises(ValidationError):
            self._valid(pfx_b64, max_retries=20)
