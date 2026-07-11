"""Unit tests for CertificateManager."""

import base64
import ssl

import pytest
from cryptography.hazmat.primitives.serialization import pkcs12, Encoding

from py_dfe.certificate.manager import CertificateManager
from py_dfe.exceptions import CertificateError


class TestCertificateManager:
    def test_loads_pfx(self, cert_manager):
        assert cert_manager.cert_pem.startswith(b"-----BEGIN CERTIFICATE-----")

    def test_key_pem(self, cert_manager):
        assert b"PRIVATE KEY" in cert_manager.key_pem

    def test_ssl_context_is_ssl_context(self, cert_manager):
        with cert_manager.ssl_context() as ctx:
            assert isinstance(ctx, ssl.SSLContext)

    def test_ssl_context_cleans_temp_files(self, cert_manager, tmp_path, monkeypatch):
        """Temp PEM files must be removed after the context exits."""
        import tempfile
        created = []
        real_mkstemp = tempfile.mkstemp

        def spy_mkstemp(*args, **kwargs):
            fd, path = real_mkstemp(*args, **kwargs)
            created.append(path)
            return fd, path

        monkeypatch.setattr(tempfile, "mkstemp", spy_mkstemp)

        with cert_manager.ssl_context():
            pass

        from pathlib import Path
        for p in created:
            assert not Path(p).exists(), f"Temp file not cleaned: {p}"

    def test_invalid_pfx_raises(self):
        with pytest.raises(CertificateError):
            CertificateManager(pfx_b64=base64.b64encode(b"not-a-pfx").decode(), password="x")

    def test_wrong_password_raises(self, pfx_bytes):
        b64 = base64.b64encode(pfx_bytes).decode()
        with pytest.raises(CertificateError):
            CertificateManager(pfx_b64=b64, password="wrong-password")

    def test_certificate_property(self, cert_manager):
        from cryptography.x509 import Certificate
        assert isinstance(cert_manager.certificate, Certificate)
