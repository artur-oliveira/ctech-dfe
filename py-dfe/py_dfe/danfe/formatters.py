"""Pure Brazilian-locale formatting helpers for DANFE rendering."""

from __future__ import annotations

import datetime
from decimal import Decimal, InvalidOperation


def money_br(value: str | float | int | None) -> str:
    """Format a monetary value as '1.234,56' (dot thousands, comma decimal)."""
    if value is None or value == "":
        value = 0
    try:
        dec = Decimal(str(value))
    except (InvalidOperation, ValueError):
        dec = Decimal(0)
    # Two decimals, US grouping, then swap separators.
    formatted = f"{dec:,.2f}"
    return formatted.replace(",", "_").replace(".", ",").replace("_", ".")


def dt_local(iso: str | None) -> str:
    """ISO-8601 (with offset) → 'dd/mm/yyyy HH:MM:SS', keeping the wall clock."""
    if not iso:
        return ""
    dt = datetime.datetime.fromisoformat(iso)
    return dt.strftime("%d/%m/%Y %H:%M:%S")


def date_br(iso: str | None) -> str:
    """ISO-8601 (with offset) → 'dd/mm/yyyy', keeping the wall clock."""
    if not iso:
        return ""
    dt = datetime.datetime.fromisoformat(iso)
    return dt.strftime("%d/%m/%Y")


def time_br(iso: str | None) -> str:
    """ISO-8601 (with offset) → 'HH:MM:SS', keeping the wall clock."""
    if not iso:
        return ""
    dt = datetime.datetime.fromisoformat(iso)
    return dt.strftime("%H:%M:%S")


def now_br() -> str:
    """Current local time as 'dd/mm/yyyy HH:MM:SS' (document generation stamp)."""
    return datetime.datetime.now().strftime("%d/%m/%Y %H:%M:%S")


def mask_cnpj(digits: str) -> str:
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) != 14:
        return digits or ""
    return f"{d[0:2]}.{d[2:5]}.{d[5:8]}/{d[8:12]}-{d[12:14]}"


def mask_cpf(digits: str) -> str:
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) != 11:
        return digits or ""
    return f"{d[0:3]}.{d[3:6]}.{d[6:9]}-{d[9:11]}"


def chave_blocks(key: str) -> str:
    d = "".join(filter(str.isdigit, key or ""))
    return " ".join(d[i:i + 4] for i in range(0, len(d), 4))


def mask_cpf_cnpj(digits: str) -> str:
    """Mask as CNPJ (14 digits) or CPF (11 digits); unchanged otherwise."""
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) == 14:
        return mask_cnpj(d)
    if len(d) == 11:
        return mask_cpf(d)
    return digits or ""


def num_nf(value: str) -> str:
    """NF-e number as '999.999.999' (9 digits, zero-padded)."""
    d = "".join(filter(str.isdigit, str(value or "")))
    if not d:
        return ""
    d = d.zfill(9)
    return f"{d[0:3]}.{d[3:6]}.{d[6:9]}"


def mask_cep(digits: str) -> str:
    d = "".join(filter(str.isdigit, digits or ""))
    if len(d) != 8:
        return digits or ""
    return f"{d[0:5]}-{d[5:8]}"


def pct(value: str | float | int | None) -> str:
    """Percentage/aliquota as '18,00' (reuses money_br grouping rules)."""
    return money_br(value)
