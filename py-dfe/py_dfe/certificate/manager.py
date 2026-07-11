"""Digital certificate (PFX/PKCS12) management."""

from __future__ import annotations

import base64
import ssl
import tempfile
import warnings
from contextlib import contextmanager
from pathlib import Path
from typing import Generator

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.serialization import pkcs12
from cryptography.x509 import Certificate

from py_dfe.exceptions import CertificateError

# TODO: Fix warnings
warnings.filterwarnings("ignore", category=UserWarning, message=".*PKCS#12 bundle could not be parsed as DER.*")


class CertificateManager:
    """Loads a PFX certificate and exposes PEM-encoded cert/key.

    Example
    -------
    >>> import httpx
    >>> mgr = CertificateManager(pfx_b64="<cert_b64>", password="<cert_password>")
    >>> with mgr.ssl_context() as ctx:
    ...     client = httpx.Client(verify=ctx)
    """

    def __init__(self, pfx_b64: str, password: str) -> None:
        try:
            pfx_bytes = base64.b64decode(pfx_b64)
            self._private_key, self._certificate, self._chain = (
                pkcs12.load_key_and_certificates(pfx_bytes, password.encode())
            )
        except Exception as exc:
            raise CertificateError(f"Failed to load PFX certificate: {exc}") from exc

    @property
    def cert_pem(self) -> bytes:
        return self._certificate.public_bytes(serialization.Encoding.PEM)

    @property
    def key_pem(self) -> bytes:
        return self._private_key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.TraditionalOpenSSL,
            serialization.NoEncryption(),
        )

    @property
    def certificate(self) -> Certificate | None:
        return self._certificate

    @contextmanager
    def ssl_context(self) -> Generator[ssl.SSLContext, None, None]:
        """Context manager that yields an SSLContext with mTLS configured.

        Writes cert/key to temporary files inside /tmp (safe for Lambda).
        Files are deleted on exit.
        """
        cert_fd = key_fd = None
        cert_path = key_path = None
        try:

            cert_fd, cert_path = tempfile.mkstemp(suffix=".pem", dir="/tmp")
            key_fd, key_path = tempfile.mkstemp(suffix=".pem", dir="/tmp")

            Path(cert_path).write_bytes(self.cert_pem)
            Path(key_path).write_bytes(self.key_pem)

            ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
            ctx.check_hostname = False

            ctx.verify_mode = ssl.CERT_NONE
            ctx.load_cert_chain(certfile=cert_path, keyfile=key_path)

            yield ctx
        finally:
            for fd, path in [(cert_fd, cert_path), (key_fd, key_path)]:
                if fd is not None:
                    import os
                    try:
                        os.close(fd)
                    except OSError:
                        pass
                if path:
                    Path(path).unlink(missing_ok=True)

    def cert_paths(self) -> tuple[str, str]:
        """Write cert/key to /tmp and return (cert_path, key_path).

        Caller is responsible for deleting the files.
        """
        _, cert_path = tempfile.mkstemp(suffix=".pem", dir="/tmp")
        _, key_path = tempfile.mkstemp(suffix=".pem", dir="/tmp")
        Path(cert_path).write_bytes(self.cert_pem)
        Path(key_path).write_bytes(self.key_pem)
        return cert_path, key_path
