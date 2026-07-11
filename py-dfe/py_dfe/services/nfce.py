"""NFC-e (Nota Fiscal de Consumidor Eletrônica) service facade."""

from __future__ import annotations

from py_dfe.certificate.manager import CertificateManager
from py_dfe.services._nf import _NFServiceClient


class NFCeServiceClient(_NFServiceClient):
    """High-level facade for all NF-e web services.

    Example
    -------
    >>> cert_manager = CertificateManager()
    >>> svc = NFCeServiceClient(cert_manager, uf="PI", environment="homologacao")
    >>> status_result = svc.status_service()
    """

    def __init__(
            self,
            cert_manager: CertificateManager,
            uf: str,
            environment: str,
            validate_schema: bool = True,
            max_retries: int = 3,
            cnpj=None,
    ) -> None:
        super().__init__(
            cert_manager, uf, environment, 'nfce', validate_schema=validate_schema,
            max_retries=max_retries, cnpj=cnpj,
        )
