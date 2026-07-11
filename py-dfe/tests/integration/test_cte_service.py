"""
Integration tests - CT-e, all authorizers, HOMOLOGAÇÃO.

Skipped unless TEST_CERT_PATH and TEST_CERT_PASSWORD are set.
Every authorizer runs status + authorization (response may be a rejection, but
communication must succeed). SVRS (PI) runs the full chain.
"""

from __future__ import annotations

import os

import pytest

from py_dfe.constants.enums import DocType, Environment, UF
from py_dfe.services import create_service
from tests.integration.fiscal_payloads import (
    build_cte,
    build_cte_cancelamento,
)

_SKIP = pytest.mark.skipif(
    os.environ.get("TEST_CERT_PATH") is None or os.getenv("TEST_CERT_PASSWORD") is None,
    reason="TEST_CERT_PATH / TEST_CERT_PASSWORD not set - skipping real SEFAZ tests",
)

_STATUS_STATS = frozenset({"107", "108", "109"})
_AUTH_STATS = frozenset({"100"})
_CANCEL_STATS = frozenset({"101", "135"})
_CHECK_CTE_RECEPTION_STATS = frozenset({'646', '209'})


def _cte_svc(real_cert_manager, uf: str):
    return create_service(DocType.CTE.value, real_cert_manager, uf=uf, environment=Environment.HOMOLOGATION)


def _assert_status(result: dict, cuf: str) -> None:
    assert isinstance(result, dict)
    ret = result.get("retConsStatServCTe", {})
    assert ret.get("cStat") in _STATUS_STATS, ret
    assert ret.get("cUF") == cuf


def _assert_comunicacao_authorization(result: dict) -> None:
    assert isinstance(result, dict), "No response dict"
    ret = result.get("retCTe", {})
    assert ret.get("cStat") in _CHECK_CTE_RECEPTION_STATS, f"Invalid cStat in retCTe: {ret}"


@pytest.mark.integration
@_SKIP
class TestCTeMG:
    def test_status_service(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.MG.value).status_service(uf="31")
        _assert_status(result, "31")

    def test_authorization(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.MG.value).recepcao_sinc(build_cte(uf="MG")[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestCTeMS:
    def test_status_service(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.MS.value).status_service(uf="50")
        _assert_status(result, "50")

    def test_authorization(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.MS.value).recepcao_sinc(build_cte(uf="MS")[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestCTeMT:
    def test_status_service(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.MT.value).status_service(uf="51")
        _assert_status(result, "51")

    def test_authorization(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.MT.value).recepcao_sinc(build_cte(uf="MT")[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestCTePR:
    def test_status_service(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.PR.value).status_service(uf="41")
        _assert_status(result, "41")

    def test_authorization(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.PR.value).recepcao_sinc(build_cte(uf="PR")[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestCTeSP:
    def test_status_service(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.SP.value).status_service(uf="35")
        _assert_status(result, "35")

    def test_authorization(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.SP.value).recepcao_sinc(build_cte(uf="SP")[0])
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestCTeSVRS:
    _chave: str | None = None
    _nprot: str | None = None

    def test_status_service(self, real_cert_manager):
        result = _cte_svc(real_cert_manager, UF.GO.value).status_service(uf="52")
        _assert_status(result, "52")

    def test_authorization(self, real_cert_manager):
        svc = create_service(
            DocType.CTE, real_cert_manager, uf=UF.PI.value, environment=Environment.HOMOLOGATION
        )
        payload, chave = build_cte(uf="PI")
        result = svc.recepcao_sinc(payload)

        _assert_comunicacao_authorization(result)

        TestCTeSVRS._chave = chave
        TestCTeSVRS._nprot = "322260000007475"

    def test_query_cte_by_access_key(self, real_cert_manager):
        if TestCTeSVRS._chave is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = create_service(
            DocType.CTE, real_cert_manager, uf=UF.PI.value, environment=Environment.HOMOLOGATION
        )
        result = svc.query_cte_by_access_key(TestCTeSVRS._chave)

        ret = result.get("retConsSitCTe", {})
        assert ret.get("cStat") in {"217"}, f"Unexpected status: {ret}"

    def test_cancelar(self, real_cert_manager):
        if TestCTeSVRS._chave is None or TestCTeSVRS._nprot is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = create_service(
            DocType.CTE, real_cert_manager, uf=UF.PI.value, environment=Environment.HOMOLOGATION
        )
        ret = svc.cancel_cte(TestCTeSVRS._chave, 'Teste de cancelamento de CT-e', 1, TestCTeSVRS._nprot)

        inf = ret.get("retEventoCTe", ret).get("infEvento", {})
        assert inf.get("cStat") == '217', f"CT-e cancellation failed: {inf}"
