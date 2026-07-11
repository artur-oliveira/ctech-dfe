"""Integration: full DANFC-e render through the Lambda handler.

Skipped when WeasyPrint native libraries are unavailable.
"""

from __future__ import annotations

import base64
import json

import pytest

from py_dfe.constants import danfe as c
from py_dfe.handler import handler
from tests.danfe_fixtures import sample_nfe_proc

pytestmark = pytest.mark.integration


def _event(**body):
    return {
        "cnpj": "12345678000199", "uf": "SP", "environment": "producao",
        "doc_type": "nfce", "service": c.SERVICE_GERAR_DANFE, "body": body,
    }


def setup_module(_):
    pytest.importorskip("weasyprint")


def test_full_completo_pdf_and_html():
    resp = handler(_event(xml=sample_nfe_proc(), layout=c.LAYOUT_COMPLETO), None)
    assert resp["statusCode"] == 200
    body = json.loads(resp["body"])
    assert base64.b64decode(body["pdf_b64"])[:4] == b"%PDF"
    assert len(body["html"]) == 1
    html = body["html"][0]
    for needle in (c.TEXT_DOC_AUXILIAR, "PRODUTO TESTE 1", "data:image/png;base64,", "Protocolo"):
        assert needle in html


def test_full_contingencia_two_vias():
    resp = handler(_event(xml=sample_nfe_proc(tp_emis=c.TP_EMIS_CONTINGENCIA_OFFLINE)), None)
    body = json.loads(resp["body"])
    assert len(body["html"]) == 2
    assert c.VIA_ESTABELECIMENTO in body["html"][1]
    assert c.TEXT_CONTINGENCIA_L1 in "".join(body["html"])


def test_full_homologacao_and_resumido():
    resp = handler(_event(xml=sample_nfe_proc(tp_amb=c.TP_AMB_HOMOLOGACAO), layout=c.LAYOUT_RESUMIDO), None)
    body = json.loads(resp["body"])
    html = body["html"][0]
    assert c.TEXT_HOMOLOGACAO in html
    assert "PRODUTO TESTE 1" not in html
