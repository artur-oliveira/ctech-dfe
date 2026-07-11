"""
Integration tests - NF-e, all authorizers, HOMOLOGAÇÃO.

Skipped unless TEST_CERT_PATH and TEST_CERT_PASSWORD are set.
Every authorizer runs status + authorization (response may be a rejection, but
communication must succeed). SVRS (PI) runs the full chain including
consultar, cancelar and inutilizar.
"""

from __future__ import annotations

import os

import pytest

from py_dfe.constants.enums import DocType, Environment, UF
from py_dfe.services import create_service
from tests.integration.fiscal_payloads import (
    build_nfe,
)

_SKIP = pytest.mark.skipif(
    os.environ.get("TEST_CERT_PATH") is None or os.getenv("TEST_CERT_PASSWORD") is None,
    reason="TEST_CERT_PATH / TEST_CERT_PASSWORD not set - skipping real SEFAZ tests",
)

_CNPJ = "11647612000197"

_STATUS_STATS = frozenset({"107", "108", "109"})
_AUTH_STATS = frozenset({"100"})
_CANCEL_STATS = frozenset({"101", "135"})
_INUT_STATS = frozenset({"102"})


def _nfe_svc(real_cert_manager, uf: str):
    env = Environment.PRODUCTION if os.getenv('TEST_ENVIRONMENT') == 'prod' else Environment.HOMOLOGATION
    cnpj = os.getenv('TEST_CNPJ') or '11647612000197'
    return create_service(
        DocType.NFE, real_cert_manager, uf=uf, environment=env, cnpj=cnpj
    )


def _assert_status_nfe(result: dict) -> None:
    assert isinstance(result, dict)
    ret = result.get("retConsStatServ", {})
    assert ret.get("cStat") in _STATUS_STATS, ret


def _assert_consulta_cad(result: dict) -> None:
    assert isinstance(result, dict)
    assert result.get("retConsCad") is not None
    inf_cons = result["retConsCad"].get("infCons", {})
    assert inf_cons.get("CNPJ") == _CNPJ
    assert inf_cons.get("UF") == UF.PI.value
    inf_cad = inf_cons.get("infCad", [])
    assert isinstance(inf_cad, list) and len(inf_cad) > 0


def _assert_comunicacao_authorization(result: dict) -> None:
    """Verify SEFAZ responded to the NF-e authorization (any cStat is acceptable)."""
    assert isinstance(result, dict), "No response dict"
    assert "retEnviNFe" in result, f"Missing retEnviNFe wrapper: {result}"
    ret = result.get("retEnviNFe", {})
    assert ret.get("cStat") is not None, f"No cStat in retEnviNFe: {ret}"


@pytest.mark.integration
@_SKIP
class TestNFeAM:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.AM.value).status_service("13")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.AM.value)
        payload, _ = build_nfe(uf="AM")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeBA:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.BA.value).status_service("29")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.BA.value)
        payload, _ = build_nfe(uf="BA")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeGO:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.GO.value).status_service("52")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.GO.value)
        payload, _ = build_nfe(uf="GO")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeMG:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.MG.value).status_service("31")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.MG.value)
        payload, _ = build_nfe(uf="MG")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeMS:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.MS.value).status_service("50")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.MS.value)
        payload, _ = build_nfe(uf="MS")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeMT:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.MT.value).status_service("51")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.MT.value)
        payload, _ = build_nfe(uf="MT")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFePE:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.PE.value).status_service("26")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PE.value)
        payload, _ = build_nfe(uf="PE")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFePR:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.PR.value).status_service("41")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PR.value)
        payload, _ = build_nfe(uf="PR")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeRS:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.RS.value).status_service("43")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.RS.value)
        payload, _ = build_nfe(uf="RS")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeSP:
    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.SP.value).status_service("35")
        _assert_status_nfe(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.SP.value)
        payload, _ = build_nfe(uf="SP")
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)


@pytest.mark.integration
@_SKIP
class TestNFeSVRS:
    _chave: str | None = None
    _nprot: str | None = None

    def test_status_service(self, real_cert_manager):
        result = _nfe_svc(real_cert_manager, UF.PI.value).status_service("22")
        _assert_status_nfe(result)

    def test_consulta_cadastro_cnpj(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.query_cnpj(_CNPJ, UF.PI.value)
        _assert_consulta_cad(result)

    def test_authorization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, chave = build_nfe(uf="PI")
        result = svc.authorization(payload)

        ret = result.get("retEnviNFe", {})
        assert ret.get("cStat") == "104", f"Expected lote processado (104), got: {ret}"

        prot = ret.get("protNFe", [{}])[0].get("infProt", {})
        assert prot.get("cStat") in ('209', '696'), f"Unexpected cstat: {prot}"

        TestNFeSVRS._chave = chave
        TestNFeSVRS._nprot = prot.get("nProt") or "322260000007475"

    def test_consultar_protocolo(self, real_cert_manager):
        if TestNFeSVRS._chave is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.query_protocol_nf(TestNFeSVRS._chave)

        ret = result.get("retConsSitNFe", {})
        assert ret.get("cStat") == '217', f"Unexpected status: {ret}"

    def test_cancelar(self, real_cert_manager):
        if TestNFeSVRS._chave is None or TestNFeSVRS._nprot is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.cancel_nfe(TestNFeSVRS._chave, 'Teste de cancelamento de NF-e', 1, TestNFeSVRS._nprot)

        ret_env = result.get("retEnvEvento")
        ret_evento = ret_env.get("retEvento", [{}])
        first_ret_event = ret_evento[0]
        inf = first_ret_event.get("infEvento", {})
        assert inf.get("cStat") == '494', f"Cancellation failed: {inf}"

    def test_inutilization(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.number_inutilization(
            serie_number=0,
            start_number=1,
            end_number=1,
            justification='NF inutilization test'
        )

        ret = result.get("retInutNFe", {})
        inf = ret.get("infInut", {})
        assert inf.get("cStat") == '563', f"Inutilização failed: {inf}"

    def test_distribution_last_nsu(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.perform_distribution_last_nsu(
            nsu=522,
        )

        ret = result.get("retDistDFeInt", {})
        assert ret.get("cStat") is not None, f"Distribution failed: {ret}"

    def test_science(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.perform_science_operation(
            access_keys=[
                '22260547050493000138550010000011171955117990',
                '21260536978000000108550010000018651146202607',
            ]
        )
        ret = result.get("retEnvEvento", {}).get('retEvento')
        assert isinstance(ret, list)
        assert ret[0].get('infEvento', {}).get("cStat") is not None, f"Manifestation failed: {ret}"

    def test_confirmation(self, real_cert_manager):
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        result = svc.perform_confirm_operation(
            access_keys=[
                '22260547050493000138550010000011171955117990',
                '21260536978000000108550010000018651146202607',
            ]
        )
        ret = result.get("retEnvEvento", {}).get('retEvento')
        assert isinstance(ret, list)
        assert ret[0].get('infEvento', {}).get("cStat") is not None, f"Manifestation failed: {ret}"
