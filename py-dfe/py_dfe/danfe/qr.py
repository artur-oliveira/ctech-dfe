"""QR Code image generation for fiscal auxiliary documents."""

from __future__ import annotations

import base64
import io

import segno

from py_dfe.exceptions import DANFE_MISSING_QRCODE, DFeError

# Manual §4.5: error correction level M, UTF-8.
_QR_ERROR = "m"


def qr_data_uri(payload: str) -> str:
    """Render *payload* as a PNG QR Code embedded as a base64 data-URI."""
    if not payload:
        raise DFeError(422, DANFE_MISSING_QRCODE, "QR Code payload is empty")
    qr = segno.make(payload, error=_QR_ERROR, encoding="utf-8")
    buf = io.BytesIO()
    qr.save(buf, kind="png", scale=4, border=2)
    b64 = base64.b64encode(buf.getvalue()).decode("ascii")
    return f"data:image/png;base64,{b64}"
