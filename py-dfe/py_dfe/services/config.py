"""Service configuration - Strategy pattern for each document type."""

from __future__ import annotations

from dataclasses import dataclass, field

from py_dfe.constants.enums import (
    CTE_WSDL_SERVICE,
    MDFE_WSDL_SERVICE,
    NFE_WSDL_SERVICE,
    DocType,
)


@dataclass(frozen=True)
class ServiceConfig:
    """Immutable configuration for a specific DFe document type.

    Encapsulates all metadata a service needs so the HTTP layer remains
    completely generic (Strategy pattern).
    """

    doc_type: DocType
    schema_version: str
    wsdl_services: dict[str, str]
    services_requiring_signature: frozenset[str]
    services_requiring_validation: frozenset[str]
    sign_id_xpath: dict[str, str] = field(default_factory=dict)

    def requires_signature(self, service: str) -> bool:
        return service in self.services_requiring_signature

    def requires_validation(self, service: str) -> bool:
        return service in self.services_requiring_validation

    def get_sign_xpath(self, service: str) -> str:
        return self.sign_id_xpath.get(service, "")


_NFE_NS = "http://www.portalfiscal.inf.br/nfe"
_CTE_NS = "http://www.portalfiscal.inf.br/cte"
_MDFE_NS = "http://www.portalfiscal.inf.br/mdfe"

NFE_CONFIG = ServiceConfig(
    doc_type=DocType.NFE,
    schema_version="4.00",
    wsdl_services=NFE_WSDL_SERVICE,
    services_requiring_signature=frozenset({
        "NFeAutorizacao", "NfeInutilizacao", "RecepcaoEvento",
    }),
    services_requiring_validation=frozenset({
        "NFeAutorizacao", "RecepcaoEvento",
    }),
    sign_id_xpath={
        "NFeAutorizacao": f".//{{{_NFE_NS}}}infNFe",
        "NfeInutilizacao": f".//{{{_NFE_NS}}}infInut",
        "RecepcaoEvento": f".//{{{_NFE_NS}}}infEvento",
    },
)

NFCE_CONFIG = ServiceConfig(
    doc_type=DocType.NFCE,
    schema_version="4.00",
    wsdl_services=NFE_WSDL_SERVICE,
    services_requiring_signature=frozenset({
        "NFeAutorizacao", "NfeInutilizacao", "RecepcaoEvento",
    }),
    services_requiring_validation=frozenset({
        "NFeAutorizacao", "NfeInutilizacao", "RecepcaoEvento",
    }),
    sign_id_xpath={
        "NFeAutorizacao": f".//{{{_NFE_NS}}}infNFe",
        "NfeInutilizacao": f".//{{{_NFE_NS}}}infInut",
        "RecepcaoEvento": f".//{{{_NFE_NS}}}infEvento",
    },
)

CTE_CONFIG = ServiceConfig(
    doc_type=DocType.CTE,
    schema_version="4.00",
    wsdl_services=CTE_WSDL_SERVICE,
    services_requiring_signature=frozenset({
        "CTeRecepcaoSinc", "CTeRecepcaoOS", "CTeRecepcaoGTVe",
        "CTeRecepcaoSimp", "CTeRecepcaoEvento",
    }),
    services_requiring_validation=frozenset({
        "CTeRecepcaoSinc", "CTeRecepcaoOS", "CTeRecepcaoGTVe",
        "CTeRecepcaoSimp", "CTeRecepcaoEvento",
    }),
    sign_id_xpath={
        "CTeRecepcaoSinc": f".//{{{_CTE_NS}}}infCte",
        "CTeRecepcaoOS": f".//{{{_CTE_NS}}}infCTeOS",
        "CTeRecepcaoGTVe": f".//{{{_CTE_NS}}}infGTVe",
        "CTeRecepcaoSimp": f".//{{{_CTE_NS}}}infCte",
        "CTeRecepcaoEvento": f".//{{{_CTE_NS}}}infEvento",
    },
)

MDFE_CONFIG = ServiceConfig(
    doc_type=DocType.MDFE,
    schema_version="3.00",
    wsdl_services=MDFE_WSDL_SERVICE,
    services_requiring_signature=frozenset({
        "MDFeRecepcaoSinc", "MDFeRecepcaoEvento",
    }),
    services_requiring_validation=frozenset({
        "MDFeRecepcaoSinc", "MDFeRecepcaoEvento",
    }),
    sign_id_xpath={
        "MDFeRecepcaoSinc": f".//{{{_MDFE_NS}}}infMDFe",
        "MDFeRecepcaoEvento": f".//{{{_MDFE_NS}}}infEvento",
    },
)

SERVICE_CONFIGS: dict[str, ServiceConfig] = {
    "nfe": NFE_CONFIG,
    "nfce": NFCE_CONFIG,
    "cte": CTE_CONFIG,
    "mdfe": MDFE_CONFIG,
}


def get_config(doc_type: str) -> ServiceConfig:
    """Return the ServiceConfig for the given doc_type string."""
    try:
        return SERVICE_CONFIGS[doc_type]
    except KeyError:
        raise ValueError(
            f"Unknown doc_type {doc_type!r}. Valid: {list(SERVICE_CONFIGS)}"
        )
