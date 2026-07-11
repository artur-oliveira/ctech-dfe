"""Unit tests for py_dfe.xmlops.processor."""
import json
import textwrap

import pytest
from lxml import etree

from py_dfe.xmlops.processor import build_processed_xml, _all_by_local, _first_by_local

NFE_NS = "http://www.portalfiscal.inf.br/nfe"
CTE_NS = "http://www.portalfiscal.inf.br/cte"
MDF_NS = "http://www.portalfiscal.inf.br/mdfe"


def _xml(s: str) -> bytes:
    return textwrap.dedent(s).strip().encode()


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

class TestFindHelpers:
    def test_all_by_local_root_match(self):
        root = etree.fromstring(f'<NFe xmlns="{NFE_NS}"/>'.encode())
        assert len(_all_by_local(root, 'NFe')) == 1

    def test_all_by_local_nested(self):
        root = etree.fromstring(f'<enviNFe xmlns="{NFE_NS}"><NFe/></enviNFe>'.encode())
        assert len(_all_by_local(root, 'NFe')) == 1

    def test_all_by_local_multiple(self):
        root = etree.fromstring(
            f'<envEvento xmlns="{NFE_NS}"><evento/><evento/></envEvento>'.encode()
        )
        assert len(_all_by_local(root, 'evento')) == 2

    def test_first_by_local_none(self):
        root = etree.fromstring(f'<enviNFe xmlns="{NFE_NS}"/>'.encode())
        assert _first_by_local(root, 'protNFe') is None


# ---------------------------------------------------------------------------
# NF-e emission (nfeProc)
# ---------------------------------------------------------------------------

NFE_REQUEST = _xml(f"""
<enviNFe versao="4.00" xmlns="{NFE_NS}">
  <idLote>1</idLote>
  <indSinc>1</indSinc>
  <NFe>
    <infNFe Id="NFe35" versao="4.00"/>
  </NFe>
</enviNFe>
""")

NFE_RESPONSE = _xml(f"""
<nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4">
  <retEnviNFe versao="4.00" xmlns="{NFE_NS}">
    <cStat>104</cStat>
    <protNFe versao="4.00">
      <infProt>
        <cStat>100</cStat>
        <xMotivo>Autorizado</xMotivo>
        <nProt>135</nProt>
      </infProt>
    </protNFe>
  </retEnviNFe>
</nfeResultMsg>
""")


class TestNfeEmission:
    def test_returns_nfe_proc(self):
        xml = build_processed_xml('nfe', 'NFeAutorizacao', NFE_REQUEST, NFE_RESPONSE)
        assert xml is not None
        root = etree.fromstring(xml.encode())
        assert etree.QName(root.tag).localname == 'nfeProc'
        assert root.get('versao') == '4.00'

    def test_contains_nfe_and_prot(self):
        xml = build_processed_xml('nfe', 'NFeAutorizacao', NFE_REQUEST, NFE_RESPONSE)
        root = etree.fromstring(xml.encode())
        children = [etree.QName(c.tag).localname for c in root]
        assert 'NFe' in children
        assert 'protNFe' in children

    def test_nfce_same_structure(self):
        xml = build_processed_xml('nfce', 'NfceAutorizacao', NFE_REQUEST, NFE_RESPONSE)
        assert xml is not None
        root = etree.fromstring(xml.encode())
        assert etree.QName(root.tag).localname == 'nfeProc'

    def test_returns_none_without_protocol(self):
        response_no_prot = _xml(f'<nfeResultMsg><retEnviNFe xmlns="{NFE_NS}"><cStat>103</cStat></retEnviNFe></nfeResultMsg>')
        xml = build_processed_xml('nfe', 'NFeAutorizacao', NFE_REQUEST, response_no_prot)
        assert xml is None


# ---------------------------------------------------------------------------
# NF-e event (procEventoNFe)
# ---------------------------------------------------------------------------

NFE_EVENT_REQUEST = _xml(f"""
<envEvento versao="1.00" xmlns="{NFE_NS}">
  <idLote>1</idLote>
  <evento versao="1.00">
    <infEvento Id="ID110111" />
  </evento>
</envEvento>
""")

NFE_EVENT_RESPONSE = _xml(f"""
<nfeResultMsg xmlns="http://x">
  <nfeRecepcaoEventoNFResult xmlns="http://y">
    <retEnvEvento versao="1.00" xmlns="{NFE_NS}">
      <retEvento versao="1.00">
        <infEvento><cStat>135</cStat></infEvento>
      </retEvento>
    </retEnvEvento>
  </nfeRecepcaoEventoNFResult>
</nfeResultMsg>
""")


