"""Regression tests for MDF-e child-element ordering.

The Go api builds the MDFe payload as a JSON object whose keys arrive in
alphabetical order (Go marshals map keys sorted). py-dfe MUST reorder children
per XSD_ORDER before serializing, otherwise SEFAZ rejects the manifest.

SEFAZ's synchronous receiver (MDFeRecepcaoSinc) no longer accepts the <enviMDFe>
batch wrapper: the <MDFe> document is the root node sent in mdfeDadosMsg. The
fixture below mirrors that root.

These tests feed deliberately scrambled dicts through dict_to_xml and assert the
serialized element order matches the MDF-e 3.00 schema sequence — in particular
the rodoviário modal nodes added to XSD_ORDER (infModal/rodo/veicTracao/infANTT).
"""

from lxml import etree

from py_dfe.xmlops.builder import dict_to_xml

MDFE_NS = "http://www.portalfiscal.inf.br/mdfe"


def _local_children(el):
    return [etree.QName(c.tag).localname for c in el]


def _find(root, name):
    return root.find(f".//{{{MDFE_NS}}}{name}")


def _build_scrambled_mdfe():
    """MDFe root with children inserted in (wrong) alphabetical order, as Go emits."""
    # Keys here are intentionally NOT in schema order.
    infMDFe = {
        "@versao": "3.00",
        "@Id": "MDFe35260612345678000190580010000000011000000019",
        "tot": {"vCarga": "2000.00", "cUnid": "01", "qCarga": "101.0000", "qNFe": "2"},
        "prodPred": {"xProd": "MOTOR", "tpCarga": "05", "NCM": "85011019"},
        "infModal": {
            "@versao": "3.00",
            "rodo": {
                "veicTracao": {
                    "UF": "SP",
                    "tpCar": "00",
                    "tpRod": "01",
                    "tara": "10000",
                    "placa": "ABC1D23",
                    "condutor": [{"CPF": "11122233344", "xNome": "MOTORISTA"}],
                },
                "infANTT": {"RNTRC": "12345678"},
            },
        },
        "infDoc": {
            "infMunDescarga": [
                {
                    "infNFe": [{"chNFe": "1" * 44}, {"chNFe": "2" * 44}],
                    "xMunDescarga": "Rio de Janeiro",
                    "cMunDescarga": "3304557",
                }
            ]
        },
        "emit": {
            "enderEmit": {"UF": "SP", "xMun": "Sao Paulo", "cMun": "3550308", "xLgr": "Rua Teste"},
            "xNome": "CTECH",
            "CNPJ": "12345678000190",
        },
        "ide": {
            "UFFim": "RJ",
            "UFIni": "SP",
            "modal": "01",
            "mod": "58",
            "tpAmb": "2",
            "cUF": "35",
            "infMunCarrega": [{"xMunCarrega": "Sao Paulo", "cMunCarrega": "3550308"}],
        },
    }
    return {"MDFe": {"@xmlns": MDFE_NS, "infMDFe": infMDFe}}


def test_mdfe_is_root_no_envimdfe():
    root = dict_to_xml(_build_scrambled_mdfe())
    assert etree.QName(root.tag).localname == "MDFe"
    assert root.find(f".//{{{MDFE_NS}}}enviMDFe") is None


def test_infmdfe_children_reordered():
    root = dict_to_xml(_build_scrambled_mdfe())
    infMDFe = _find(root, "infMDFe")
    order = _local_children(infMDFe)
    # Schema sequence: ide, emit, infModal, infDoc, [seg], prodPred, tot, ...
    for earlier, later in [("ide", "emit"), ("emit", "infModal"), ("infModal", "infDoc"),
                           ("infDoc", "prodPred"), ("prodPred", "tot")]:
        assert order.index(earlier) < order.index(later), f"{earlier} must precede {later}: {order}"


def test_veictracao_children_reordered():
    root = dict_to_xml(_build_scrambled_mdfe())
    veic = _find(root, "veicTracao")
    order = _local_children(veic)
    # placa, tara before condutor; condutor before tpRod, tpCar, UF.
    assert order.index("placa") < order.index("condutor")
    assert order.index("tara") < order.index("condutor")
    assert order.index("condutor") < order.index("tpRod")
    assert order.index("tpRod") < order.index("UF")


def test_rodo_and_munDescarga_order():
    root = dict_to_xml(_build_scrambled_mdfe())
    rodo = _find(root, "rodo")
    assert _local_children(rodo).index("infANTT") < _local_children(rodo).index("veicTracao")

    mun = _find(root, "infMunDescarga")
    order = _local_children(mun)
    assert order.index("cMunDescarga") < order.index("xMunDescarga") < order.index("infNFe")
