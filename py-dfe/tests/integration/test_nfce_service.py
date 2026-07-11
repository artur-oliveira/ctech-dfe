"""
Integration tests - NFC-e, all authorizers, HOMOLOGAÇÃO.

Skipped unless TEST_CERT_PATH and TEST_CERT_PASSWORD are set.
Every authorizer runs status + authorization (response may be a rejection, but
communication must succeed). SVRS (PI) also runs cancelar.
"""

from __future__ import annotations

import os

import pytest

from py_dfe.constants.enums import DocType, Environment, UF
from py_dfe.services import create_service
from tests.integration.fiscal_payloads import build_nfe, build_nfe_cancelamento

_SKIP = pytest.mark.skipif(
    os.environ.get("TEST_CERT_PATH") is None or os.getenv("TEST_CERT_PASSWORD") is None,
    reason="TEST_CERT_PATH / TEST_CERT_PASSWORD not set - skipping real SEFAZ tests",
)

_STATUS_STATS = frozenset({"107", "108", "109"})
_AUTH_STATS = frozenset({"100"})
_CANCEL_STATS = frozenset({"101", "135"})


def _nfce_svc(real_cert_manager, uf: str):
    return create_service(DocType.NFCE.value, real_cert_manager, uf=uf, environment=Environment.HOMOLOGATION)


def _assert_status(result: dict, cuf: str) -> None:
    assert isinstance(result, dict)
    ret = result.get("retConsStatServ", {})
    assert ret.get("cStat") in _STATUS_STATS, ret
    assert ret.get("cUF") == cuf


def _assert_comunicacao_authorization(result: dict) -> None:
    assert isinstance(result, dict), "No response dict"
    ret = result.get("retEnviNFe", {})
    assert ret.get("cStat") is not None, f"No cStat in retEnviNFe: {ret}"


@pytest.mark.integration
@_SKIP
class TestNFCeAM:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.AM.value).status_service(uf="13")
        _assert_status(result, "13")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.AM.value).authorization(build_nfe(uf="AM", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCeGO:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.GO.value).status_service(uf="52")
        _assert_status(result, "52")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.GO.value).authorization(build_nfe(uf="GO", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCeMS:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.MS.value).status_service(uf="50")
        _assert_status(result, "50")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.MS.value).authorization(build_nfe(uf="MS", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCeMT:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.MT.value).status_service(uf="51")
        _assert_status(result, "51")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.MT.value).authorization(build_nfe(uf="MT", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCePR:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.PR.value).status_service(uf="41")
        _assert_status(result, "41")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.PR.value).authorization(build_nfe(uf="PR", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCeRS:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.RS.value).status_service(uf="43")
        _assert_status(result, "43")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.RS.value).authorization(build_nfe(uf="RS", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCeSP:
    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.SP.value).status_service(uf="35")
        _assert_status(result, "35")

    def test_authorization(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.SP.value).authorization(build_nfe(uf="SP", mod='65')[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFCeSVRS:
    _chave: str | None = None
    _nprot: str | None = None

    def test_status_service(self, real_cert_manager):
        result = _nfce_svc(real_cert_manager, UF.RJ.value).status_service(uf="33")
        _assert_status(result, "33")

    def test_authorization(self, real_cert_manager):
        svc = create_service(
            DocType.NFCE, real_cert_manager, uf=UF.PI.value, environment=Environment.HOMOLOGATION
        )
        payload, chave = build_nfe(uf="PI", mod='65')
        result = svc.authorization(payload)

        ret = result.get("retEnviNFe", {})
        assert ret.get("cStat") == "104", f"Expected lote processado (104), got: {ret}"

        prot = ret.get("protNFe", [{}])[0].get("infProt", {})
        assert prot.get("cStat") in _AUTH_STATS, f"NFC-e not authorized: {prot}"

        TestNFCeSVRS._chave = chave
        TestNFCeSVRS._nprot = prot.get("nProt") or "322260000007475"

    def test_cancelar(self, real_cert_manager):
        if TestNFCeSVRS._chave is None or TestNFCeSVRS._nprot is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = create_service(
            DocType.NFCE, real_cert_manager, uf=UF.PI.value, environment=Environment.HOMOLOGATION
        )
        payload = build_nfe_cancelamento(TestNFCeSVRS._chave, TestNFCeSVRS._nprot)
        result = svc.perform_event(payload)

        ret = result
        ret_env = ret.get("retEnvEvento", ret)
        ret_evento = ret_env.get("retEvento", {})
        if isinstance(ret_evento, list):
            ret_evento = ret_evento[0]
        inf = ret_evento.get("infEvento", {})
        assert inf.get("cStat") in _CANCEL_STATS, f"NFC-e cancellation failed: {inf}"
