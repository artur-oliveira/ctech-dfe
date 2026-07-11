"""Shared pytest fixtures."""

from __future__ import annotations

import base64
import os

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives.serialization import pkcs12
from cryptography.x509.oid import NameOID

from py_dfe.certificate.manager import CertificateManager


@pytest.fixture(scope="session")
def test_private_key():
    return rsa.generate_private_key(public_exponent=65537, key_size=2048)


@pytest.fixture(scope="session")
def test_certificate(test_private_key):
    subject = issuer = x509.Name([
        x509.NameAttribute(NameOID.COMMON_NAME, "TEST SEFAZ"),
        x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Test Org"),
    ])
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(test_private_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(__import__("datetime").datetime.now(__import__("datetime").timezone.utc))
        .not_valid_after(
            __import__("datetime").datetime.now(__import__("datetime").timezone.utc)
            + __import__("datetime").timedelta(days=365)
        )
        .sign(test_private_key, hashes.SHA256())
    )
    return cert


@pytest.fixture(scope="session")
def pfx_bytes(test_private_key, test_certificate):
    return pkcs12.serialize_key_and_certificates(
        name=b"test",
        key=test_private_key,
        cert=test_certificate,
        cas=None,
        encryption_algorithm=serialization.BestAvailableEncryption(b"senha123"),
    )


@pytest.fixture(scope="session")
def pfx_b64(pfx_bytes):
    return base64.b64encode(pfx_bytes).decode()


@pytest.fixture(scope="session")
def cert_manager(pfx_b64):
    return CertificateManager(pfx_b64=pfx_b64, password="senha123")


@pytest.fixture(scope="session")
def real_cert_manager(cert_manager):
    if os.getenv('TEST_CERT_PATH') is not None and os.getenv('TEST_CERT_PASSWORD') is not None:
        with open(str(os.getenv('TEST_CERT_PATH')), 'rb') as f:
            binary_content = f.read()
            base64_utf8_str = base64.b64encode(binary_content).decode('utf-8')
            return CertificateManager(
                pfx_b64=base64_utf8_str,
                password=str(os.getenv("TEST_CERT_PASSWORD")),
            )
    return cert_manager


@pytest.fixture
def lambda_event(pfx_b64):
    return {
        "cnpj": "12345678000195",
        "certificate_b64": pfx_b64,
        "certificate_password": "senha123",
        "uf": "SP",
        "environment": "hom",
        "doc_type": "nfe",
        "service": "NfeStatusServico",
        "body": {
            "consStatServ": {
                "@versao": "4.00",
                "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                "tpAmb": "2",
                "cUF": "35",
                "xServ": "STATUS",
            }
        },
    }
