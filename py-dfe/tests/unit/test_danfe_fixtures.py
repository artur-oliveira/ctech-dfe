from lxml import etree

from tests.danfe_fixtures import sample_nfe_proc, sample_nfe55_proc


def test_sample_is_well_formed_and_has_key_nodes():
    xml = sample_nfe_proc()
    root = etree.fromstring(xml.encode("utf-8"))
    ns = {"n": "http://www.portalfiscal.inf.br/nfe"}
    assert root.find(".//n:infNFeSupl/n:qrCode", ns) is not None
    assert root.find(".//n:protNFe", ns) is not None
    assert len(root.findall(".//n:det", ns)) == 2


def test_contingency_toggle():
    xml = sample_nfe_proc(tp_emis="9")
    assert "<tpEmis>9</tpEmis>" in xml


def test_sample_nfe55_proc_is_model_55():
    xml = sample_nfe55_proc(n_items=3)
    assert "<mod>55</mod>" in xml
    assert xml.count("<det") == 3
    assert "<transp>" in xml
    assert "<dup>" in xml


def test_sample_nfe55_proc_contingency_flags():
    assert "<tpEmis>2</tpEmis>" in sample_nfe55_proc(tp_emis="2")
    assert "<tpAmb>2</tpAmb>" in sample_nfe55_proc(tp_amb="2")
