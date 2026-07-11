"""
Integration tests - MDF-e, SVRS (único autorizador), HOMOLOGAÇÃO.

Skipped unless TEST_CERT_PATH and TEST_CERT_PASSWORD are set.
"""

from __future__ import annotations

import os

import pytest

from py_dfe.constants.enums import DocType, Environment, UF
from py_dfe.services import create_service
from tests.integration.fiscal_payloads import (
    build_mdfe,
    build_mdfe_cancelamento,
    build_mdfe_consulta,
)

_SKIP = pytest.mark.skipif(
    os.environ.get("TEST_CERT_PATH") is None or os.getenv("TEST_CERT_PASSWORD") is None,
    reason="TEST_CERT_PATH / TEST_CERT_PASSWORD not set - skipping real SEFAZ tests",
)

_CNPJ = "11647612000197"

_STATUS_STATS = frozenset({"107", "108", "109"})
_AUTH_STATS = frozenset({"100"})
_CANCEL_STATS = frozenset({"101", "135"})
_CONS_NAO_ENC_STATS = frozenset({"140", "141"})


def _mdfe_svc(real_cert_manager, uf: str):
    return create_service(DocType.MDFE.value, real_cert_manager, uf=uf, environment=Environment.HOMOLOGATION)


@pytest.mark.integration
@_SKIP
class TestMDFeSVRS:
    _chave: str | None = None
    _nprot: str | None = None

    def test_status_service(self, real_cert_manager):
        result = _mdfe_svc(real_cert_manager, UF.SP.value).status_service()

        assert isinstance(result, dict)
        ret = result.get("mdfeResultMsg", {}).get("retConsStatServMDFe", {})
        assert ret.get("cStat") in _STATUS_STATS, ret

    def test_cons_nao_enc(self, real_cert_manager):
        svc = _mdfe_svc(real_cert_manager, UF.SP.value)
        payload = {
            "consMDFeNaoEnc": {
                "@versao": "3.00",
                "@xmlns": "http://www.portalfiscal.inf.br/mdfe",
                "tpAmb": "2",
                "xServ": "CONS-MDFe-NAO-ENC",
                "CNPJ": _CNPJ,
            }
        }
        result = svc.cons_nao_enc(payload)

        assert isinstance(result, dict)
        ret = result.get("mdfeResultMsg", {}).get("retConsMDFeNaoEnc", {})
        assert ret.get("cStat") in _CONS_NAO_ENC_STATS, ret

    def test_authorization(self, real_cert_manager):
        svc = _mdfe_svc(real_cert_manager, UF.PI.value)
        payload, chave = build_mdfe()
        result = svc.recepcao_sinc(payload)

        ret = result.get("mdfeResultMsg", {}).get("retMDFe", {})
        assert ret.get("cStat") in _AUTH_STATS, f"MDF-e not authorized: {ret}"

        prot = ret.get("protMDFe", {}).get("infProt", {})
        assert prot.get("cStat") in _AUTH_STATS, f"MDF-e protocol not OK: {prot}"

        TestMDFeSVRS._chave = chave
        TestMDFeSVRS._nprot = prot.get("nProt")

    def test_consultar(self, real_cert_manager):
        if TestMDFeSVRS._chave is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = _mdfe_svc(real_cert_manager, UF.PI.value)
        result = svc.consultar(build_mdfe_consulta(TestMDFeSVRS._chave))

        ret = result.get("mdfeResultMsg", {}).get("retConsSitMDFe", {})
        assert ret.get("cStat") in {"100", "101"}, f"Unexpected status: {ret}"

    def test_cancelar(self, real_cert_manager):
        if TestMDFeSVRS._chave is None or TestMDFeSVRS._nprot is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = _mdfe_svc(real_cert_manager, UF.PI.value)
        payload = build_mdfe_cancelamento(TestMDFeSVRS._chave, TestMDFeSVRS._nprot)
        result = svc.perform_event(payload)

        ret = result.get("mdfeResultMsg", {})
        ret_env = ret.get("retEventoMDFe", ret)
        ret_evento = ret_env.get("retEvento", {})
        if isinstance(ret_evento, list):
            ret_evento = ret_evento[0]
        inf = ret_evento.get("infEvento", {})
        assert inf.get("cStat") in _CANCEL_STATS, f"MDF-e cancellation failed: {inf}"
