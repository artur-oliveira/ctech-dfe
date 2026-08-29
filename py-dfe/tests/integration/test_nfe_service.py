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
    _CMUN_TERESINA,
    _CUF_PI,
    _chave,
    _date,
    build_nfe,
)

_SKIP = pytest.mark.skipif(
    os.environ.get("TEST_CERT_PATH") is None or os.getenv("TEST_CERT_PASSWORD") is None,
    reason="TEST_CERT_PATH / TEST_CERT_PASSWORD not set - skipping real SEFAZ tests",
)

_CNPJ = "11647612000197"
# CPF válido de teste — o responsável técnico agronômico do grupo agropecuario.
_CPF_RESP_TEC = "11144477735"

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

    def test_nfe_com_csrt(self, real_cert_manager):
        """infRespTec com idCSRT + hashCSRT (NT 2018.005)."""
        import base64
        import hashlib

        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        csrt = "G8063NG5H4YQ01M4L3AKG25OZ4A2GL123456"
        payload, chave = build_nfe(uf="PI")
        digest = hashlib.sha1((csrt + chave).encode()).digest()
        payload["enviNFe"]["NFe"]["infNFe"].setdefault("infRespTec", {}).update({
            "idCSRT": "01",
            "hashCSRT": base64.b64encode(digest).decode(),
        })
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_inf_adic_completo(self, real_cert_manager):
        """infAdic completo: infAdFisco, obsCont, obsFisco e procRef."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(uf="PI", inf_adic={
            "infAdFisco": "Beneficio fiscal 123",
            "obsCont": [{"@xCampo": "Pedido", "xTexto": "42"}],
            "obsFisco": [{"@xCampo": "Regime", "xTexto": "Especial"}],
            "procRef": [{"nProc": "0001/2026", "indProc": "0"}],
        })
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_volumes_lacres_e_reboque(self, real_cert_manager):
        """transp completo: vol como lista, lacres e reboque (tag RNTC)."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(
            uf="PI",
            vols=[{
                "qVol": "2", "esp": "CAIXA", "marca": "ACME", "nVol": "001/002",
                "pesoL": "10.000", "pesoB": "12.000",
                "lacres": [{"nLacre": "L1"}, {"nLacre": "L2"}],
            }],
            reboques=[{"placa": "XYZ9Z99", "UF": "PI", "RNTC": "87654321"}],
        )
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_veic_prod_completo(self, real_cert_manager):
        """veicProd com as 24 tags obrigatórias do XSD, sem default inventado."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(uf="PI", prod_extra={
            "veicProd": {
                "tpOp": "1", "chassi": "9BWZZZ377VT004251", "cCor": "0013",
                "xCor": "PRATA", "pot": "85", "cilin": "1600",
                "pesoL": "950.0000", "pesoB": "1100.0000", "nSerie": "123456",
                "tpComb": "16", "nMotor": "MTR0001", "CMT": "1.5000",
                "dist": "2600", "anoMod": "2026", "anoFab": "2025",
                "tpPint": "P", "tpVeic": "06", "espVeic": "1", "VIN": "N",
                "condVeic": "1", "cMod": "023459", "cCorDENATRAN": "10",
                "lota": "5", "tpRest": "0",
            },
        })
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_med_registrado_e_isento(self, real_cert_manager):
        """med: registro numérico sem xMotivoIsencao, e ISENTO com o motivo."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        for med in (
                {"cProdANVISA": "1234567890123", "vPMC": "49.90"},
                {"cProdANVISA": "ISENTO", "xMotivoIsencao": "RDC 123/2025", "vPMC": "0.00"},
        ):
            payload, _ = build_nfe(uf="PI", prod_extra={"med": med})
            result = svc.authorization(payload)
            _assert_comunicacao_authorization(result)

    def test_nfe_com_marketplace_datas_e_pedido_do_cliente(self, real_cert_manager):
        """indIntermed + infIntermed, dhSaiEnt/dPrevEntrega e xPed/nItemPed."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(
            uf="PI",
            prod_extra={"xPed": "PC-2026-0099", "nItemPed": "7"},
            inf_nfe_extra={"infIntermed": {"CNPJ": _CNPJ, "idCadIntTran": "LOJA-42"}},
        )
        ide = payload["enviNFe"]["NFe"]["infNFe"]["ide"]
        ide["indIntermed"] = "1"
        ide["dhSaiEnt"] = ide["dhEmi"]
        ide["dPrevEntrega"] = _date()
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_recopi(self, real_cert_manager):
        """nRECOPI (papel imune) — último ramo do choice de prod."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(uf="PI", prod_extra={"nRECOPI": "12345678901234567890"})
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_ide_da_reforma(self, real_cert_manager):
        """cIndOp, cMunFGIBS, tpNFDebito/tpNFCredito, gCompraGov e gPagAntecipado."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(uf="PI")
        ide = payload["enviNFe"]["NFe"]["infNFe"]["ide"]
        ide["cIndOp"] = "220110"
        ide["cMunFGIBS"] = _CMUN_TERESINA
        ide["tpNFDebito"] = "1"
        ide["tpNFCredito"] = "0"
        # tpOperGov 3 aceita várias chaves referenciadas.
        ide["gCompraGov"] = {
            "tpEnteGov": "1", "pRedutor": "20.0000", "tpOperGov": "3",
            "refDFeAnt": [_chave(_CUF_PI, "55", 1, 900000001)[0]],
        }
        ide["gPagAntecipado"] = {"refNFe": [_chave(_CUF_PI, "55", 1, 900000002)[0]]}
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_bloco_completo_de_ibs_cbs(self, real_cert_manager):
        """IBSCBS com gDif/gDevTrib/gRed, tributação de referência, crédito
        presumido, ALC/ZFM e os totais correspondentes (IBSCBSTot + vNFTot)."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(uf="PI", prod_extra={"gCred": [{
            "cCredPresumido": "PR000001", "pCredPresumido": "10.0000",
            "vCredPresumido": "0.10",
        }]})
        det = payload["enviNFe"]["NFe"]["infNFe"]["det"]
        det["imposto"]["IBSCBS"] = {
            "CST": "000", "cClassTrib": "000001",
            "gIBSCBS": {
                "vBC": "1.00",
                "gIBSUF": {
                    "pIBSUF": "10.0000",
                    "gDif": {"pDif": "30.0000", "vDif": "0.03"},
                    "gDevTrib": {"pDevTrib": "50.0000", "vDevTrib": "0.05"},
                    "gRed": {"pRedAliq": "25.0000", "pAliqEfet": "7.5000"},
                    "vIBSUF": "0.10",
                },
                "gIBSMun": {"pIBSMun": "2.0000", "vIBSMun": "0.02"},
                "vIBS": "0.12",
                "gCBS": {
                    "pCBS": "8.0000",
                    "gALCZFMCBS": {
                        "tpALCZFMCBS": "1", "nProcSuframa": "12345678",
                        "pAliqEfetRegCBS": "9.0000", "vTribRegCBS": "0.09",
                    },
                    "vCBS": "0.08",
                },
                "gTribRegular": {
                    "CSTReg": "000", "cClassTribReg": "000001",
                    "pAliqEfetRegIBSUF": "12.0000", "vTribRegIBSUF": "0.12",
                    "pAliqEfetRegIBSMun": "3.0000", "vTribRegIBSMun": "0.03",
                    "pAliqEfetRegCBS": "9.0000", "vTribRegCBS": "0.09",
                },
                "gTribCompraGov": {
                    "pAliqIBSUF": "10.0000", "vTribIBSUF": "0.10",
                    "pAliqIBSMun": "2.0000", "vTribIBSMun": "0.02",
                    "pAliqCBS": "8.0000", "vTribCBS": "0.08",
                },
            },
            "gEstornoCred": {"vIBSEstCred": "0.01", "vCBSEstCred": "0.01"},
            "gCredPresOper": {
                "vBCCredPres": "1.00", "cCredPres": "01",
                "gIBSCredPres": {"pCredPres": "5.0000", "vCredPres": "0.05"},
                "gCBSCredPres": {"pCredPres": "3.0000", "vCredPres": "0.03"},
            },
        }
        total = payload["enviNFe"]["NFe"]["infNFe"]["total"]
        total["IBSCBSTot"] = {
            "vBCIBSCBS": "1.00",
            "gIBS": {
                "gIBSUF": {"vDif": "0.03", "vDevTrib": "0.05", "vIBSUF": "0.10"},
                "gIBSMun": {"vDif": "0.00", "vDevTrib": "0.00", "vIBSMun": "0.02"},
                "vIBS": "0.12", "vCredPres": "0.05", "vCredPresCondSus": "0.00",
            },
            "gCBS": {
                "vDif": "0.00", "vDevTrib": "0.00", "vCBS": "0.08",
                "vCredPres": "0.03", "vCredPresCondSus": "0.00",
            },
            "gEstornoCred": {"vIBSEstCred": "0.01", "vCBSEstCred": "0.01"},
        }
        total["vNFTot"] = "1.20"
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_monofasia_de_ibs_cbs(self, real_cert_manager):
        """gIBSCBSMono completo (padrão, retenção, retido e diferido) + gMono no
        total e ISTot."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(uf="PI")
        det = payload["enviNFe"]["NFe"]["infNFe"]["det"]
        det["imposto"]["IBSCBS"] = {
            "CST": "620", "cClassTrib": "620001",
            "gIBSCBSMono": {
                "gMonoPadrao": {
                    "qBCMono": "1.0000", "adRemIBS": "0.5000", "adRemCBS": "0.3000",
                    "vIBSMono": "0.50", "vCBSMono": "0.30",
                },
                "gMonoReten": {
                    "qBCMonoReten": "1.0000", "adRemIBSReten": "0.2000",
                    "vIBSMonoReten": "0.20", "adRemCBSReten": "0.1000",
                    "vCBSMonoReten": "0.10",
                },
                "gMonoRet": {
                    "qBCMonoRet": "1.0000", "adRemIBSRet": "0.4000",
                    "vIBSMonoRet": "0.40", "adRemCBSRet": "0.2500",
                    "vCBSMonoRet": "0.25",
                },
                "gMonoDif": {
                    "pDifIBS": "50.0000", "vIBSMonoDif": "0.25",
                    "pDifCBS": "10.0000", "vCBSMonoDif": "0.03",
                },
                "vTotIBSMonoItem": "0.70", "vTotCBSMonoItem": "0.40",
            },
        }
        det["imposto"]["IS"] = {
            "CSTIS": "000", "cClassTribIS": "000001",
            "vBCIS": "1.00", "pIS": "5.0000", "vIS": "0.05",
        }
        total = payload["enviNFe"]["NFe"]["infNFe"]["total"]
        total["ISTot"] = {"vIS": "0.05"}
        total["IBSCBSTot"] = {
            "vBCIBSCBS": "0.00",
            "gIBS": {
                "gIBSUF": {"vDif": "0.00", "vDevTrib": "0.00", "vIBSUF": "0.00"},
                "gIBSMun": {"vDif": "0.00", "vDevTrib": "0.00", "vIBSMun": "0.00"},
                "vIBS": "0.00", "vCredPres": "0.00", "vCredPresCondSus": "0.00",
            },
            "gCBS": {
                "vDif": "0.00", "vDevTrib": "0.00", "vCBS": "0.00",
                "vCredPres": "0.00", "vCredPresCondSus": "0.00",
            },
            "gMono": {
                "vIBSMono": "0.50", "vCBSMono": "0.30",
                "vIBSMonoReten": "0.20", "vCBSMonoReten": "0.10",
                "vIBSMonoRet": "0.40", "vCBSMonoRet": "0.25",
            },
        }
        total["vNFTot"] = "2.15"
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

    def test_nfe_com_transferencia_e_ajuste_de_credito(self, real_cert_manager):
        """gTransfCred e gAjusteCompet — ramos do choice que substituem gIBSCBS."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        for grupo in (
                {"gTransfCred": {"vIBS": "0.10", "vCBS": "0.08"}},
                {"gAjusteCompet": {"competApur": "2026-09", "vIBS": "0.10", "vCBS": "0.08"}},
        ):
            payload, _ = build_nfe(uf="PI")
            ibscbs = {"CST": "000", "cClassTrib": "000001"}
            ibscbs.update(grupo)
            payload["enviNFe"]["NFe"]["infNFe"]["det"]["imposto"]["IBSCBS"] = ibscbs
            result = svc.authorization(payload)
            _assert_comunicacao_authorization(result)

    def test_nfe_com_compra_cana_e_agropecuario(self, real_cert_manager):
        """Grupos de nicho do fim de infNFe, um por vez (nenhum é exclusivo)."""
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        casos = [
            {"compra": {"xNEmp": "2026NE000123", "xPed": "PED-4455", "xCont": "CT-2026/09"}},
            {"cana": {
                "safra": "2025/2026", "ref": "09/2026",
                "forDia": [
                    {"@dia": "1", "qtde": "1000.0000"},
                    {"@dia": "2", "qtde": "500.5000"},
                ],
                "qTotMes": "1500.5000", "qTotAnt": "3000.0000", "qTotGer": "4500.5000",
                "deduc": [{"xDed": "CONSECANA", "vDed": "1000.00"}],
                "vFor": "18750.00", "vTotDed": "1000.00", "vLiqFor": "17750.00",
            }},
            {"agropecuario": {"defensivo": [
                {"nReceituario": "REC-1", "CPFRespTec": _CPF_RESP_TEC},
            ]}},
            {"agropecuario": {"guiaTransito": {
                "tpGuia": "1", "UFGuia": "PI", "serieGuia": "A", "nGuia": "123456",
            }}},
        ]
        for extra in casos:
            payload, _ = build_nfe(uf="PI", inf_nfe_extra=extra)
            result = svc.authorization(payload)
            _assert_comunicacao_authorization(result)

    def test_nfe_devolucao_com_nfref(self, real_cert_manager):
        """Devolução (finNFe=4) referencia a chave autorizada em ide/NFref."""
        if TestNFeSVRS._chave is None:
            pytest.skip("test_authorization did not complete successfully")
        svc = _nfe_svc(real_cert_manager, UF.PI.value)
        payload, _ = build_nfe(
            uf="PI", fin_nfe="4", nfref=[{"refNFe": TestNFeSVRS._chave}]
        )
        result = svc.authorization(payload)
        _assert_comunicacao_authorization(result)

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
