"""Service factory - single import point."""

from py_dfe.certificate.manager import CertificateManager
from py_dfe.services.cte import CTeServiceClient
from py_dfe.services.mdfe import MDFeServiceClient
from py_dfe.services.nfce import NFCeServiceClient
from py_dfe.services.nfe import NFeServiceClient

_FACTORIES = {
    "nfe": NFeServiceClient,
    "nfce": NFCeServiceClient,
    "cte": CTeServiceClient,
    "mdfe": MDFeServiceClient,
}


def create_service(
        doc_type: str,
        cert_manager: CertificateManager,
        uf: str,
        environment: str,
        cnpj: str = None,
        **kwargs,
):
    """Factory function - return the correct service instance for *doc_type*."""
    cls = _FACTORIES.get(doc_type)
    if cls is None:
        raise ValueError(f"Unknown doc_type {doc_type!r}. Valid: {list(_FACTORIES)}")
    return cls(cert_manager, uf, environment, cnpj=cnpj, **kwargs)
