from __future__ import annotations

import json
import logging
from typing import Any

from pydantic import ValidationError

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.danfe import RENDER_ONLY_SERVICES
from py_dfe.exceptions import (
    CERT_REQUIRED,
    UNEXPECTED_ERROR_CODE,
    VALIDATION_ERROR_CODE,
    DFeError,
    to_problem,
)
from py_dfe.logging_config import configure as _configure_logging
from py_dfe.models.request import LambdaRequest, LambdaResponse
from py_dfe.services import create_service

_configure_logging()
logger = logging.getLogger(__name__)


def handler(event: dict[str, Any], context: Any) -> dict[str, Any]:
    """AWS Lambda handler.

    Accepts either a raw dict or a dict with a nested ``"body"`` string
    (API Gateway proxy integration).
    """
    try:
        raw = _unwrap_event(event)
        req = LambdaRequest.model_validate(raw)
    except ValidationError as exc:
        return LambdaResponse(
            statusCode=422,
            body=to_problem(
                422, VALIDATION_ERROR_CODE, str(exc) or VALIDATION_ERROR_CODE
            ).model_dump_json(),
            headers=dict(),
        ).model_dump()

    logger.info(
        "Request: doc_type=%s service=%s uf=%s environment=%s cnpj=%s "
        "validate_schema=%s max_retries=%s cert_b64_len=%d body_keys=%s",
        req.doc_type,
        req.service,
        req.uf,
        req.environment,
        _mask_cnpj(req.cnpj),
        req.validate_schema,
        req.max_retries,
        len(req.certificate_b64) if req.certificate_b64 else 0,
        list(req.body.keys())
        if isinstance(req.body, dict)
        else type(req.body).__name__,
    )

    try:
        cert_manager = None
        if req.certificate_b64:
            cert_manager = CertificateManager(
                req.certificate_b64, req.certificate_password
            )
        elif req.service not in RENDER_ONLY_SERVICES:
            raise DFeError(
                400, CERT_REQUIRED,
                f"Service {req.service!r} requires a certificate",
            )
        service = create_service(
            doc_type=req.doc_type,
            cert_manager=cert_manager,
            uf=req.uf,
            environment=req.environment,
            validate_schema=req.validate_schema,
            max_retries=req.max_retries,
        )
        result = service.call(req.service, req.body)
        xml_present = "@xml" in result
        logger.info(
            "Success: doc_type=%s service=%s cnpj=%s processed_xml=%s response_keys=%s",
            req.doc_type,
            req.service,
            _mask_cnpj(req.cnpj),
            xml_present,
            list(result.keys()),
        )
        return LambdaResponse(
            statusCode=200, body=json.dumps(result), headers=dict()
        ).model_dump()

    except DFeError as exc:
        logger.error(
            "DFeError: doc_type=%s service=%s cnpj=%s error=%s",
            req.doc_type,
            req.service,
            _mask_cnpj(req.cnpj),
            exc,
            exc_info=True,
        )
        p = exc.to_problem()
        return LambdaResponse(
            statusCode=p.status, body=p.model_dump_json(), headers=dict()
        ).model_dump()

    except Exception as exc:
        logger.exception(
            "Unexpected error: doc_type=%s service=%s cnpj=%s error=%s",
            req.doc_type,
            req.service,
            _mask_cnpj(req.cnpj),
            exc,
        )
        return LambdaResponse(
            statusCode=500,
            body=to_problem(
                500, UNEXPECTED_ERROR_CODE, "An unexpected error occurred"
            ).model_dump_json(),
            headers=dict(),
        ).model_dump()


def _unwrap_event(event: dict[str, Any]) -> dict[str, Any]:
    """Support both direct invocation and API Gateway proxy integration."""
    if "body" in event and isinstance(event["body"], str):
        try:
            return json.loads(event["body"])
        except json.JSONDecodeError:
            pass
    return event


def _mask_cnpj(cnpj: str) -> str:
    return f"{cnpj[:2]}****{cnpj[-2:]}"
