"""
Download all SEFAZ WSDL files in HOMOLOGAÇÃO and extract SOAPAction operation names.

Usage:
    TEST_CERT_PATH=/path/to/cert.pfx TEST_CERT_PASSWORD=senha python scripts/fetch_wsdls.py

Output:
    - wsdls/<authorizer>/<service>.xml   - raw WSDL file
    - wsdls/operations.json              - { "doc_type": { "service": "operationName" } }
    - wsdls/report.txt                   - human-readable summary
"""

from __future__ import annotations

import base64
import json
import os
import re
import sys
from pathlib import Path

import httpx
from lxml import etree

_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(_ROOT))

from py_dfe.certificate.manager import CertificateManager
from py_dfe.constants.endpoints import _NFE, _NFCE, _CTE, _MDFE  # noqa

_CERT_PATH = os.environ.get("TEST_CERT_PATH")
_CERT_PASS = os.environ.get("TEST_CERT_PASSWORD")
_OUT = Path(__file__).parent / "wsdls"
_ENV = "hom"
_TIMEOUT = httpx.Timeout(15.0, connect=5.0)

_HUB_SKIP = {
}


def _load_cert_manager() -> CertificateManager:
    if not _CERT_PATH or not _CERT_PASS:
        print("ERROR: set TEST_CERT_PATH and TEST_CERT_PASSWORD", file=sys.stderr)
        sys.exit(1)
    with open(_CERT_PATH, "rb") as f:
        pfx_b64 = base64.b64encode(f.read()).decode()
    return CertificateManager(pfx_b64=pfx_b64, password=_CERT_PASS)


def _fetch_wsdl(client: httpx.Client, url: str) -> bytes | None:
    wsdl_url = url.rstrip("/") + "?wsdl"
    try:
        resp = client.get(wsdl_url)
        if resp.status_code == 200:
            return resp.content
        print(f"  HTTP {resp.status_code} - {wsdl_url}")
        return None
    except Exception as exc:
        print(f"  ERROR {exc.__class__.__name__}: {exc} - {wsdl_url}")
        return None


def _extract_operation(wsdl_bytes: bytes) -> str | None:
    """Return the first wsdl:operation name found in the WSDL."""
    try:
        root = etree.fromstring(wsdl_bytes)
        wsdl_ns = "http://schemas.xmlsoap.org/wsdl/"
        soap_ns = "http://schemas.xmlsoap.org/wsdl/soap/"
        soap12_ns = "http://schemas.xmlsoap.org/wsdl/soap12/"

        for ns in (soap12_ns, soap_ns):
            for op in root.iter(f"{{{wsdl_ns}}}operation"):
                soap_op = op.find(f"{{{ns}}}operation")
                if soap_op is not None:
                    action = soap_op.get("soapAction", "")
                    if action:
                        return action.rstrip("/").split("/")[-1]

        for op in root.iter(f"{{{wsdl_ns}}}operation"):
            name = op.get("name")
            if name:
                return name
    except Exception as exc:
        print(f"    parse error: {exc}")
    return None


def _slug(url: str) -> str:
    """Turn a URL into a safe filename."""
    url = re.sub(r"^https?://", "", url)
    url = re.sub(r"[^a-zA-Z0-9_.-]", "_", url)
    return url[:120]


def _process_registry(
        client: httpx.Client,
        doc_type: str,
        registry: dict,
        operations: dict,
        seen_urls: set[str],
) -> None:
    doc_ops = operations.setdefault(doc_type, {})
    auth_entries = registry.items()

    for authorizer, envs in auth_entries:
        services = envs.get(_ENV, {})
        for service_key, url in services.items():
            if url in seen_urls:
                op = doc_ops.get(service_key)
                if op is None:
                    for _dt, _ops in operations.items():
                        if service_key in _ops:
                            doc_ops[service_key] = _ops[service_key]
                            break
                continue
            seen_urls.add(url)

            label = f"[{doc_type}/{authorizer}] {service_key}"
            print(f"  {label}")
            wsdl_bytes = _fetch_wsdl(client, url)
            if wsdl_bytes is None:
                continue

            out_dir = _OUT / doc_type / authorizer
            out_dir.mkdir(parents=True, exist_ok=True)
            fname = f"{service_key}.xml"
            (out_dir / fname).write_bytes(wsdl_bytes)

            op = _extract_operation(wsdl_bytes)
            if op:
                doc_ops[service_key] = op
                print(f"    → operation: {op}")
            else:
                print(f"    → operation: (not found)")


def main() -> None:
    cert_mgr = _load_cert_manager()
    _OUT.mkdir(exist_ok=True)

    operations: dict[str, dict[str, str]] = {}
    seen_urls: set[str] = set()

    registries = [
        ("nfe", _NFE),
        ("nfce", _NFCE),
        ("cte", _CTE),
        ("mdfe", _MDFE),
    ]

    with cert_mgr.ssl_context() as ssl_ctx:
        transport = httpx.HTTPTransport(verify=ssl_ctx)
        with httpx.Client(transport=transport, timeout=_TIMEOUT) as client:
            for doc_type, registry in registries:
                print(f"\n{'=' * 60}")
                print(f"  {doc_type.upper()}")
                print(f"{'=' * 60}")
                _process_registry(client, doc_type, registry, operations, seen_urls)

    ops_path = _OUT / "operations.json"
    ops_path.write_text(json.dumps(operations, indent=2, ensure_ascii=False))
    print(f"\n✓ operations.json → {ops_path}")

    report_lines = ["# SEFAZ WSDL Operations (HOMOLOGAÇÃO)\n"]
    for doc_type, svc_map in operations.items():
        report_lines.append(f"\n## {doc_type.upper()}")
        for svc, op in sorted(svc_map.items()):
            report_lines.append(f"  {svc:<30} → {op}")

    report_path = _OUT / "report.txt"
    report_path.write_text("\n".join(report_lines) + "\n")
    print(f"✓ report.txt      → {report_path}")

    print("\n" + "\n".join(report_lines))


if __name__ == "__main__":
    main()
