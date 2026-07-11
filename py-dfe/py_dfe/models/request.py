"""Pydantic models for Lambda input/output."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field, field_validator


class LambdaRequest(BaseModel):
    """Input schema for the SEFAZ Lambda function.

    Fields
    ------
    cnpj:
        CNPJ of the issuing company (14 digits, no formatting).
    certificate_b64:
        Base-64 encoded PFX/PKCS12 certificate file.
    certificate_password:
        Password for the PFX certificate.
    uf:
        Brazilian state abbreviation (e.g. ``"SP"``).
    environment:
        ``"producao"`` or ``"homologacao"``.
    doc_type:
        ``"nfe"``, ``"nfce"``, ``"cte"``, or ``"mdfe"``.
    service:
        Service key (e.g. ``"NFeAutorizacao"``).
    body:
        JSON payload to be converted to XML and sent to SEFAZ.
    validate_schema:
        Whether to validate the XML against the XSD before sending.
    max_retries:
        Maximum number of retry attempts on transient errors.
    """

    cnpj: str = Field(..., pattern=r"^\d{14}$")
    certificate_b64: str | None = Field(default=None)
    certificate_password: str | None = Field(default=None)
    uf: str = Field(..., min_length=2, max_length=2)
    environment: str = Field(..., pattern=r"^(producao|homologacao|prod|hom)$")
    doc_type: str = Field(..., pattern=r"^(nfe|nfce|cte|mdfe)$")
    service: str = Field(..., min_length=3)
    body: dict[str, Any]
    validate_schema: bool = False
    max_retries: int = Field(default=3, ge=0, le=10)

    @field_validator("uf")
    @classmethod
    def uf_uppercase(cls, v: str) -> str:
        return v.upper()

    @field_validator("doc_type", mode="before")
    @classmethod
    def lowercase(cls, v: str) -> str:
        return v.lower() if isinstance(v, str) else v

    @field_validator("environment", mode="before")
    @classmethod
    def normalize_environment(cls, v: str) -> str:
        if isinstance(v, str):
            v = v.lower()
            if v == "producao":
                return "prod"
            elif v == "homologacao":
                return "hom"
        return v


class LambdaResponse(BaseModel):
    """Output schema returned by the Lambda function."""

    statusCode: int = Field(...)
    body: str = Field(...)
    headers: dict[str, Any] = Field(...)
