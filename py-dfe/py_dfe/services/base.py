"""Base SEFAZ service client - generic HTTP + retry + response parsing."""

from __future__ import annotations

import logging
import time
from typing import Any

import httpx
from lxml import etree

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.endpoints import get_endpoint
from py_dfe.constants.enums import Environment, DOC_TYPE_CODE, get_uf_ibge, UF_IBGE
from py_dfe.exceptions import RetryExhaustedError, SOAPError
from py_dfe.services.config import ServiceConfig, get_config
from py_dfe.soap.envelope import SOAPEnvelopeBuilder, extract_body
from py_dfe.xmlops.builder import parse_xml_bytes, to_xml_bytes
from py_dfe.xmlops.processor import build_processed_xml
from py_dfe.xmlops.signer import sign_xml
from py_dfe.xmlops.validator import validate

logger = logging.getLogger(__name__)

_GZIP_ENDPOINTS = frozenset(
    {
        "CTeRecepcaoSinc",
        "CTeRecepcaoOS",
        "CTeRecepcaoGTVe",
        "CTeRecepcaoSimp",
        "MDFeRecepcaoSinc",
    }
)

_RETRYABLE_HTTP = frozenset({503, 504, 502, 500})


class SefazClient:
    """Generic SEFAZ web service client.

    Handles:
    - JSON payload → XML
    - Optional XSD validation
    - Optional XML signing
    - SOAP envelope construction
    - mTLS HTTP POST via httpx
    - Retry with exponential back-off
    - Response extraction and XML → dict conversion

    Use one of the concrete factory methods (``for_nfe``, ``for_cte``, …) or
    instantiate directly with a ``ServiceConfig``.
    """

    MAX_RETRIES: int = 3
    TIMEOUT_CONNECT: float = 3.0
    TIMEOUT_READ: float = 15.0
    BACKOFF_BASE: float = 1.0

    def __init__(
        self,
        doc_type: str,
        uf: str,
        environment: str,
        cert_manager: CertificateManager,
        *,
        validate_schema: bool = True,
        max_retries: int = MAX_RETRIES,
        timeout: httpx.Timeout | None = None,
        cnpj: str = None,
        original_uf: str = None,
    ) -> None:
        self._doc_type = doc_type
        self._config: ServiceConfig = get_config(doc_type)
        self._uf = uf
        self._environment = environment
        self._cert = cert_manager
        self._validate = validate_schema
        self._max_retries = max_retries
        self._cnpj = cnpj
        self._timeout = timeout or httpx.Timeout(
            self.TIMEOUT_READ,
            connect=self.TIMEOUT_CONNECT,
        )
        self._original_uf = original_uf

    def for_uf(self, uf) -> "SefazClient":
        return SefazClient(
            doc_type=self._doc_type,
            uf=uf,
            environment=self.environment,
            cert_manager=self.cert_manager,
            validate_schema=self.validate_schema,
            max_retries=self.max_retries,
            timeout=self._timeout,
            original_uf=self.uf,
        )

    def is_production(self):
        return self._environment == Environment.PRODUCTION

    def env_type_str(self):
        return "1" if self.is_production() else "2"

    @property
    def uf(self):
        return self._uf

    @property
    def original_uf(self):
        return self._original_uf

    @property
    def doc_type(self):
        return self._doc_type

    @property
    def uf_code(self):
        return get_uf_ibge(self.uf)

    @property
    def original_uf_code(self):
        return get_uf_ibge(self.original_uf)

    @property
    def doc_type_code(self):
        return str(DOC_TYPE_CODE[self._doc_type])

    @property
    def environment(self):
        return self._environment

    @property
    def cert_manager(self):
        return self._cert

    @property
    def validate_schema(self):
        return self._validate

    @property
    def max_retries(self):
        return self._max_retries

    @property
    def cnpj(self):
        return self._cnpj

    def call(
        self, service: str, payload: dict[str, Any], include_header=False
    ) -> dict[str, Any]:
        """Execute a SEFAZ service call.

        Parameters
        ----------
        service:
            Service key (e.g. ``"NFeAutorizacao"``).
        payload:
            JSON-compatible dict to be converted to XML.
        include_header:
            Flag to include SOAP Header

        Returns
        -------
        dict
            Parsed XML response as a dict.
        """
        doc_type = self._config.doc_type.value
        xml_bytes = self._prepare_payload(doc_type, service, payload)
        logger.debug("raw xml: %s", xml_bytes.decode())
        url = get_endpoint(doc_type, self._uf, self._environment, service)
        builder = SOAPEnvelopeBuilder(
            doc_type,
            str(self._uf if UF_IBGE.get(self._uf) else self._original_uf),
            service,
        )
        soap_body = builder.build(xml_bytes, service in _GZIP_ENDPOINTS, include_header)
        logger.debug("soap xml: %s", soap_body.decode())
        headers = {"Content-Type": builder.content_type}
        raw_response = self._post_with_retry(url, soap_body, headers)
        logger.debug("received xml: %s", raw_response.decode())
        result = self._parse_response(raw_response)
        processed = build_processed_xml(
            doc_type, service, xml_bytes, extract_body(raw_response)
        )
        if processed is not None:
            result["@xml"] = processed
        return result

    def _prepare_payload(self, doc_type: str, service: str, payload: dict) -> bytes:
        """Validate → sign → return XML bytes."""
        xml_bytes = to_xml_bytes(payload)

        if self._config.requires_signature(service):
            xml_bytes = self._sign(xml_bytes, service)

        if self._validate and self._config.requires_validation(service):
            validate(xml_bytes, doc_type, service)

        return xml_bytes

    def _sign(self, xml_bytes: bytes, service: str) -> bytes:
        root = etree.fromstring(xml_bytes)
        xpath = self._config.get_sign_xpath(service)

        targets = root.findall(xpath) if xpath else []
        if not targets:
            targets = [root]

        for target in targets:
            ref_id = target.get("Id", "")
            parent = target.getparent()

            if parent is None:
                # target is the root itself; sign in place and return.
                signed = sign_xml(root, self._cert, ref_id)
                return etree.tostring(signed)

            signed_parent = sign_xml(parent, self._cert, ref_id)

            if parent is root:
                root = signed_parent
                continue

            grandparent = parent.getparent()
            idx = list(grandparent).index(parent)
            grandparent.remove(parent)
            grandparent.insert(idx, signed_parent)

        return etree.tostring(root)

    def _post_with_retry(self, url: str, body: bytes, headers: dict[str, str]) -> bytes:
        last_exc: Exception | None = None

        with self._cert.ssl_context() as ssl_ctx:
            transport = httpx.HTTPTransport(verify=ssl_ctx)
            with httpx.Client(transport=transport, timeout=self._timeout) as client:
                for attempt in range(self._max_retries + 1):
                    try:
                        response = client.post(url, content=body, headers=headers)

                        if (
                            response.status_code in _RETRYABLE_HTTP
                            and attempt < self._max_retries
                        ):
                            logger.warning(
                                "Retrying (attempt %d/%d) after HTTP %d from %s",
                                attempt + 1,
                                self._max_retries,
                                response.status_code,
                                url,
                            )
                            self._sleep(attempt)
                            continue

                        if response.status_code >= 400:
                            raise SOAPError(
                                f"HTTP {response.status_code} from {url}",
                                status_code=response.status_code,
                                body=response.text,
                            )

                        return response.content

                    except (httpx.TimeoutException, httpx.NetworkError) as exc:
                        last_exc = exc
                        if attempt < self._max_retries:
                            logger.warning(
                                "Retrying (attempt %d/%d) after %s: %s",
                                attempt + 1,
                                self._max_retries,
                                type(exc).__name__,
                                exc,
                            )
                            self._sleep(attempt)
                        continue

        raise RetryExhaustedError(
            f"All {self._max_retries} retries exhausted for {url}",
            last_error=last_exc,
        )

    @staticmethod
    def _sleep(attempt: int) -> None:
        time.sleep(SefazClient.BACKOFF_BASE * (2**attempt))

    @classmethod
    def _parse_response(cls, raw: bytes) -> dict[str, Any]:
        body_bytes = extract_body(raw)
        result = parse_xml_bytes(body_bytes)

        elems = list(result.values())
        inner = elems[0] if elems else result

        if isinstance(inner, dict):
            cstat = inner.get("cStat") or _nested_get(inner, "cStat")
            xmotivo = inner.get("xMotivo") or _nested_get(inner, "xMotivo")
            if cstat:
                logger.info("SEFAZ response cStat=%s xMotivo=%s", cstat, xmotivo)

        return result


def _nested_get(d: dict, key: str) -> str | None:
    """Recursively search for *key* in a nested dict."""
    for k, v in d.items():
        if k == key:
            return v
        if isinstance(v, dict):
            found = _nested_get(v, key)
            if found is not None:
                return found
    return None


def _ensure_list(d: dict, path: str):
    keys = path.split("/")
    target_key = keys.pop(-1)
    target = d
    while len(keys) > 0:
        target = target.get(keys.pop(0))
        if target is None:
            return d

    if (
        target is not None
        and target.get(target_key) is not None
        and isinstance(target.get(target_key), dict)
    ):
        target[target_key] = [target.get(target_key)]

    return d
