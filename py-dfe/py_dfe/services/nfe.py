"""NF-e (Nota Fiscal Eletrônica) service facade."""

from __future__ import annotations

import random
from typing import Any

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.enums import TP_EVENTO_CANCELAMENTO, TP_EVENTO_CIENCIA_OPERACAO, TP_EVENTO_CONFIRMACAO_OPERACAO
from py_dfe.services._nf import _NFServiceClient
from py_dfe.services.base import _ensure_list
from py_dfe.utils.date import dh


class NFeServiceClient(_NFServiceClient):
    """High-level facade for all NF-e web services.

    Example
    -------
    >>> cert_manager = CertificateManager()
    >>> svc = NFeServiceClient(cert_manager, uf="PI", environment="homologacao")
    >>> status_result = svc.status_service()
    """

    def __init__(
            self,
            cert_manager: CertificateManager,
            uf: str,
            environment: str,
            *,
            validate_schema: bool = True,
            max_retries: int = 3,
            cnpj=None,
    ) -> None:
        super().__init__(
            cert_manager, uf, environment, 'nfe',
            validate_schema=validate_schema,
            max_retries=max_retries,
            cnpj=cnpj,
        )

    def perform_manifestation(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._parse_an_event_response(self._client.for_uf('AN').call("RecepcaoEvento", payload))

    def perform_science_operation(
            self, access_keys, batch_id=None
    ) -> dict[str, Any]:
        if batch_id is None:
            batch_id = str(random.randint(1, 999_999_999_999_999))
        return self.perform_manifestation({
            "envEvento": {
                "@versao": "1.00",
                "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                "idLote": batch_id,
                "evento": [{
                    "@versao": "1.00",
                    "infEvento": {
                        "@Id": f"ID{TP_EVENTO_CIENCIA_OPERACAO}{access_key}{int(1):02d}",
                        "cOrgao": '91',
                        "tpAmb": self._client.env_type_str(),
                        "CNPJ": self._client.cnpj,
                        "chNFe": access_key,
                        "dhEvento": dh(),
                        "tpEvento": TP_EVENTO_CIENCIA_OPERACAO,
                        "nSeqEvento": str(1),
                        "verEvento": "1.00",
                        "detEvento": {
                            "@versao": "1.00",
                            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                            "descEvento": "Ciencia da Operacao",
                        },
                    },
                } for access_key in access_keys],
            }
        })

    def perform_confirm_operation(
            self, access_keys, batch_id=None
    ) -> dict[str, Any]:
        if batch_id is None:
            batch_id = str(random.randint(1, 999_999_999_999_999))
        return self.perform_manifestation({
            "envEvento": {
                "@versao": "1.00",
                "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                "idLote": batch_id,
                "evento": [{
                    "@versao": "1.00",
                    "infEvento": {
                        "@Id": f"ID{TP_EVENTO_CONFIRMACAO_OPERACAO}{access_key}{int(1):02d}",
                        "cOrgao": '91',
                        "tpAmb": self._client.env_type_str(),
                        "CNPJ": self._client.cnpj,
                        "chNFe": access_key,
                        "dhEvento": dh(),
                        "tpEvento": TP_EVENTO_CONFIRMACAO_OPERACAO,
                        "nSeqEvento": str(1),
                        "verEvento": "1.00",
                        "detEvento": {
                            "@versao": "1.00",
                            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                            "descEvento": "Confirmacao da Operacao",
                        },
                    },
                } for access_key in access_keys],
            }
        })

    def perform_distribution(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._parse_distribution_response(self._client.for_uf('AN').call("NFeDistribuicaoDFe", payload))

    def perform_distribution_last_nsu(self, nsu) -> dict[str, Any]:
        return self.perform_distribution({
            'distDFeInt': {
                '@versao': '1.01',
                '@xmlns': 'http://www.portalfiscal.inf.br/nfe',
                'tpAmb': self._client.env_type_str(),
                'cUFAutor': self._client.uf_code,
                'CNPJ': self._client.cnpj,
                'distNSU': {
                    'ultNSU': str(nsu).zfill(15)
                }
            }
        })

    def perform_distribution_query_nsu(self, nsu) -> dict[str, Any]:
        return self.perform_distribution({
            'distDFeInt': {
                '@versao': '1.01',
                '@xmlns': 'http://www.portalfiscal.inf.br/nfe',
                'tpAmb': self._client.env_type_str(),
                'cUFAutor': self._client.uf_code,
                'CNPJ': self._client.cnpj,
                'consNSU': {
                    'NSU': str(nsu).zfill(15)
                }
            }
        })

    def perform_distribution_access_key(self, access_key) -> dict[str, Any]:
        return self.perform_distribution({
            'distDFeInt': {
                '@versao': '1.01',
                '@xmlns': 'http://www.portalfiscal.inf.br/nfe',
                'tpAmb': self._client.env_type_str(),
                'cUFAutor': self._client.uf_code,
                'CNPJ': self._client.cnpj,
                'consChNFe': {
                    'chNFe': access_key
                }
            }
        })

    @classmethod
    def _parse_distribution_response(cls, res):
        return _ensure_list(
            res.get('nfeDistDFeInteresseResponse', {}).get('nfeDistDFeInteresseResult', {}),
            'retDistDFeInt/loteDistDFeInt/docZip'
        )

    @classmethod
    def _parse_an_event_response(cls, res):
        return _ensure_list(res.get('nfeRecepcaoEventoNFResult', {}), 'retEnvEvento/retEvento')
