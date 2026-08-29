"""MDF-e (Manifesto Eletrônico de Documentos Fiscais) service facade."""

from __future__ import annotations

from typing import Any

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.endpoints import get_authorizer
from py_dfe.exceptions import InvalidSefazResponseError
from py_dfe.services.base import SefazClient, _ensure_list

_RESPONSE_NODE_PATH: dict[tuple[str, str], list[str]] = {
    ("SVRS", "MDFeDistribuicaoDFe"): [
        "mdfeDistDFeInteresseResult",
    ],
    ("SVRS", "MDFeRecepcaoSinc"): [
        "mdfeRecepcaoResult",
    ],
    ("SVRS", "MDFeRecepcaoEvento"): [
        "mdfeRecepcaoEventoResult",
    ],
}

_ENSURE_LIST_PATHS = {}


class MDFeServiceClient:
    """High-level facade for MDF-e web services (all states → SVRS)."""

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
            doc_type="mdfe",
            uf=uf,
            environment=environment,
            cert_manager=cert_manager,
            validate_schema=validate_schema,
            max_retries=max_retries,
            cnpj=cnpj,
        )

    @classmethod
    def _parse_result_message(
        cls,
        payload: dict[str, Any],
        *,
        authorizer: str | None = None,
        service: str | None = None,
    ):
        xml = payload.get("@xml")
        node_path = _RESPONSE_NODE_PATH.get((authorizer, service), ["mdfeResultMsg"])
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

    def status_service(self) -> dict[str, Any]:
        payload = {
            "consStatServMDFe": {
                "@versao": "3.00",
                "@xmlns": "http://www.portalfiscal.inf.br/mdfe",
                "tpAmb": self._client.env_type_str(),
                "xServ": "STATUS",
            }
        }
        return self._client.call("MDFeStatusServico", payload)

    def recepcao_sinc(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._client.call("MDFeRecepcaoSinc", payload)

    def consultar(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._client.call("MDFeConsulta", payload)

    def cons_nao_enc(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._client.call("MDFeConsNaoEnc", payload)

    def distribuicao_dfe(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._client.call("MDFeDistribuicaoDFe", payload)

    def perform_event(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._client.call("MDFeRecepcaoEvento", payload)
