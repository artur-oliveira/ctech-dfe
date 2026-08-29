"""Paridade entre as duas tabelas de ordem XSD.

`go-dfe/internal/xmlops/xsdorder/table.go` é um port 1:1 de
`py_dfe/xmlops/xsd_order.py`. Quando as duas divergem, o sintoma não é um erro:
o builder do py-dfe dá a **mesma** posição a todo filho que a tabela não conhece
(`rank.get(k, len(order))`), então a ordem passa a ser a de iteração do mapa
que a API montou — aleatória em Go. O XML sai inválido de forma intermitente.

Este teste lê o arquivo Go e compara chave por chave.
"""
from __future__ import annotations

import re
from pathlib import Path

import pytest

from py_dfe.xmlops.xsd_order import XSD_ORDER

_TABLE_GO = Path(__file__).resolve().parents[3] / "go-dfe" / "internal" / "xmlops" / "xsdorder" / "table.go"

# Comentário de linha inteira ou de fim de linha — nenhum valor da tabela
# contém "//", então basta cortar na primeira ocorrência.
_LINE_COMMENT = re.compile(r"//.*$", re.MULTILINE)
_ENTRY = re.compile(r'"((?:[^"\\]|\\.)*)"\s*:\s*\{([^{}]*)\}', re.DOTALL)
_ITEM = re.compile(r'"((?:[^"\\]|\\.)*)"')


def _parse_go_table(source: str) -> dict[str, list[str]]:
    """Extrai `"chave": {"a", "b"}` do arquivo Go, ignorando comentários."""
    clean = _LINE_COMMENT.sub("", source)
    return {
        key: _ITEM.findall(body)
        for key, body in _ENTRY.findall(clean)
    }


@pytest.fixture(scope="module")
def go_table() -> dict[str, list[str]]:
    if not _TABLE_GO.exists():
        pytest.skip(f"tabela Go não encontrada em {_TABLE_GO}")
    parsed = _parse_go_table(_TABLE_GO.read_text(encoding="utf-8"))
    assert len(parsed) > 100, f"parser extraiu só {len(parsed)} chaves — regex quebrou"
    return parsed


def test_toda_chave_do_go_existe_no_python(go_table):
    missing = sorted(set(go_table) - set(XSD_ORDER))
    assert not missing, f"chaves só na tabela Go: {missing}"


def test_toda_chave_do_python_existe_no_go(go_table):
    missing = sorted(set(XSD_ORDER) - set(go_table))
    assert not missing, f"chaves só na tabela Python: {missing}"


def test_ordem_identica_chave_por_chave(go_table):
    divergent = {
        key: {"go": go_table[key], "py": XSD_ORDER[key]}
        for key in sorted(set(go_table) & set(XSD_ORDER))
        if go_table[key] != XSD_ORDER[key]
    }
    assert not divergent, f"ordem divergente: {divergent}"
