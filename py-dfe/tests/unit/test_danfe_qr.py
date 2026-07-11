import base64

import pytest

from py_dfe.danfe.qr import qr_data_uri
from py_dfe.exceptions import DFeError


def test_qr_data_uri_shape():
    uri = qr_data_uri("https://example.gov.br/nfce/qrcode?p=abc|2|1|1|HASH")
    assert uri.startswith("data:image/png;base64,")
    raw = base64.b64decode(uri.split(",", 1)[1])
    assert raw[:8] == b"\x89PNG\r\n\x1a\n"  # PNG magic


def test_qr_empty_raises():
    with pytest.raises(DFeError) as exc:
        qr_data_uri("")
    assert exc.value.status_code == 422
