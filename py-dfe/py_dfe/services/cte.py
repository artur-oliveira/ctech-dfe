"""CT-e (Conhecimento de Transporte Eletrônico) service facade."""

from __future__ import annotations

from typing import Any

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.endpoints import get_authorizer
from py_dfe.constants.enums import TP_EVENTO_CANCELAMENTO
from py_dfe.exceptions import InvalidSefazResponseError
from py_dfe.services.base import SefazClient, _ensure_list
from py_dfe.utils.access_key import parse_cnpj, parse_uf
from py_dfe.utils.date import dh

_RESPONSE_NODE_PATH: dict[tuple[str, str], list[str]] = {
    ("AN", "CTeDistribuicaoDFe"): [
        "cteDistDFeInteresseResponse",
        "cteDistDFeInteresseResult",
    ],
}

_ENSURE_LIST_PATHS = {}


class CTeServiceClient:
    """High-level facade for CT-e web services."""

    def __init__(
        self,
        cert_manager: CertificateManager,
        uf: str,
        environment: str,
        *,
        validate_schema: bool = True,
        max_retries: int = 3,
        cnpj: str = None,
    ) -> None:
        self._client = SefazClient(
            doc_type="cte",
            uf=uf,
            environment=environment,
            cert_manager=cert_manager,
            validate_schema=validate_schema,
            max_retries=max_retries,
            cnpj=cnpj,
        )

    def call(self, service: str, payload: dict[str, Any]) -> dict[str, Any]:
        authorizer = get_authorizer(self._client.doc_type, self._client.uf)
        result = self._parse_result_message(
            self._client.call(
                service,
                payload,
            ),
            authorizer=authorizer,
            service=service,
        )
        for path in _ENSURE_LIST_PATHS.get(service, []):
            _ensure_list(result, path)
        return result

    def status_service(self, uf: str) -> dict[str, Any]:
        payload = {
            "consStatServCTe": {
                "@versao": "4.00",
                "@xmlns": "http://www.portalfiscal.inf.br/cte",
                "tpAmb": self._client.env_type_str(),
                "cUF": uf,
                "xServ": "STATUS",
            }
        }
        return self.call("CTeStatusServico", payload, "cteStatusServicoCTResult")

    def recepcao_sinc(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call(
            "CTeRecepcaoSinc", payload, "cteRecepcaoResult", "cteRecepcaoSincResult"
        )

    def recepcao_os(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("CTeRecepcaoOS", payload)

    def recepcao_gtve(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("CTeRecepcaoGTVe", payload)

    def recepcao_simp(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("CTeRecepcaoSimp", payload)

    def query_cte_by_access_key(self, access_key: str) -> dict[str, Any]:
        return self.query_cte(
            {
                "consSitCTe": {
                    "@versao": "4.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/cte",
                    "tpAmb": self._client.env_type_str(),
                    "xServ": "CONSULTAR",
                    "chCTe": access_key,
                }
            }
        )

    def query_cte(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("CTeConsulta", payload, "cteConsultaCTResult")

    def cancel_cte(
        self,
        access_key: str,
        justification: str,
        event_sequence_number: str,
        protocol_number: str,
    ):
        return self.perform_event(
            {
                "eventoCTe": {
                    "@versao": "4.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/cte",
                    "infEvento": {
                        "@Id": f"ID{TP_EVENTO_CANCELAMENTO}{access_key}{int(event_sequence_number):03d}",
                        "cOrgao": parse_uf(access_key),
                        "tpAmb": self._client.env_type_str(),
                        "CNPJ": parse_cnpj(access_key),
                        "chCTe": access_key,
                        "dhEvento": dh(),
                        "tpEvento": TP_EVENTO_CANCELAMENTO,
                        "nSeqEvento": str(event_sequence_number),
                        "detEvento": {
                            "@versaoEvento": "4.00",
                            "evCancCTe": {
                                "descEvento": "Cancelamento",
                                "nProt": protocol_number,
                                "xJust": justification,
                            },
                        },
                    },
                }
            }
        )

    def perform_event(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("CTeRecepcaoEvento", payload, "cteRecepcaoEventoResult")

    def distribuicao_dfe(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._client.for_uf("AN").call("CTeDistribuicaoDFe", payload)

    @classmethod
    def _parse_result_message(
        cls,
        payload: dict[str, Any],
        *,
        authorizer: str | None = None,
        service: str | None = None,
    ):
        xml = payload.get("@xml")
        node_path = _RESPONSE_NODE_PATH.get((authorizer, service), ["cteResultMsg"])
        inner = payload
        for node in node_path:
            if not isinstance(inner, dict) or node not in inner:
                raise InvalidSefazResponseError(
                    f"Expected node {node!r} not found in {inner}"
                )
            inner = inner[node]
        if xml is not None and isinstance(inner, dict):
            inner["@xml"] = xml
        return inner
