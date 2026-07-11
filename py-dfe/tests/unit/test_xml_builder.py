"""Unit tests for JSON ↔ XML conversion."""

import pytest
from lxml import etree

from py_dfe.xmlops.builder import dict_to_xml, parse_xml_bytes, to_xml_bytes, xml_to_dict
from py_dfe.exceptions import XMLBuildError


class TestDictToXml:
    def test_simple_element(self):
        root = dict_to_xml({"root": {"child": "value"}})
        assert root.tag == "root"
        assert root.find("child").text == "value"

    def test_attributes_from_at_prefix(self):
        root = dict_to_xml({"elem": {"@versao": "4.00", "tag": "val"}})
        assert root.get("versao") == "4.00"

    def test_namespace_from_xmlns(self):
        root = dict_to_xml({"consStatServ": {
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "@versao": "4.00",
            "xServ": "STATUS",
        }})
        assert "portalfiscal" in root.tag
        assert root.get("versao") == "4.00"

    def test_text_content(self):
        root = dict_to_xml({"elem": "hello"})
        assert root.text == "hello"

    def test_list_creates_siblings(self):
        root = dict_to_xml({"root": {"item": ["a", "b", "c"]}})
        items = root.findall("item")
        assert len(items) == 3
        assert [i.text for i in items] == ["a", "b", "c"]

    def test_hash_text_key(self):
        root = dict_to_xml({"elem": {"@id": "1", "#text": "content"}})
        assert root.text == "content"
        assert root.get("id") == "1"

    def test_multiple_root_keys_raises(self):
        with pytest.raises(XMLBuildError):
            dict_to_xml({"a": 1, "b": 2})

    def test_nested_structure(self):
        data = {"nfe": {"infNFe": {"ide": {"nNF": "1"}}}}
        root = dict_to_xml(data)
        assert root.find("infNFe/ide/nNF").text == "1"

    def test_numeric_values_become_strings(self):
        root = dict_to_xml({"elem": {"num": 42}})
        assert root.find("num").text == "42"


class TestToXmlBytes:
    def test_returns_bytes(self):
        result = to_xml_bytes({"root": {"x": "1"}})
        assert isinstance(result, bytes)

    def test_valid_xml(self):
        result = to_xml_bytes({"root": {"x": "1"}})
        parsed = etree.fromstring(result)
        assert parsed.tag == "root"


class TestXmlToDict:
    def test_simple(self):
        xml = b"<root><a>1</a><b>2</b></root>"
        result = parse_xml_bytes(xml)
        assert result == {"root": {"a": "1", "b": "2"}}

    def test_attribute(self):
        xml = b'<root versao="4.00"><x>y</x></root>'
        result = parse_xml_bytes(xml)
        assert result["root"]["@versao"] == "4.00"

    def test_repeated_children(self):
        xml = b"<root><item>a</item><item>b</item></root>"
        result = parse_xml_bytes(xml)
        assert result["root"]["item"] == ["a", "b"]

    def test_text_only_element(self):
        xml = b"<root>hello</root>"
        result = parse_xml_bytes(xml)
        assert result == {"root": "hello"}

    def test_roundtrip(self):
        original = {"consStatServ": {
            "@versao": "4.00",
            "tpAmb": "2",
            "cUF": "35",
            "xServ": "STATUS",
        }}
        xml_bytes = to_xml_bytes(original)
        recovered = parse_xml_bytes(xml_bytes)

        assert recovered["consStatServ"]["tpAmb"] == "2"
        assert recovered["consStatServ"]["cUF"] == "35"
