import datetime
import random
from typing import Any

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.endpoints import get_authorizer
from py_dfe.constants.enums import TP_EVENTO_CANCELAMENTO
from py_dfe.exceptions import InvalidSefazResponseError, DFeError
from py_dfe.services.base import SefazClient, _ensure_list
from py_dfe.utils.access_key import parse_uf, parse_cnpj
from py_dfe.utils.date import dh

# Ordered list of nodes to traverse to reach the actual payload.
# Default (no entry) is ["nfeResultMsg"]. Each entry fully replaces the default.
# MG skips nfeResultMsg entirely; MT has an extra inner node after nfeResultMsg.
_RESPONSE_NODE_PATH: dict[tuple[str, str], list[str]] = {
    ("MG", "NfeConsultaCadastro"): ["consultaCadastro4Result"],
    ("AM", "NfeConsultaCadastro"): ["consultaCadastro2Result"],
    ("MT", "NfeConsultaCadastro"): ["nfeResultMsg", "consultaCadastroResult"],
    ("AN", "NFeDistribuicaoDFe"): [
        "nfeDistDFeInteresseResponse",
        "nfeDistDFeInteresseResult",
    ],
    ("AN", "RecepcaoEvento"): ["nfeRecepcaoEventoNFResult"],
}

_INCLUDE_HEADERS: dict[tuple[str, str], bool] = {
    ("MT", "NfeConsultaCadastro"): True,
}

# Generic _ensure_list paths applied automatically after unwrapping the result node.
# Eliminates the need for per-method _parse_*_result boilerplate.
_ENSURE_LIST_PATHS: dict[str, list[str]] = {
    "NFeAutorizacao": ["retEnviNFe/protNFe"],
    "NfeConsultaCadastro": ["retConsCad/infCons/infCad"],
    "RecepcaoEvento": ["retEnvEvento/retEvento"],
}


class _NFServiceClient:
    def __init__(
        self,
        cert_manager: CertificateManager,
        uf: str,
        environment: str,
        doc_type,
        *,
        validate_schema: bool = True,
        max_retries: int = 3,
        cnpj: str | None = None,
    ) -> None:
        self._client = SefazClient(
            doc_type=doc_type,
            uf=uf,
            environment=environment,
            cert_manager=cert_manager,
            validate_schema=validate_schema,
            max_retries=max_retries,
            cnpj=cnpj,
        )

    def call(self, service: str, payload: dict[str, Any]) -> dict[str, Any]:
        """Generic call for any NF-e service."""
        authorizer = get_authorizer(self._client.doc_type, self._client.uf)
        result = self._parse_result_message(
            self._client.call(
                service,
                payload,
                _INCLUDE_HEADERS.get(
                    (
                        authorizer,
                        service,
                    )
                )
                or False,
            ),
            authorizer=authorizer,
            service=service,
        )
        for path in _ENSURE_LIST_PATHS.get(service, []):
            _ensure_list(result, path)
        return result

    def query_cnpj(self, cnpj: str, uf: str | None = None) -> dict[str, Any]:
        return self.query_register(
            {
                "ConsCad": {
                    "@versao": "2.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                    "infCons": {
                        "xServ": "CONS-CAD",
                        "UF": uf if uf else self._client.uf,
                        "CNPJ": cnpj,
                    },
                }
            }
        )

    def status_service(self, uf: str | None = None) -> dict[str, Any]:
        if uf is None:
            uf = self._client.uf_code
        return self.call(
            "NfeStatusServico",
            {
                "consStatServ": {
                    "@versao": "4.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                    "tpAmb": self._client.env_type_str(),
                    "cUF": uf,
                    "xServ": "STATUS",
                }
            },
        )

    def authorization(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("NFeAutorizacao", payload)

    def return_authorization(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("NFeRetAutorizacao", payload)

    def query_protocol(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("NfeConsultaProtocolo", payload)

    def query_protocol_nf(self, access_key: str) -> dict[str, Any]:
        return self.query_protocol(
            {
                "consSitNFe": {
                    "@versao": "4.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                    "tpAmb": self._client.env_type_str(),
                    "xServ": "CONSULTAR",
                    "chNFe": access_key,
                }
            }
        )

    def number_inutilization(
        self,
        serie_number,
        start_number,
        justification=None,
        end_number=None,
        year=None,
        cnpj=None,
    ) -> dict[str, Any]:
        if year is None:
            year = datetime.date.today().year
        if end_number is None:
            end_number = start_number

        year = str(year)[2:4]

        cnpj = cnpj or self._client.cnpj
        if not cnpj:
            raise DFeError(400, "missing_cnpj", "CNPJ cannot be empty")

        id_str = (
            f"ID{self._client.uf_code}{year}{self._client.cnpj}{self._client.doc_type_code}"
            f"{serie_number:03d}{int(start_number):09d}{int(end_number):09d}"
        )
        return self.inutilization(
            {
                "inutNFe": {
                    "@versao": "4.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                    "infInut": {
                        "@Id": id_str,
                        "tpAmb": self._client.env_type_str(),
                        "xServ": "INUTILIZAR",
                        "cUF": self._client.uf_code,
                        "ano": str(year),
                        "CNPJ": cnpj,
                        "mod": self._client.doc_type_code,
                        "serie": str(serie_number),
                        "nNFIni": str(start_number),
                        "nNFFin": str(end_number),
                        "xJust": justification,
                    },
                }
            }
        )

    def inutilization(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("NfeInutilizacao", payload)

    def query_register(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self.call("NfeConsultaCadastro", payload)

    def cancel_nfe(
        self,
        access_key: str,
        justification: str,
        event_sequence_number: str,
        protocol_number: str,
        batch_id: str | None = None,
    ):
        if batch_id is None:
            batch_id = str(random.randint(1, 999_999_999_999_999))
        return self.perform_event(
            {
                "envEvento": {
                    "@versao": "1.00",
                    "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                    "idLote": batch_id,
                    "evento": {
                        "@versao": "1.00",
                        "infEvento": {
                            "@Id": f"ID{TP_EVENTO_CANCELAMENTO}{access_key}{int(event_sequence_number):02d}",
                            "cOrgao": parse_uf(access_key),
                            "tpAmb": self._client.env_type_str(),
                            "CNPJ": parse_cnpj(access_key),
                            "chNFe": access_key,
                            "dhEvento": dh(),
                            "tpEvento": TP_EVENTO_CANCELAMENTO,
                            "nSeqEvento": str(event_sequence_number),
                            "verEvento": "1.00",
                            "detEvento": {
                                "@versao": "1.00",
                                "@xmlns": "http://www.portalfiscal.inf.br/nfe",
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
        return self.call("RecepcaoEvento", payload)

    @classmethod
    def _parse_result_message(
        cls,
        payload: dict[str, Any],
        *,
        authorizer: str | None = None,
        service: str | None = None,
    ) -> dict[str, Any]:
        xml = payload.get("@xml")
        node_path = _RESPONSE_NODE_PATH.get((authorizer, service), ["nfeResultMsg"])
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