class TestNfeEvent:
    def test_returns_proc_evento_nfe(self):
        xml = build_processed_xml('nfe', 'RecepcaoEvento', NFE_EVENT_REQUEST, NFE_EVENT_RESPONSE)
        assert xml is not None
        root = etree.fromstring(xml.encode())
        assert etree.QName(root.tag).localname == 'procEventoNFe'
        assert root.get('versao') == '1.00'

    def test_contains_evento_and_ret_evento(self):
        xml = build_processed_xml('nfe', 'RecepcaoEvento', NFE_EVENT_REQUEST, NFE_EVENT_RESPONSE)
        root = etree.fromstring(xml.encode())
        children = [etree.QName(c.tag).localname for c in root]
        assert children == ['evento', 'retEvento']

    def test_multiple_events_returns_json_list(self):
        req = _xml(f"""
        <envEvento versao="1.00" xmlns="{NFE_NS}">
          <evento versao="1.00"><infEvento Id="ID1"/></evento>
          <evento versao="1.00"><infEvento Id="ID2"/></evento>
        </envEvento>
        """)
        resp = _xml(f"""
        <nfeResultMsg>
          <retEnvEvento xmlns="{NFE_NS}">
            <retEvento versao="1.00"><infEvento><cStat>135</cStat></infEvento></retEvento>
            <retEvento versao="1.00"><infEvento><cStat>135</cStat></infEvento></retEvento>
          </retEnvEvento>
        </nfeResultMsg>
        """)
        result = build_processed_xml('nfe', 'RecepcaoEvento', req, resp)
        assert result is not None
        parsed = json.loads(result)
        assert isinstance(parsed, list)
        assert len(parsed) == 2


# ---------------------------------------------------------------------------
# CT-e emission (cteProc)
# ---------------------------------------------------------------------------

CTE_REQUEST = _xml(f"""
<CTe xmlns="{CTE_NS}">
  <infCTe Id="CTe35" versao="4.00"/>
</CTe>
""")

CTE_RESPONSE = _xml(f"""
<cteRecepcaoSincResult xmlns="http://x">
  <retCTe versao="4.00" xmlns="{CTE_NS}">
    <cStat>100</cStat>
    <protCTe versao="4.00">
      <infProt><cStat>100</cStat><nProt>135</nProt></infProt>
    </protCTe>
  </retCTe>
</cteRecepcaoSincResult>
""")


class TestCteEmission:
    def test_returns_cte_proc(self):
        xml = build_processed_xml('cte', 'CTeRecepcaoSinc', CTE_REQUEST, CTE_RESPONSE)
        assert xml is not None
        root = etree.fromstring(xml.encode())
        assert etree.QName(root.tag).localname == 'cteProc'
        assert root.get('versao') == '4.00'

    def test_contains_cte_and_prot(self):
        xml = build_processed_xml('cte', 'CTeRecepcaoSinc', CTE_REQUEST, CTE_RESPONSE)
        root = etree.fromstring(xml.encode())
        children = [etree.QName(c.tag).localname for c in root]
        assert 'CTe' in children
        assert 'protCTe' in children


# ---------------------------------------------------------------------------
# MDF-e emission (mdfeProc)
# ---------------------------------------------------------------------------

MDF_REQUEST = _xml(f"""
<MDFe xmlns="{MDF_NS}">
  <infMDFe Id="MDFe35" versao="3.00"/>
</MDFe>
""")

MDF_RESPONSE = _xml(f"""
<retMDFe versao="3.00" xmlns="{MDF_NS}">
  <cStat>100</cStat>
  <protMDFe versao="3.00">
    <infProt><cStat>100</cStat><nProt>135</nProt></infProt>
  </protMDFe>
</retMDFe>
""")


class TestMdfeEmission:
    def test_returns_mdfe_proc(self):
        xml = build_processed_xml('mdfe', 'MDFeRecepcaoSinc', MDF_REQUEST, MDF_RESPONSE)
        assert xml is not None
        root = etree.fromstring(xml.encode())
        assert etree.QName(root.tag).localname == 'mdfeProc'
        assert root.get('versao') == '3.00'

    def test_contains_mdfe_and_prot(self):
        xml = build_processed_xml('mdfe', 'MDFeRecepcaoSinc', MDF_REQUEST, MDF_RESPONSE)
        root = etree.fromstring(xml.encode())
        children = [etree.QName(c.tag).localname for c in root]
        assert 'MDFe' in children
        assert 'protMDFe' in children


# ---------------------------------------------------------------------------
# Non-applicable services
# ---------------------------------------------------------------------------

class TestNonApplicableServices:
    def test_status_service_returns_none(self):
        assert build_processed_xml('nfe', 'NfeStatusServico', b'<a/>', b'<b/>') is None

    def test_query_returns_none(self):
        assert build_processed_xml('nfe', 'NfeConsultaProtocolo', b'<a/>', b'<b/>') is None

    def test_unknown_doc_type_returns_none(self):
        assert build_processed_xml('unknown', 'NFeAutorizacao', b'<a/>', b'<b/>') is None

    def test_bad_xml_returns_none(self):
        assert build_processed_xml('nfe', 'NFeAutorizacao', b'not xml', b'<b/>') is None
