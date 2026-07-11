"""Unit tests for endpoint resolution."""

import pytest

from py_dfe.constants.endpoints import get_endpoint, list_services


class TestGetEndpoint:
    def test_nfe_sp_producao_autorizacao(self):
        url = get_endpoint("nfe", "SP", "prod", "NFeAutorizacao")
        assert "nfe.fazenda.sp.gov.br" in url
        assert "nfeautorizacao4" in url.lower()

    def test_nfe_sp_homologacao_autorizacao(self):
        url = get_endpoint("nfe", "SP", "hom", "NFeAutorizacao")
        assert "hom" in url

    def test_nfe_am_producao(self):
        url = get_endpoint("nfe", "AM", "prod", "NFeAutorizacao")
        assert "nfe.sefaz.am.gov.br" in url

    def test_nfe_rs_producao_cad_svrs(self):
        url = get_endpoint("nfe", "RS", "prod", "NfeConsultaCadastro")
        assert "cad.svrs.rs.gov.br" in url

    def test_nfe_svrs_state_rj(self):
        url = get_endpoint("nfe", "RJ", "prod", "NFeAutorizacao")
        assert "svrs.rs.gov.br" in url

    def test_nfe_svrs_state_sc(self):
        url = get_endpoint("nfe", "SC", "prod", "NFeAutorizacao")
        assert "svrs.rs.gov.br" in url

    def test_nfce_am_producao(self):
        url = get_endpoint("nfce", "AM", "prod", "NFeAutorizacao")
        assert "nfce.sefaz.am.gov.br" in url

    def test_nfce_svrs_state(self):
        url = get_endpoint("nfce", "RJ", "prod", "NFeAutorizacao")
        assert "svrs.rs.gov.br" in url

    def test_cte_mg_producao(self):
        url = get_endpoint("cte", "MG", "prod", "CTeRecepcaoSinc")
        assert "cte.fazenda.mg.gov.br" in url

    def test_cte_svrs_state(self):
        url = get_endpoint("cte", "GO", "prod", "CTeRecepcaoSinc")
        assert "svrs.rs.gov.br" in url

    def test_mdfe_all_states_svrs(self):
        for uf in ["SP", "RJ", "RS", "MG", "GO"]:
            url = get_endpoint("mdfe", uf, "prod", "MDFeRecepcaoSinc")
            assert "mdfe.svrs.rs.gov.br" in url

    def test_nfe_an_distribuicao(self):
        url = get_endpoint("nfe", "AN", "prod", "NFeDistribuicaoDFe")
        assert "nfe.fazenda.gov.br" in url

    def test_cte_an_distribuicao(self):
        url = get_endpoint("cte", "AN", "prod", "CTeDistribuicaoDFe")
        assert "cte.fazenda.gov.br" in url

    def test_unknown_service_raises(self):
        with pytest.raises(KeyError):
            get_endpoint("nfe", "SP", "prod", "NonExistentService")

    def test_unknown_doc_type_raises(self):
        with pytest.raises(KeyError):
            get_endpoint("invalid", "SP", "prod", "NFeAutorizacao")


class TestListServices:
    def test_nfe_sp_has_all_services(self):
        services = list_services("nfe", "SP", "prod")
        assert "NFeAutorizacao" in services
        assert "NFeRetAutorizacao" in services
        assert "NfeStatusServico" in services

    def test_mdfe_has_recepcao_sinc(self):
        services = list_services("mdfe", "SP", "prod")
        assert "MDFeRecepcaoSinc" in services
