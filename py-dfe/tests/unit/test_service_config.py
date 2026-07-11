"""Unit tests for ServiceConfig (Strategy pattern)."""

import pytest

from py_dfe.services.config import get_config, NFE_CONFIG, CTE_CONFIG, MDFE_CONFIG


class TestServiceConfig:
    def test_nfe_requires_signature_autorizacao(self):
        assert NFE_CONFIG.requires_signature("NFeAutorizacao")

    def test_nfe_does_not_require_signature_status(self):
        assert not NFE_CONFIG.requires_signature("NfeStatusServico")

    def test_cte_requires_signature_recepcao_sinc(self):
        assert CTE_CONFIG.requires_signature("CTeRecepcaoSinc")

    def test_mdfe_requires_signature_recepcao_sinc(self):
        assert MDFE_CONFIG.requires_signature("MDFeRecepcaoSinc")

    def test_nfe_requires_validation_autorizacao(self):
        assert NFE_CONFIG.requires_validation("NFeAutorizacao")

    def test_nfe_does_not_require_validation_status(self):
        assert not NFE_CONFIG.requires_validation("NfeStatusServico")

    def test_get_config_nfe(self):
        cfg = get_config("nfe")
        assert cfg.doc_type.value == "nfe"

    def test_get_config_cte(self):
        cfg = get_config("cte")
        assert cfg.schema_version == "4.00"

    def test_get_config_mdfe(self):
        cfg = get_config("mdfe")
        assert cfg.schema_version == "3.00"

    def test_get_config_unknown_raises(self):
        with pytest.raises(ValueError, match="Unknown doc_type"):
            get_config("boleto")

    def test_config_is_immutable(self):
        with pytest.raises(Exception):
            NFE_CONFIG.schema_version = "9.99"
