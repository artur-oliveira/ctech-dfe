import json

import pytest

from py_dfe.constants import danfe as c
from py_dfe.handler import handler
from py_dfe.models.request import LambdaRequest
from tests.danfe_fixtures import sample_nfe_proc


def test_request_accepts_missing_certificate():
    req = LambdaRequest.model_validate({
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": c.SERVICE_GERAR_DANFE,
        "body": {"xml": "<x/>"},
    })
    assert req.certificate_b64 is None


def test_handler_routes_gerardanfe_without_cert():
    pytest.importorskip("weasyprint")
    event = {
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": c.SERVICE_GERAR_DANFE,
        "body": {"xml": sample_nfe_proc(), "layout": c.LAYOUT_COMPLETO},
    }
    resp = handler(event, None)
    assert resp["statusCode"] == 200
    body = json.loads(resp["body"])
    assert "pdf_b64" in body and body["html"]


def test_handler_rejects_sefaz_service_without_cert():
    event = {
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": "NfeStatusServico", "body": {},
    }
    resp = handler(event, None)
    assert resp["statusCode"] == 400  # CERT_REQUIRED
