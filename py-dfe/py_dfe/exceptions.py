"""Custom exceptions for py-dfe."""
from pydantic import BaseModel, Field, ConfigDict

CERTIFICATE_ERROR_CODE = 'certificate error'
ENDPOINT_NOT_FOUND_ERROR_CODE = 'endpoint not found'
SOAP_REQUEST_ERROR_CODE = 'soap request error'
XML_BUILD_ERROR = 'xml build error'
XML_SIGN_ERROR = 'xml sign error'
XML_VALIDATION_ERROR = 'xml validation error'
INVALID_SEFAZ_RESPONSE_ERROR = 'invalid sefaz response error'
RETRY_EXHAUSTED_ERROR = 'retry exhausted error'
VALIDATION_ERROR_CODE = 'validation error'
UNEXPECTED_ERROR_CODE = 'unexpected error'
CERT_REQUIRED = 'certificate required'


class Problem(BaseModel):
    model_config = ConfigDict(extra='allow')

    type: str = Field(default='about:blank', description='Problem type.')
    title: str = Field(description='Problem title.')
    detail: str = Field(description='Problem detail.')
    status: int = Field(description='Problem status')


def to_problem(status_code, error_code, error_description, **kwargs):
    return Problem.model_validate({
        'type': 'about:blank',
        'title': error_code,
        'detail': error_description,
        'status': status_code,
        **kwargs
    })


class DFeError(Exception):
    """Base exception for all py-dfe errors."""

    def __init__(self, status_code: int, error_code: str, error_description: str, **kwargs) -> None:
        super().__init__(error_description)
        self.status_code = status_code
        self.error_code = error_code
        self.error_description = error_description
        self.kwargs = kwargs

    def to_problem(self):
        return to_problem(self.status_code, self.error_code, self.error_description, **self.kwargs)


class CertificateError(DFeError):
    """Error loading or processing the digital certificate."""

    def __init__(self, message: str) -> None:
        super().__init__(400, CERTIFICATE_ERROR_CODE, message)


class EndpointNotFoundError(DFeError):
    """No endpoint URL found for the given parameters."""

    def __init__(self, message: str) -> None:
        super().__init__(400, ENDPOINT_NOT_FOUND_ERROR_CODE, message)


class SOAPError(DFeError):
    """Error building or parsing a SOAP message."""

    def __init__(self, message: str, status_code: int | None = None, body: str | None = None) -> None:
        super().__init__(status_code or 400, SOAP_REQUEST_ERROR_CODE, message, response_body=body)
        self.body = body


class XMLBuildError(DFeError):
    """Error converting JSON payload to XML."""

    def __init__(self, message: str) -> None:
        super().__init__(400, XML_BUILD_ERROR, message)


class XMLSignError(DFeError):
    """Error signing the XML document."""

    def __init__(self, message: str) -> None:
        super().__init__(400, XML_SIGN_ERROR, message)


class XMLValidationError(DFeError):
    """XSD schema validation failure."""

    def __init__(self, message: str, errors: list[str] | None = None) -> None:
        super().__init__(400, XML_VALIDATION_ERROR, message, errors=errors or [])
        self.errors = errors or []


class InvalidSefazResponseError(DFeError):
    def __init__(self, message: str):
        super().__init__(400, INVALID_SEFAZ_RESPONSE_ERROR, message)


class RetryExhaustedError(DFeError):
    """All retry attempts exhausted."""

    def __init__(self, message: str, last_error: Exception | None = None) -> None:
        super().__init__(400, RETRY_EXHAUSTED_ERROR, message)
        self.last_error = last_error
