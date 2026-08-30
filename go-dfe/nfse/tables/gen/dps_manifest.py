#!/usr/bin/env python3
"""Gera o manifesto canônico de caminhos emitíveis da DPS 1.01 a partir do XSD.

Uso (a partir da raiz do repositório):
    python3 go-dfe/nfse/tables/gen/dps_manifest.py

Entrada:  tmp/nfse/nfse-esquemas_xsd-*/Schemas/1.01/{DPS,tiposComplexos,tiposSimples}_v1.01.xsd
Saída:    go-dfe/nfse/nacional/testdata/dps_paths_v1.01.json (versionado)

O manifesto é a fonte do gate de cobertura em dps_coverage_test.go: o teste falha
com a lista exata de caminhos ausentes/inesperados. O script não roda em produção.
"""
import glob
import json
import os
import xml.etree.ElementTree as ET

XS = "{http://www.w3.org/2001/XMLSchema}"
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
OUT = os.path.join(ROOT, "go-dfe", "nfse", "nacional", "testdata", "dps_paths_v1.01.json")

# Conteúdo produzido pelo assinador XML-DSig, não pelo montador da DPS.
EXCLUDED_PREFIXES = ("DPS/Signature",)

# Profundidade máxima de expansão; a DPS não tem tipo recursivo, o limite só
# protege o gerador contra um XSD futuro que introduza um.
MAX_DEPTH = 40


def find_schema(name):
    matches = glob.glob(os.path.join(ROOT, "tmp", "nfse", "nfse-esquemas_xsd-*", "Schemas", "1.01", name))
    if not matches:
        raise SystemExit(f"XSD não encontrado: tmp/nfse/*/Schemas/1.01/{name}")
    return sorted(matches)[-1]


def load_types():
    """Indexa complexTypes e simpleTypes de todos os XSDs do namespace NFS-e."""
    complex_types, simple_types, elements = {}, {}, {}
    for name in ("DPS_v1.01.xsd", "tiposComplexos_v1.01.xsd", "tiposSimples_v1.01.xsd"):
        root = ET.parse(find_schema(name)).getroot()
        for node in root.findall(f"{XS}complexType"):
            complex_types[node.get("name")] = node
        for node in root.findall(f"{XS}simpleType"):
            simple_types[node.get("name")] = node
        for node in root.findall(f"{XS}element"):
            elements[node.get("name")] = node
    return complex_types, simple_types, elements


def strip_ns(qname):
    return qname.split(":")[-1] if qname else qname


class Walker:
    def __init__(self, complex_types, simple_types):
        self.complex_types = complex_types
        self.simple_types = simple_types
        self.paths = []
        self.seen = set()

    def emit(self, path, kind, occurrence, type_name, choice):
        if path in self.seen or path.startswith(EXCLUDED_PREFIXES):
            return
        self.seen.add(path)
        self.paths.append(
            {
                "path": path,
                "kind": kind,
                "occurrence": occurrence,
                "type": type_name,
                "choice": choice,
            }
        )

    def walk_type(self, type_name, path, depth):
        if depth > MAX_DEPTH:
            raise SystemExit(f"profundidade máxima excedida em {path}")
        node = self.complex_types.get(type_name)
        if node is None:
            return  # tipo simples: a folha já foi emitida pelo chamador
        for attr in node.findall(f"{XS}attribute"):
            name = attr.get("name")
            required = attr.get("use") == "required"
            self.emit(
                f"{path}/@{name}",
                "attribute",
                "1..1" if required else "0..1",
                strip_ns(attr.get("type")),
                "",
            )
        for particle in ("sequence", "choice", "all"):
            for group in node.findall(f"{XS}{particle}"):
                self.walk_particle(group, path, depth, choice="")

    def walk_particle(self, group, path, depth, choice):
        for child in group:
            tag = child.tag
            if tag == f"{XS}element":
                self.walk_element(child, path, depth, choice)
            elif tag == f"{XS}choice":
                # Cada choice recebe um rótulo estável pelo caminho + índice da
                # alternativa, para o teste provar que a união dos goldens cobre
                # todas as alternativas emitíveis.
                for index, option in enumerate(child):
                    label = f"{path}#choice{index}" if not choice else f"{choice}.{index}"
                    self.walk_particle_node(option, path, depth, label)
            elif tag in (f"{XS}sequence", f"{XS}all"):
                self.walk_particle(child, path, depth, choice)

    def walk_particle_node(self, node, path, depth, choice):
        if node.tag == f"{XS}element":
            self.walk_element(node, path, depth, choice)
        elif node.tag in (f"{XS}sequence", f"{XS}choice", f"{XS}all"):
            self.walk_particle(node, path, depth, choice)

    def walk_element(self, node, path, depth, choice):
        name = node.get("name") or strip_ns(node.get("ref"))
        min_occurs = node.get("minOccurs", "1")
        max_occurs = node.get("maxOccurs", "1")
        child_path = f"{path}/{name}"
        type_name = strip_ns(node.get("type"))
        inline = node.find(f"{XS}complexType")
        if inline is not None and type_name is None:
            type_name = f"{child_path}(inline)"
        self.emit(child_path, "element", f"{min_occurs}..{max_occurs}", type_name, choice)
        if child_path.startswith(EXCLUDED_PREFIXES):
            return
        if inline is not None:
            for attr in inline.findall(f"{XS}attribute"):
                self.emit(
                    f"{child_path}/@{attr.get('name')}",
                    "attribute",
                    "1..1" if attr.get("use") == "required" else "0..1",
                    strip_ns(attr.get("type")),
                    "",
                )
            for particle in ("sequence", "choice", "all"):
                for group in inline.findall(f"{XS}{particle}"):
                    self.walk_particle(group, child_path, depth + 1, choice="")
        elif type_name:
            self.walk_type(type_name, child_path, depth + 1)


def main():
    complex_types, simple_types, elements = load_types()
    root_element = elements["DPS"]
    walker = Walker(complex_types, simple_types)
    walker.emit("DPS", "element", "1..1", strip_ns(root_element.get("type")), "")
    walker.walk_type(strip_ns(root_element.get("type")), "DPS", 0)
    walker.paths.sort(key=lambda item: item["path"])
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w", encoding="utf-8") as handle:
        json.dump(
            {
                "schema": "DPS_v1.01.xsd",
                "root": "DPS",
                "excluded_prefixes": list(EXCLUDED_PREFIXES),
                "paths": walker.paths,
            },
            handle,
            ensure_ascii=False,
            indent=2,
        )
        handle.write("\n")
    print(f"{len(walker.paths)} caminhos -> {os.path.relpath(OUT, ROOT)}")


if __name__ == "__main__":
    main()
