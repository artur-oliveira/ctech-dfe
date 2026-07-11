"""Unit tests for XSD element ordering.

Each test builds a dict with keys in WRONG insertion order and asserts
that the resulting XML has children in the correct XSD-defined sequence.
"""

import pytest
from lxml import etree

from py_dfe.xmlops.builder import to_xml_bytes


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _children(xml_bytes: bytes, path: str = None) -> list[str]:
    """Return the local-names of the direct children of the element at `path`."""
    root = etree.fromstring(xml_bytes)
    el = root if path is None else root.find(path)
    assert el is not None, f"Element not found: {path}"
    return [etree.QName(c.tag).localname for c in el]


def _order_is_correct(children: list[str], expected_order: list[str]) -> bool:
    """Return True if present elements appear in the expected relative order."""
    positions = {name: i for i, name in enumerate(children)}
    present = [name for name in expected_order if name in positions]
    return present == sorted(present, key=lambda n: positions[n])


# ---------------------------------------------------------------------------
# NF-e — infNFe
# ---------------------------------------------------------------------------

class TestInfNFe:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"infNFe": {
            "@versao": "4.00",
            **kwargs,
        }})

    def test_ide_before_emit(self):
        xml = self._build(emit={"CNPJ": "1"}, ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("ide") < kids.index("emit")

    def test_emit_before_dest(self):
        xml = self._build(dest={"CNPJ": "2"}, emit={"CNPJ": "1"}, ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("emit") < kids.index("dest")

    def test_dest_before_det(self):
        xml = self._build(det=[{"prod": {}}], dest={"CNPJ": "2"}, ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("dest") < kids.index("det")

    def test_det_before_total(self):
        xml = self._build(total={"ICMSTot": {}}, det=[{"prod": {}}], ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("det") < kids.index("total")

    def test_total_before_transp(self):
        xml = self._build(transp={"modFrete": "9"}, total={"ICMSTot": {}}, ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("total") < kids.index("transp")

    def test_transp_before_pag(self):
        xml = self._build(pag={"detPag": {}}, transp={"modFrete": "9"}, ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("transp") < kids.index("pag")

    def test_infAdic_after_pag(self):
        xml = self._build(infAdic={"infCpl": "obs"}, pag={"detPag": {}}, ide={"cUF": "35"})
        kids = _children(xml)
        assert kids.index("pag") < kids.index("infAdic")


# ---------------------------------------------------------------------------
# NF-e — ide
# ---------------------------------------------------------------------------

class TestIdeNFe:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"infNFe": {"ide": kwargs}})

    def test_cUF_first(self):
        # cUF inserted last — must still appear first
        xml = self._build(xServ="STATUS", nNF="1", mod="55", serie="1",
                          nNF2="1", cUF="35")
        kids = _children(xml, "ide")
        assert kids[0] == "cUF"

    def test_nNF_before_dhEmi(self):
        xml = self._build(dhEmi="2025-01-01T00:00:00-03:00", nNF="42", cUF="35")
        kids = _children(xml, "ide")
        assert kids.index("nNF") < kids.index("dhEmi")

    def test_tpAmb_before_finNFe(self):
        xml = self._build(finNFe="1", tpAmb="2", cUF="35")
        kids = _children(xml, "ide")
        assert kids.index("tpAmb") < kids.index("finNFe")

    def test_procEmi_before_verProc(self):
        xml = self._build(verProc="1.0", procEmi="0", cUF="35")
        kids = _children(xml, "ide")
        assert kids.index("procEmi") < kids.index("verProc")


# ---------------------------------------------------------------------------
# NF-e — emit (CNPJ/CPF must be first)
# ---------------------------------------------------------------------------

class TestEmitNFe:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"infNFe": {"emit": kwargs}})

    def test_cnpj_first_when_inserted_last(self):
        xml = self._build(xNome="X", xFant="Y",
                          enderEmit={"xLgr": "R"}, IE="123", CRT="1",
                          CNPJ="12345678000195")  # CNPJ last
        kids = _children(xml, "emit")
        assert kids[0] == "CNPJ"

    def test_cpf_first_when_inserted_last(self):
        xml = self._build(xNome="X", CRT="1",
                          CPF="12345678901")  # CPF last
        kids = _children(xml, "emit")
        assert kids[0] == "CPF"

    def test_xNome_before_xFant(self):
        xml = self._build(CNPJ="1", xFant="F", xNome="N")
        kids = _children(xml, "emit")
        assert kids.index("xNome") < kids.index("xFant")

    def test_enderEmit_before_IE(self):
        xml = self._build(CNPJ="1", IE="123", enderEmit={"xLgr": "R"})
        kids = _children(xml, "emit")
        assert kids.index("enderEmit") < kids.index("IE")

    def test_IE_before_CRT(self):
        xml = self._build(CNPJ="1", CRT="1", IE="123")
        kids = _children(xml, "emit")
        assert kids.index("IE") < kids.index("CRT")


# ---------------------------------------------------------------------------
# NF-e — dest
# ---------------------------------------------------------------------------

class TestDestNFe:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"infNFe": {"dest": kwargs}})

    def test_cnpj_before_xNome(self):
        xml = self._build(indIEDest="9", xNome="X", CNPJ="1")
        kids = _children(xml, "dest")
        assert kids.index("CNPJ") < kids.index("xNome")

    def test_xNome_before_indIEDest(self):
        xml = self._build(CNPJ="1", indIEDest="9", xNome="X")
        kids = _children(xml, "dest")
        assert kids.index("xNome") < kids.index("indIEDest")


# ---------------------------------------------------------------------------
# NF-e — prod
# ---------------------------------------------------------------------------

class TestProd:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"prod": kwargs})

    def test_cProd_first(self):
        xml = self._build(NCM="1", CFOP="5102", cProd="P001", xProd="X")
        kids = _children(xml)
        assert kids[0] == "cProd"

    def test_order_first_five(self):
        xml = self._build(CFOP="5102", NCM="12345678", cEAN="SEM GTIN",
                          xProd="Produto", cProd="P01")
        kids = _children(xml)
        assert _order_is_correct(kids, ["cProd", "cEAN", "xProd", "NCM", "CFOP"])

    def test_vProd_before_cEANTrib(self):
        xml = self._build(cProd="P", cEANTrib="SEM GTIN", vProd="10.00",
                          uCom="UN", qCom="1", vUnCom="10.00")
        kids = _children(xml)
        assert kids.index("vProd") < kids.index("cEANTrib")

    def test_indTot_after_vDesc(self):
        xml = self._build(cProd="P", indTot="1", vDesc="0.00", vProd="10.00")
        kids = _children(xml)
        assert kids.index("vDesc") < kids.index("indTot")


# ---------------------------------------------------------------------------
# NF-e — imposto
# ---------------------------------------------------------------------------

class TestImposto:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"imposto": kwargs})

    def test_ICMS_before_PIS(self):
        xml = self._build(PIS={"PISNT": {"CST": "07"}}, ICMS={"ICMS40": {}})
        kids = _children(xml)
        assert kids.index("ICMS") < kids.index("PIS")

    def test_PIS_before_COFINS(self):
        xml = self._build(COFINS={"COFINSNT": {}}, PIS={"PISNT": {}})
        kids = _children(xml)
        assert kids.index("PIS") < kids.index("COFINS")

    def test_IBSCBS_after_COFINS(self):
        xml = self._build(
            IBSCBS={"CST": "400", "cClassTrib": "400001"},
            COFINS={"COFINSNT": {"CST": "07"}},
            PIS={"PISNT": {"CST": "07"}},
            ICMS={"ICMS40": {"orig": "0", "CST": "40"}},
        )
        kids = _children(xml)
        assert kids.index("COFINS") < kids.index("IBSCBS")

    def test_IS_before_IBSCBS(self):
        xml = self._build(
            IBSCBS={"CST": "400", "cClassTrib": "400001"},
            IS={"CST": "01"},
        )
        kids = _children(xml)
        assert kids.index("IS") < kids.index("IBSCBS")


# ---------------------------------------------------------------------------
# IBSCBS (TTribNFe) — CST → cClassTrib → gIBSCBS
# ---------------------------------------------------------------------------

class TestIBSCBS:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"IBSCBS": kwargs})

    def test_CST_first(self):
        xml = self._build(gIBSCBS={"vBC": "100"}, cClassTrib="000001", CST="000")
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_cClassTrib_before_gIBSCBS(self):
        xml = self._build(gIBSCBS={"vBC": "100"}, cClassTrib="000001", CST="000")
        kids = _children(xml)
        assert kids.index("cClassTrib") < kids.index("gIBSCBS")

    def test_exempt_has_no_gIBSCBS(self):
        xml = self._build(CST="400", cClassTrib="400001")
        kids = _children(xml)
        assert "gIBSCBS" not in kids
        assert kids[0] == "CST"


# ---------------------------------------------------------------------------
# gIBSCBS inner (TCIBS) — vBC → gIBSUF → gIBSMun → vIBS → gCBS
# ---------------------------------------------------------------------------

class TestGIBSCBS:
    def _build(self, **kwargs) -> bytes:
        # parent_tag = IBSCBS triggers "IBSCBS:gIBSCBS" lookup
        return to_xml_bytes({"IBSCBS": {"CST": "000", "cClassTrib": "000001",
                                         "gIBSCBS": kwargs}})

    def _kids(self, **kwargs) -> list[str]:
        return _children(self._build(**kwargs), "gIBSCBS")

    def test_vBC_first(self):
        kids = self._kids(gCBS={"pCBS": "9"}, gIBSMun={"pIBSMun": "1"},
                          gIBSUF={"pIBSUF": "8"}, vIBS="9", vBC="100")
        assert kids[0] == "vBC"

    def test_gIBSUF_before_gIBSMun(self):
        kids = self._kids(vBC="100", gIBSMun={"pIBSMun": "1"},
                          gIBSUF={"pIBSUF": "8"}, vIBS="9", gCBS={"pCBS": "9"})
        assert kids.index("gIBSUF") < kids.index("gIBSMun")

    def test_vIBS_after_gIBSMun(self):
        kids = self._kids(vBC="100", vIBS="9", gIBSMun={"pIBSMun": "1"},
                          gIBSUF={"pIBSUF": "8"}, gCBS={"pCBS": "9"})
        assert kids.index("gIBSMun") < kids.index("vIBS")

    def test_gCBS_last(self):
        kids = self._kids(vBC="100", gIBSUF={"pIBSUF": "8"},
                          gIBSMun={"pIBSMun": "1"}, vIBS="9", gCBS={"pCBS": "9"})
        assert kids[-1] == "gCBS"

    def test_gIBSUF_children_order(self):
        xml = self._build(vBC="100", gIBSUF={"vIBSUF": "8", "pIBSUF": "8"},
                          gIBSMun={"pIBSMun": "1", "vIBSMun": "1"},
                          vIBS="9", gCBS={"pCBS": "9", "vCBS": "9"})
        kids = _children(xml, "gIBSCBS/gIBSUF")
        assert kids.index("pIBSUF") < kids.index("vIBSUF")

    def test_gIBSMun_children_order(self):
        xml = self._build(vBC="100", gIBSUF={"pIBSUF": "8", "vIBSUF": "8"},
                          gIBSMun={"vIBSMun": "1", "pIBSMun": "1"},
                          vIBS="9", gCBS={"pCBS": "9", "vCBS": "9"})
        kids = _children(xml, "gIBSCBS/gIBSMun")
        assert kids.index("pIBSMun") < kids.index("vIBSMun")

    def test_gCBS_children_order(self):
        xml = self._build(vBC="100", gIBSUF={"pIBSUF": "8", "vIBSUF": "8"},
                          gIBSMun={"pIBSMun": "1", "vIBSMun": "1"},
                          vIBS="9", gCBS={"vCBS": "9", "pCBS": "9"})
        kids = _children(xml, "gIBSCBS/gCBS")
        assert kids.index("pCBS") < kids.index("vCBS")


# ---------------------------------------------------------------------------
# ICMS groups
# ---------------------------------------------------------------------------

class TestICMSGroups:
    def test_ICMS40_orig_before_CST(self):
        xml = to_xml_bytes({"ICMS40": {"CST": "40", "orig": "0"}})
        kids = _children(xml)
        assert kids.index("orig") < kids.index("CST")

    def test_ICMSSN102_orig_before_CSOSN(self):
        xml = to_xml_bytes({"ICMSSN102": {"CSOSN": "102", "orig": "0"}})
        kids = _children(xml)
        assert kids.index("orig") < kids.index("CSOSN")

    def test_ICMSSN101_pCredSN_after_CSOSN(self):
        xml = to_xml_bytes({"ICMSSN101": {"pCredSN": "3.00", "CSOSN": "101",
                                           "orig": "0", "vCredICMSSN": "3.00"}})
        kids = _children(xml)
        assert kids.index("CSOSN") < kids.index("pCredSN")

    def test_ICMS00_modBC_after_CST(self):
        xml = to_xml_bytes({"ICMS00": {"vBC": "0", "CST": "00",
                                        "orig": "0", "pICMS": "0", "vICMS": "0",
                                        "modBC": "3"}})
        kids = _children(xml)
        assert kids.index("CST") < kids.index("modBC")

    def test_ICMSSN500_orig_before_CSOSN(self):
        xml = to_xml_bytes({"ICMSSN500": {"CSOSN": "500", "orig": "0"}})
        kids = _children(xml)
        assert kids.index("orig") < kids.index("CSOSN")


# ---------------------------------------------------------------------------
# PIS / COFINS
# ---------------------------------------------------------------------------

class TestPISCOFINS:
    def test_PISAliq_CST_first(self):
        xml = to_xml_bytes({"PISAliq": {"vPIS": "0", "pPIS": "0", "vBC": "0", "CST": "01"}})
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_PISNT_CST_first(self):
        xml = to_xml_bytes({"PISNT": {"CST": "07"}})
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_COFINSAliq_CST_first(self):
        xml = to_xml_bytes({"COFINSAliq": {"vCOFINS": "0", "pCOFINS": "0",
                                            "vBC": "0", "CST": "01"}})
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_PISOutr_CST_before_vBC(self):
        xml = to_xml_bytes({"PISOutr": {"vPIS": "0", "pPIS": "0",
                                         "vBC": "100", "CST": "99"}})
        kids = _children(xml)
        assert kids.index("CST") < kids.index("vBC")


# ---------------------------------------------------------------------------
# ICMSTot
# ---------------------------------------------------------------------------

class TestICMSTot:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"ICMSTot": kwargs})

    def test_vBC_first(self):
        xml = self._build(vNF="100", vProd="100", vBC="0")
        kids = _children(xml)
        assert kids[0] == "vBC"

    def test_vProd_before_vFrete(self):
        xml = self._build(vBC="0", vFrete="0", vProd="100")
        kids = _children(xml)
        assert kids.index("vProd") < kids.index("vFrete")

    def test_vNF_before_vTotTrib(self):
        xml = self._build(vBC="0", vTotTrib="0", vNF="100", vProd="100")
        kids = _children(xml)
        assert kids.index("vNF") < kids.index("vTotTrib")

    def test_vDesc_before_vII(self):
        xml = self._build(vBC="0", vII="0", vDesc="0", vProd="100")
        kids = _children(xml)
        assert kids.index("vDesc") < kids.index("vII")


# ---------------------------------------------------------------------------
# total (NF-e)
# ---------------------------------------------------------------------------

class TestTotal:
    def test_ICMSTot_first(self):
        xml = to_xml_bytes({"total": {
            "IBSCBSTot": {"vBCIBSCBS": "0"},
            "ICMSTot": {"vBC": "0"},
        }})
        kids = _children(xml)
        assert kids[0] == "ICMSTot"

    def test_ISTot_before_IBSCBSTot(self):
        xml = to_xml_bytes({"total": {
            "IBSCBSTot": {"vBCIBSCBS": "0"},
            "ISTot": {"vIS": "0"},
            "ICMSTot": {"vBC": "0"},
        }})
        kids = _children(xml)
        assert kids.index("ISTot") < kids.index("IBSCBSTot")

    def test_IBSCBSTot_after_ICMSTot(self):
        xml = to_xml_bytes({"total": {
            "IBSCBSTot": {"vBCIBSCBS": "0"},
            "ICMSTot": {"vBC": "0"},
        }})
        kids = _children(xml)
        assert kids.index("ICMSTot") < kids.index("IBSCBSTot")


# ---------------------------------------------------------------------------
# transp / vol
# ---------------------------------------------------------------------------

class TestTransp:
    def test_modFrete_first(self):
        xml = to_xml_bytes({"transp": {"vol": {"qVol": "1"}, "modFrete": "9"}})
        kids = _children(xml)
        assert kids[0] == "modFrete"

    def test_vol_after_modFrete(self):
        xml = to_xml_bytes({"transp": {"vol": {"pesoL": "1"}, "modFrete": "9"}})
        kids = _children(xml)
        assert kids.index("modFrete") < kids.index("vol")


# ---------------------------------------------------------------------------
# pag / detPag
# ---------------------------------------------------------------------------

class TestPag:
    def test_detPag_before_vTroco(self):
        xml = to_xml_bytes({"pag": {"vTroco": "0", "detPag": {"tPag": "01", "vPag": "10"}}})
        kids = _children(xml)
        assert kids.index("detPag") < kids.index("vTroco")

    def test_detPag_tPag_before_vPag(self):
        xml = to_xml_bytes({"detPag": {"vPag": "10", "tPag": "01"}})
        kids = _children(xml)
        assert kids.index("tPag") < kids.index("vPag")


# ---------------------------------------------------------------------------
# Service request elements
# ---------------------------------------------------------------------------

class TestServiceElements:
    def test_consStatServ_order(self):
        xml = to_xml_bytes({"consStatServ": {"xServ": "STATUS", "cUF": "35", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids == ["tpAmb", "cUF", "xServ"]

    def test_consSitNFe_order(self):
        xml = to_xml_bytes({"consSitNFe": {"chNFe": "111", "xServ": "CONSULTAR", "tpAmb": "2"}})
        kids = _children(xml)
        assert _order_is_correct(kids, ["tpAmb", "xServ", "chNFe"])

    def test_inutNFe_infInut_tpAmb_first(self):
        xml = to_xml_bytes({"inutNFe": {"infInut": {
            "xJust": "X", "nNFFin": "1", "nNFIni": "1",
            "serie": "1", "mod": "55", "CNPJ": "1",
            "ano": "25", "cUF": "35", "xServ": "INUTILIZAR", "tpAmb": "2",
        }}})
        kids = _children(xml, "infInut")
        assert kids[0] == "tpAmb"

    def test_inutNFe_infInut_xJust_last(self):
        xml = to_xml_bytes({"inutNFe": {"infInut": {
            "xJust": "justificativa", "nNFFin": "1", "nNFIni": "1",
            "serie": "1", "mod": "55", "CNPJ": "1",
            "ano": "25", "cUF": "35", "xServ": "INUTILIZAR", "tpAmb": "2",
        }}})
        kids = _children(xml, "infInut")
        assert kids[-1] == "xJust"

    def test_distDFeInt_tpAmb_first(self):
        xml = to_xml_bytes({"distDFeInt": {
            "distNSU": {"ultNSU": "000"},
            "CNPJ": "1", "cUFAutor": "35", "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids[0] == "tpAmb"

    def test_distDFeInt_CNPJ_before_distNSU(self):
        xml = to_xml_bytes({"distDFeInt": {
            "distNSU": {"ultNSU": "000"},
            "cUFAutor": "35", "tpAmb": "2", "CNPJ": "1",
        }})
        kids = _children(xml)
        assert kids.index("CNPJ") < kids.index("distNSU")


# ---------------------------------------------------------------------------
# Events (NF-e)
# ---------------------------------------------------------------------------

class TestEventosNFe:
    def test_envEvento_idLote_before_evento(self):
        xml = to_xml_bytes({"envEvento": {
            "evento": {"infEvento": {}}, "idLote": "1",
        }})
        kids = _children(xml)
        assert kids.index("idLote") < kids.index("evento")

    def test_infEvento_cOrgao_first(self):
        xml = to_xml_bytes({"evento": {"infEvento": {
            "detEvento": {"descEvento": "X"},
            "verEvento": "1.00", "nSeqEvento": "1",
            "tpEvento": "110111", "dhEvento": "T",
            "chNFe": "1", "CNPJ": "1", "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids[0] == "cOrgao"

    def test_infEvento_chNFe_before_dhEvento(self):
        xml = to_xml_bytes({"evento": {"infEvento": {
            "dhEvento": "T", "chNFe": "1",
            "tpEvento": "110111", "nSeqEvento": "1",
            "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids.index("chNFe") < kids.index("dhEvento")

    def test_infEvento_detEvento_last(self):
        xml = to_xml_bytes({"evento": {"infEvento": {
            "detEvento": {"descEvento": "Cancelamento"},
            "verEvento": "1.00", "nSeqEvento": "1",
            "tpEvento": "110111", "dhEvento": "T",
            "chNFe": "1", "CNPJ": "1", "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids[-1] == "detEvento"

    def test_detEvento_descEvento_first(self):
        xml = to_xml_bytes({"detEvento": {
            "xJust": "Erro", "nProt": "135", "descEvento": "Cancelamento",
        }})
        kids = _children(xml)
        assert kids[0] == "descEvento"

    def test_detEvento_nProt_before_xJust(self):
        xml = to_xml_bytes({"detEvento": {
            "xJust": "Erro de emissão", "descEvento": "Cancelamento", "nProt": "135",
        }})
        kids = _children(xml)
        assert kids.index("nProt") < kids.index("xJust")


# ---------------------------------------------------------------------------
# CT-e — emit disambiguation (IE before xNome, unlike NF-e)
# ---------------------------------------------------------------------------

class TestEmitCTe:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"infCte": {"emit": kwargs}})

    def test_cnpj_first(self):
        xml = self._build(CRT="3", xNome="X", IE="1", CNPJ="1")
        kids = _children(xml, "emit")
        assert kids[0] == "CNPJ"

    def test_IE_before_xNome(self):
        # CT-e specific: IE before xNome (opposite of NF-e)
        xml = self._build(CNPJ="1", xNome="X", CRT="3", IE="1")
        kids = _children(xml, "emit")
        assert kids.index("IE") < kids.index("xNome")

    def test_xNome_before_enderEmit(self):
        xml = self._build(CNPJ="1", enderEmit={"xLgr": "R"}, xNome="X", IE="1")
        kids = _children(xml, "emit")
        assert kids.index("xNome") < kids.index("enderEmit")


# ---------------------------------------------------------------------------
# CT-e — ide disambiguation
# ---------------------------------------------------------------------------

class TestIdeCTe:
    def test_cUF_first(self):
        xml = to_xml_bytes({"infCte": {"ide": {
            "mod": "57", "cUF": "35", "CFOP": "5352",
        }}})
        kids = _children(xml, "ide")
        assert kids[0] == "cUF"

    def test_CFOP_before_mod(self):
        xml = to_xml_bytes({"infCte": {"ide": {
            "mod": "57", "cUF": "35", "CFOP": "5352",
        }}})
        kids = _children(xml, "ide")
        assert kids.index("CFOP") < kids.index("mod")


# ---------------------------------------------------------------------------
# MDF-e — emit and ide disambiguation
# ---------------------------------------------------------------------------

class TestEmitMDFe:
    def test_cnpj_first(self):
        xml = to_xml_bytes({"infMDFe": {"emit": {
            "xNome": "X", "IE": "1", "CNPJ": "1",
        }}})
        kids = _children(xml, "emit")
        assert kids[0] == "CNPJ"

    def test_IE_before_xNome(self):
        xml = to_xml_bytes({"infMDFe": {"emit": {
            "CNPJ": "1", "xNome": "X", "IE": "1",
        }}})
        kids = _children(xml, "emit")
        assert kids.index("IE") < kids.index("xNome")


class TestIdeMDFe:
    def test_cUF_first(self):
        xml = to_xml_bytes({"infMDFe": {"ide": {
            "mod": "58", "serie": "1", "cUF": "35",
        }}})
        kids = _children(xml, "ide")
        assert kids[0] == "cUF"

    def test_mod_after_tpAmb(self):
        xml = to_xml_bytes({"infMDFe": {"ide": {
            "mod": "58", "cUF": "35", "tpAmb": "2",
        }}})
        kids = _children(xml, "ide")
        assert kids.index("tpAmb") < kids.index("mod")


# ---------------------------------------------------------------------------
# ConsStatServ CT-e / MDF-e
# ---------------------------------------------------------------------------

class TestConsStatServ:
    def test_cte_status_order(self):
        xml = to_xml_bytes({"consStatServCTe": {
            "xServ": "STATUS", "cUF": "35", "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "cUF", "xServ"]

    def test_mdfe_status_order(self):
        xml = to_xml_bytes({"consStatServMDFe": {
            "xServ": "STATUS", "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "xServ"]


# ---------------------------------------------------------------------------
# enderEmit / enderDest
# ---------------------------------------------------------------------------

class TestEndereco:
    def test_xLgr_first(self):
        xml = to_xml_bytes({"enderEmit": {
            "UF": "SP", "xMun": "São Paulo", "cMun": "1",
            "xBairro": "B", "nro": "1", "xLgr": "Rua X",
        }})
        kids = _children(xml)
        assert kids[0] == "xLgr"

    def test_nro_before_xBairro(self):
        xml = to_xml_bytes({"enderEmit": {
            "xBairro": "B", "xLgr": "R", "nro": "1",
        }})
        kids = _children(xml)
        assert kids.index("nro") < kids.index("xBairro")

    def test_UF_before_CEP(self):
        xml = to_xml_bytes({"enderDest": {
            "CEP": "01310100", "xMun": "SP", "cMun": "1",
            "xBairro": "B", "nro": "1", "xLgr": "R", "UF": "SP",
        }})
        kids = _children(xml)
        assert kids.index("UF") < kids.index("CEP")


# ---------------------------------------------------------------------------
# envEvento CT-e / MDF-e
# ---------------------------------------------------------------------------

class TestEventoCTe:
    def test_infEvento_cOrgao_first(self):
        xml = to_xml_bytes({"eventoCTe": {"infEvento": {
            "detEvento": {}, "nSeqEvento": "1",
            "tpEvento": "110111", "dhEvento": "T",
            "chCTe": "1", "CNPJ": "1", "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids[0] == "cOrgao"

    def test_chCTe_before_dhEvento(self):
        xml = to_xml_bytes({"eventoCTe": {"infEvento": {
            "dhEvento": "T", "chCTe": "1",
            "tpEvento": "110111", "nSeqEvento": "1",
            "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids.index("chCTe") < kids.index("dhEvento")


# ---------------------------------------------------------------------------
# enviNFe / NFe
# ---------------------------------------------------------------------------

class TestEnviNFe:
    def test_idLote_before_NFe(self):
        xml = to_xml_bytes({"enviNFe": {"NFe": {}, "indSinc": "0", "idLote": "1"}})
        kids = _children(xml)
        assert kids.index("idLote") < kids.index("NFe")

    def test_indSinc_before_NFe(self):
        xml = to_xml_bytes({"enviNFe": {"NFe": {}, "idLote": "1", "indSinc": "0"}})
        kids = _children(xml)
        assert kids.index("indSinc") < kids.index("NFe")


class TestNFe:
    def test_infNFe_before_infNFeSupl(self):
        xml = to_xml_bytes({"NFe": {"infNFeSupl": {"qrCode": "X"}, "infNFe": {}}})
        kids = _children(xml)
        assert kids.index("infNFe") < kids.index("infNFeSupl")


# ---------------------------------------------------------------------------
# retirada / entrega
# ---------------------------------------------------------------------------

class TestRetiradaEntrega:
    def test_retirada_CNPJ_first(self):
        xml = to_xml_bytes({"retirada": {"xMun": "SP", "xLgr": "R", "CNPJ": "1"}})
        kids = _children(xml)
        assert kids[0] == "CNPJ"

    def test_retirada_xNome_before_xLgr(self):
        xml = to_xml_bytes({"retirada": {"xLgr": "R", "xNome": "X", "CNPJ": "1"}})
        kids = _children(xml)
        assert kids.index("xNome") < kids.index("xLgr")

    def test_retirada_cMun_before_xMun(self):
        xml = to_xml_bytes({"retirada": {"CNPJ": "1", "xMun": "SP", "cMun": "3550308"}})
        kids = _children(xml)
        assert kids.index("cMun") < kids.index("xMun")

    def test_entrega_CPF_first(self):
        xml = to_xml_bytes({"entrega": {"xMun": "SP", "xLgr": "R", "CPF": "1"}})
        kids = _children(xml)
        assert kids[0] == "CPF"

    def test_entrega_UF_after_xMun(self):
        xml = to_xml_bytes({"entrega": {"CPF": "1", "UF": "SP", "xMun": "SP", "cMun": "1"}})
        kids = _children(xml)
        assert kids.index("xMun") < kids.index("UF")


# ---------------------------------------------------------------------------
# autXML / det
# ---------------------------------------------------------------------------

class TestAutXML:
    def test_CNPJ_before_CPF(self):
        xml = to_xml_bytes({"autXML": {"CPF": "1", "CNPJ": "1"}})
        kids = _children(xml)
        assert kids.index("CNPJ") < kids.index("CPF")


class TestDet:
    def test_prod_before_imposto(self):
        xml = to_xml_bytes({"det": {"imposto": {}, "prod": {"cProd": "1"}}})
        kids = _children(xml)
        assert kids.index("prod") < kids.index("imposto")

    def test_imposto_before_obsItem(self):
        xml = to_xml_bytes({"det": {"obsItem": {}, "imposto": {}, "prod": {}}})
        kids = _children(xml)
        assert kids.index("imposto") < kids.index("obsItem")


# ---------------------------------------------------------------------------
# IS — Imposto Seletivo
# ---------------------------------------------------------------------------

class TestIS:
    def _build(self, **kwargs) -> bytes:
        return to_xml_bytes({"IS": kwargs})

    def test_CST_first(self):
        xml = self._build(vIS="0", pAliq="0", vBC="100", CST="01")
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_vBC_before_pAliq(self):
        xml = self._build(CST="01", pAliq="0", vBC="100")
        kids = _children(xml)
        assert kids.index("vBC") < kids.index("pAliq")

    def test_vIS_last_of_present(self):
        xml = self._build(CST="01", vIS="0", pAliq="0", vBC="100")
        kids = _children(xml)
        assert _order_is_correct(kids, ["CST", "vBC", "pAliq", "vIS"])


# ---------------------------------------------------------------------------
# ICMS groups (remaining)
# ---------------------------------------------------------------------------

class TestICMS10:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS10": {"CST": "10", "vBC": "0", "orig": "0",
                                        "pICMS": "0", "vICMS": "0", "modBC": "3",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_CST_after_orig(self):
        xml = to_xml_bytes({"ICMS10": {"vBC": "0", "CST": "10", "orig": "0",
                                        "pICMS": "0", "vICMS": "0", "modBC": "3",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("orig") < kids.index("CST")

    def test_vICMS_before_modBCST(self):
        xml = to_xml_bytes({"ICMS10": {"orig": "0", "CST": "10", "modBC": "3", "vBC": "0",
                                        "pICMS": "0", "vICMS": "0",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("vICMS") < kids.index("modBCST")


class TestICMS20:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS20": {"CST": "20", "orig": "0", "modBC": "3",
                                        "pRedBC": "0", "vBC": "0", "pICMS": "12", "vICMS": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_pRedBC_before_vBC(self):
        xml = to_xml_bytes({"ICMS20": {"orig": "0", "CST": "20", "modBC": "3",
                                        "vBC": "0", "pRedBC": "0", "pICMS": "12", "vICMS": "0"}})
        kids = _children(xml)
        assert kids.index("pRedBC") < kids.index("vBC")


class TestICMS30:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS30": {"CSOSN": "500", "CST": "30", "orig": "0",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_CST_before_modBCST(self):
        xml = to_xml_bytes({"ICMS30": {"orig": "0", "modBCST": "4", "CST": "30",
                                        "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("CST") < kids.index("modBCST")


class TestICMS51:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS51": {"CST": "51", "orig": "0", "modBC": "3",
                                        "vBC": "0", "pICMS": "12", "vICMS": "0",
                                        "pDif": "0", "vICMSDif": "0", "vICMSOp": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_pRedBC_before_vBC(self):
        xml = to_xml_bytes({"ICMS51": {"orig": "0", "CST": "51", "modBC": "3",
                                        "vBC": "0", "pRedBC": "0", "pICMS": "12",
                                        "vICMSOp": "0", "pDif": "0", "vICMSDif": "0", "vICMS": "0"}})
        kids = _children(xml)
        assert kids.index("pRedBC") < kids.index("vBC")


class TestICMS60:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS60": {"CST": "60", "orig": "0",
                                        "vBCSTRet": "0", "vICMSSTRet": "0", "pST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_CST_before_vBCSTRet(self):
        xml = to_xml_bytes({"ICMS60": {"orig": "0", "vBCSTRet": "0",
                                        "CST": "60", "vICMSSTRet": "0", "pST": "0"}})
        kids = _children(xml)
        assert kids.index("CST") < kids.index("vBCSTRet")


class TestICMS70:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS70": {"CST": "70", "orig": "0", "modBC": "3",
                                        "vBC": "0", "pICMS": "12", "vICMS": "0",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_pRedBC_before_vBC(self):
        xml = to_xml_bytes({"ICMS70": {"orig": "0", "CST": "70", "modBC": "3",
                                        "vBC": "0", "pRedBC": "0", "pICMS": "12", "vICMS": "0",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("pRedBC") < kids.index("vBC")


class TestICMS90:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMS90": {"CST": "90", "orig": "0", "modBC": "3",
                                        "vBC": "0", "pICMS": "12", "vICMS": "0",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_vBC_before_pRedBC(self):
        xml = to_xml_bytes({"ICMS90": {"orig": "0", "CST": "90", "modBC": "3",
                                        "pRedBC": "0", "vBC": "0", "pICMS": "12", "vICMS": "0",
                                        "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("vBC") < kids.index("pRedBC")


class TestICMSPart:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMSPart": {"CST": "10", "orig": "0", "modBC": "3",
                                          "vBC": "0", "pICMS": "0", "vICMS": "0",
                                          "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0",
                                          "pBCOp": "0", "UFST": "SP"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_pBCOp_before_UFST(self):
        xml = to_xml_bytes({"ICMSPart": {"orig": "0", "CST": "10", "modBC": "3",
                                          "vBC": "0", "pRedBC": "0", "pICMS": "0", "vICMS": "0",
                                          "modBCST": "4", "vBCST": "0", "pICMSST": "0", "vICMSST": "0",
                                          "UFST": "SP", "pBCOp": "0"}})
        kids = _children(xml)
        assert kids.index("pBCOp") < kids.index("UFST")


class TestICMSST:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMSST": {"CST": "41", "orig": "0",
                                        "vBCSTRet": "0", "vICMSSTRet": "0",
                                        "vBCSTDest": "0", "vICMSSTDest": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_vBCSTRet_before_vICMSSTRet(self):
        xml = to_xml_bytes({"ICMSST": {"orig": "0", "CST": "41",
                                        "vICMSSTRet": "0", "vBCSTRet": "0",
                                        "vBCSTDest": "0", "vICMSSTDest": "0"}})
        kids = _children(xml)
        assert kids.index("vBCSTRet") < kids.index("vICMSSTRet")


class TestICMSSN201:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMSSN201": {"CSOSN": "201", "orig": "0",
                                           "modBCST": "4", "vBCST": "0",
                                           "pICMSST": "0", "vICMSST": "0",
                                           "pCredSN": "0", "vCredICMSSN": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_pCredSN_after_vICMSST(self):
        xml = to_xml_bytes({"ICMSSN201": {"orig": "0", "CSOSN": "201",
                                           "modBCST": "4", "vBCST": "0",
                                           "pICMSST": "0", "vICMSST": "0",
                                           "vCredICMSSN": "0", "pCredSN": "0"}})
        kids = _children(xml)
        assert kids.index("vICMSST") < kids.index("pCredSN")


class TestICMSSN202:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMSSN202": {"CSOSN": "202", "orig": "0",
                                           "modBCST": "4", "vBCST": "0",
                                           "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_CSOSN_before_modBCST(self):
        xml = to_xml_bytes({"ICMSSN202": {"orig": "0", "modBCST": "4",
                                           "CSOSN": "202", "vBCST": "0",
                                           "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("CSOSN") < kids.index("modBCST")


class TestICMSSN900:
    def test_orig_first(self):
        xml = to_xml_bytes({"ICMSSN900": {"CSOSN": "900", "orig": "0", "modBC": "3",
                                           "vBC": "0", "pICMS": "12", "vICMS": "0",
                                           "modBCST": "4", "vBCST": "0",
                                           "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids[0] == "orig"

    def test_CSOSN_before_modBC(self):
        xml = to_xml_bytes({"ICMSSN900": {"orig": "0", "modBC": "3",
                                           "CSOSN": "900", "vBC": "0",
                                           "pICMS": "12", "vICMS": "0",
                                           "modBCST": "4", "vBCST": "0",
                                           "pICMSST": "0", "vICMSST": "0"}})
        kids = _children(xml)
        assert kids.index("CSOSN") < kids.index("modBC")


class TestICMSUFDest:
    def test_vBCUFDest_first(self):
        xml = to_xml_bytes({"ICMSUFDest": {
            "vICMSUFDest": "0", "pICMSUFDest": "0",
            "pICMSInter": "7", "vBCUFDest": "100",
        }})
        kids = _children(xml)
        assert kids[0] == "vBCUFDest"

    def test_pICMSInter_before_pICMSInterPart(self):
        xml = to_xml_bytes({"ICMSUFDest": {
            "vBCUFDest": "100", "vBCFCPUFDest": "0",
            "pFCPUFDest": "0", "pICMSUFDest": "0",
            "pICMSInterPart": "60", "pICMSInter": "7",
            "vFCPUFDest": "0", "vICMSUFDest": "0", "vICMSUFRemet": "0",
        }})
        kids = _children(xml)
        assert kids.index("pICMSInter") < kids.index("pICMSInterPart")

    def test_vICMSUFDest_before_vICMSUFRemet(self):
        xml = to_xml_bytes({"ICMSUFDest": {
            "vBCUFDest": "100", "pICMSInter": "7",
            "vICMSUFRemet": "0", "vICMSUFDest": "0",
        }})
        kids = _children(xml)
        assert kids.index("vICMSUFDest") < kids.index("vICMSUFRemet")


# ---------------------------------------------------------------------------
# IPI
# ---------------------------------------------------------------------------

class TestIPI:
    def test_cEnq_before_IPITrib(self):
        xml = to_xml_bytes({"IPI": {"IPITrib": {"CST": "50"}, "cEnq": "999"}})
        kids = _children(xml)
        assert kids.index("cEnq") < kids.index("IPITrib")

    def test_IPITrib_CST_first(self):
        xml = to_xml_bytes({"IPITrib": {"vIPI": "0", "pIPI": "0", "vBC": "100", "CST": "50"}})
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_IPITrib_vBC_before_pIPI(self):
        xml = to_xml_bytes({"IPITrib": {"CST": "50", "pIPI": "0", "vBC": "100", "vIPI": "0"}})
        kids = _children(xml)
        assert kids.index("vBC") < kids.index("pIPI")

    def test_CNPJProd_before_cEnq(self):
        xml = to_xml_bytes({"IPI": {"cEnq": "999", "CNPJProd": "1"}})
        kids = _children(xml)
        assert kids.index("CNPJProd") < kids.index("cEnq")


# ---------------------------------------------------------------------------
# PIS / COFINS (remaining)
# ---------------------------------------------------------------------------

class TestPISCOFINSRemaining:
    def test_PISQtde_CST_first(self):
        xml = to_xml_bytes({"PISQtde": {"vPIS": "0", "vAliqProd": "0", "qBCProd": "1", "CST": "03"}})
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_PISST_vBC_first(self):
        xml = to_xml_bytes({"PISST": {"indSomaPISST": "1", "vPIS": "0", "pPIS": "0",
                                       "vAliqProd": "0", "qBCProd": "1", "vBC": "100"}})
        kids = _children(xml)
        assert kids[0] == "vBC"

    def test_PISST_indSomaPISST_last(self):
        xml = to_xml_bytes({"PISST": {"vBC": "100", "pPIS": "0",
                                       "qBCProd": "1", "vAliqProd": "0",
                                       "vPIS": "0", "indSomaPISST": "1"}})
        kids = _children(xml)
        assert kids[-1] == "indSomaPISST"

    def test_COFINSQtde_CST_first(self):
        xml = to_xml_bytes({"COFINSQtde": {"vCOFINS": "0", "vAliqProd": "0",
                                            "qBCProd": "1", "CST": "03"}})
        kids = _children(xml)
        assert kids[0] == "CST"

    def test_COFINSOutr_CST_before_vBC(self):
        xml = to_xml_bytes({"COFINSOutr": {"vCOFINS": "0", "pCOFINS": "0",
                                            "vBC": "100", "CST": "99"}})
        kids = _children(xml)
        assert kids.index("CST") < kids.index("vBC")

    def test_COFINSST_vBC_first(self):
        xml = to_xml_bytes({"COFINSST": {"indSomaCOFINSST": "1", "vCOFINS": "0",
                                          "pCOFINS": "0", "qBCProd": "1",
                                          "vAliqProd": "0", "vBC": "100"}})
        kids = _children(xml)
        assert kids[0] == "vBC"

    def test_COFINSST_indSomaCOFINSST_last(self):
        xml = to_xml_bytes({"COFINSST": {"vBC": "100", "pCOFINS": "0",
                                          "qBCProd": "1", "vAliqProd": "0",
                                          "vCOFINS": "0", "indSomaCOFINSST": "1"}})
        kids = _children(xml)
        assert kids[-1] == "indSomaCOFINSST"


# ---------------------------------------------------------------------------
# IBSTot / CBSTot
# ---------------------------------------------------------------------------

class TestIBSCBSTotals:
    def _build(self, gIBS=None, gCBS=None) -> bytes:
        data: dict = {"vBCIBSCBS": "100"}
        if gIBS is not None:
            data["gIBS"] = gIBS
        if gCBS is not None:
            data["gCBS"] = gCBS
        return to_xml_bytes({"IBSCBSTot": data})

    def _default_gIBS(self, **overrides):
        base = {
            "gIBSUF": {"vDif": "0.00", "vDevTrib": "0.00", "vIBSUF": "5.00"},
            "gIBSMun": {"vDif": "0.00", "vDevTrib": "0.00", "vIBSMun": "3.00"},
            "vIBS": "8.00",
            "vCredPres": "0.00",
            "vCredPresCondSus": "0.00",
        }
        base.update(overrides)
        return base

    def test_gIBS_gIBSUF_first(self):
        xml = self._build(gIBS=self._default_gIBS())
        kids = _children(xml, "gIBS")
        assert kids[0] == "gIBSUF"

    def test_gIBS_gIBSMun_before_vIBS(self):
        xml = self._build(gIBS=self._default_gIBS())
        kids = _children(xml, "gIBS")
        assert kids.index("gIBSMun") < kids.index("vIBS")

    def test_gIBS_vIBS_before_vCredPres(self):
        xml = self._build(gIBS=self._default_gIBS())
        kids = _children(xml, "gIBS")
        assert kids.index("vIBS") < kids.index("vCredPres")

    def test_gIBSUF_vDif_first(self):
        xml = self._build(gIBS=self._default_gIBS())
        kids = _children(xml, "gIBS/gIBSUF")
        assert kids[0] == "vDif"

    def test_gIBSUF_vDevTrib_before_vIBSUF(self):
        xml = self._build(gIBS={
            "gIBSUF": {"vIBSUF": "5.00", "vDevTrib": "0.00", "vDif": "0.00"},
            "gIBSMun": {"vDif": "0.00", "vDevTrib": "0.00", "vIBSMun": "3.00"},
            "vIBS": "8.00", "vCredPres": "0.00", "vCredPresCondSus": "0.00",
        })
        kids = _children(xml, "gIBS/gIBSUF")
        assert kids.index("vDevTrib") < kids.index("vIBSUF")

    def test_gIBSMun_vDif_first(self):
        xml = self._build(gIBS=self._default_gIBS())
        kids = _children(xml, "gIBS/gIBSMun")
        assert kids[0] == "vDif"

    def test_gCBS_vDif_first(self):
        xml = self._build(gCBS={
            "vCBS": "10.00", "vDevTrib": "0.00",
            "vDif": "0.00", "vCredPres": "0.00", "vCredPresCondSus": "0.00",
        })
        kids = _children(xml, "gCBS")
        assert kids[0] == "vDif"

    def test_gCBS_vCBS_before_vCredPres(self):
        xml = self._build(gCBS={
            "vDif": "0.00", "vDevTrib": "0.00",
            "vCredPres": "0.00", "vCBS": "10.00", "vCredPresCondSus": "0.00",
        })
        kids = _children(xml, "gCBS")
        assert kids.index("vCBS") < kids.index("vCredPres")

    def test_vBCIBSCBS_before_gIBS(self):
        xml = self._build(gIBS=self._default_gIBS())
        kids = _children(xml)
        assert kids.index("vBCIBSCBS") < kids.index("gIBS")


# ---------------------------------------------------------------------------
# transp sub-elements
# ---------------------------------------------------------------------------

class TestTransporta:
    def test_CNPJ_first(self):
        xml = to_xml_bytes({"transporta": {"UF": "SP", "xNome": "X", "CNPJ": "1"}})
        kids = _children(xml)
        assert kids[0] == "CNPJ"

    def test_xNome_before_IE(self):
        xml = to_xml_bytes({"transporta": {"CNPJ": "1", "IE": "1", "xNome": "X"}})
        kids = _children(xml)
        assert kids.index("xNome") < kids.index("IE")

    def test_IE_before_xEnder(self):
        xml = to_xml_bytes({"transporta": {"CNPJ": "1", "xEnder": "R", "IE": "1"}})
        kids = _children(xml)
        assert kids.index("IE") < kids.index("xEnder")


class TestRetTransp:
    def test_vServ_first(self):
        xml = to_xml_bytes({"retTransp": {"cMunFG": "1", "CFOP": "5352",
                                           "vICMSRet": "0", "pICMSRet": "0",
                                           "vBCRet": "0", "vServ": "100"}})
        kids = _children(xml)
        assert kids[0] == "vServ"

    def test_CFOP_before_cMunFG(self):
        xml = to_xml_bytes({"retTransp": {"vServ": "100", "vBCRet": "0",
                                           "pICMSRet": "0", "vICMSRet": "0",
                                           "cMunFG": "1", "CFOP": "5352"}})
        kids = _children(xml)
        assert kids.index("CFOP") < kids.index("cMunFG")


class TestVeicTranspReboque:
    def test_veicTransp_placa_first(self):
        xml = to_xml_bytes({"veicTransp": {"RNTC": "1", "UF": "SP", "placa": "ABC1234"}})
        kids = _children(xml)
        assert kids[0] == "placa"

    def test_reboque_placa_first(self):
        xml = to_xml_bytes({"reboque": {"RNTC": "1", "UF": "SP", "placa": "ABC1234"}})
        kids = _children(xml)
        assert kids[0] == "placa"

    def test_veicTransp_UF_before_RNTC(self):
        xml = to_xml_bytes({"veicTransp": {"placa": "ABC1234", "RNTC": "1", "UF": "SP"}})
        kids = _children(xml)
        assert kids.index("UF") < kids.index("RNTC")


# ---------------------------------------------------------------------------
# cobr / fat / dup
# ---------------------------------------------------------------------------

class TestCobrFatDup:
    def test_cobr_fat_before_dup(self):
        xml = to_xml_bytes({"cobr": {"dup": {"nDup": "001"}, "fat": {"nFat": "1"}}})
        kids = _children(xml)
        assert kids.index("fat") < kids.index("dup")

    def test_fat_nFat_first(self):
        xml = to_xml_bytes({"fat": {"vLiq": "100", "vDesc": "0", "vOrig": "100", "nFat": "001"}})
        kids = _children(xml)
        assert kids[0] == "nFat"

    def test_fat_vOrig_before_vDesc(self):
        xml = to_xml_bytes({"fat": {"nFat": "001", "vDesc": "0", "vOrig": "100", "vLiq": "100"}})
        kids = _children(xml)
        assert kids.index("vOrig") < kids.index("vDesc")

    def test_dup_nDup_first(self):
        xml = to_xml_bytes({"dup": {"vDup": "100", "dVenc": "2025-01-01", "nDup": "001"}})
        kids = _children(xml)
        assert kids[0] == "nDup"

    def test_dup_dVenc_before_vDup(self):
        xml = to_xml_bytes({"dup": {"nDup": "001", "vDup": "100", "dVenc": "2025-01-01"}})
        kids = _children(xml)
        assert kids.index("dVenc") < kids.index("vDup")


# ---------------------------------------------------------------------------
# infAdic / infObs / exporta / compra / infRespTec / infNFeSupl
# ---------------------------------------------------------------------------

class TestInfoAdic:
    def test_infCpl_before_infObs(self):
        xml = to_xml_bytes({"infAdic": {"infObs": {"xCampo": "F"}, "infCpl": "obs"}})
        kids = _children(xml)
        assert kids.index("infCpl") < kids.index("infObs")

    def test_infObs_xCampo_before_xTexto(self):
        xml = to_xml_bytes({"infObs": {"xTexto": "V", "xCampo": "F"}})
        kids = _children(xml)
        assert kids.index("xCampo") < kids.index("xTexto")


class TestExportaCompra:
    def test_exporta_UFSaidaPais_first(self):
        xml = to_xml_bytes({"exporta": {"xLocDespacho": "D",
                                         "xLocExporta": "X", "UFSaidaPais": "SP"}})
        kids = _children(xml)
        assert kids[0] == "UFSaidaPais"

    def test_exporta_xLocExporta_before_xLocDespacho(self):
        xml = to_xml_bytes({"exporta": {"UFSaidaPais": "SP",
                                         "xLocDespacho": "D", "xLocExporta": "X"}})
        kids = _children(xml)
        assert kids.index("xLocExporta") < kids.index("xLocDespacho")

    def test_compra_xNEmp_first(self):
        xml = to_xml_bytes({"compra": {"xCont": "C", "xPed": "P", "xNEmp": "N"}})
        kids = _children(xml)
        assert kids[0] == "xNEmp"

    def test_compra_xPed_before_xCont(self):
        xml = to_xml_bytes({"compra": {"xNEmp": "N", "xCont": "C", "xPed": "P"}})
        kids = _children(xml)
        assert kids.index("xPed") < kids.index("xCont")


class TestInfoRespTec:
    def test_CNPJ_first(self):
        xml = to_xml_bytes({"infRespTec": {"hashCSRT": "H", "idCSRT": "1",
                                            "fone": "1", "email": "e@e.com",
                                            "xContato": "X", "CNPJ": "1"}})
        kids = _children(xml)
        assert kids[0] == "CNPJ"

    def test_xContato_before_email(self):
        xml = to_xml_bytes({"infRespTec": {"CNPJ": "1", "email": "e@e.com", "xContato": "X"}})
        kids = _children(xml)
        assert kids.index("xContato") < kids.index("email")

    def test_idCSRT_before_hashCSRT(self):
        xml = to_xml_bytes({"infRespTec": {"CNPJ": "1", "hashCSRT": "H",
                                            "xContato": "X", "idCSRT": "1"}})
        kids = _children(xml)
        assert kids.index("idCSRT") < kids.index("hashCSRT")


class TestInfoNFeSupl:
    def test_qrCode_before_urlChave(self):
        xml = to_xml_bytes({"infNFeSupl": {"urlChave": "http://x", "qrCode": "QR"}})
        kids = _children(xml)
        assert kids.index("qrCode") < kids.index("urlChave")


# ---------------------------------------------------------------------------
# ConsCad
# ---------------------------------------------------------------------------

class TestConsCad:
    def test_infCons_xServ_first(self):
        xml = to_xml_bytes({"ConsCad": {"infCons": {
            "IE": "1", "CNPJ": "1", "UF": "SP", "xServ": "CONS-CAD",
        }}})
        kids = _children(xml, "infCons")
        assert kids[0] == "xServ"

    def test_infCons_UF_before_CNPJ(self):
        xml = to_xml_bytes({"ConsCad": {"infCons": {
            "CNPJ": "1", "UF": "SP", "xServ": "CONS-CAD",
        }}})
        kids = _children(xml, "infCons")
        assert kids.index("UF") < kids.index("CNPJ")


# ---------------------------------------------------------------------------
# CT-e — enviCTe / CTe / infCte
# ---------------------------------------------------------------------------

class TestEnviCTe:
    def test_idLote_before_CTe(self):
        xml = to_xml_bytes({"enviCTe": {"CTe": {}, "idLote": "1"}})
        kids = _children(xml)
        assert kids.index("idLote") < kids.index("CTe")


class TestCTe:
    def test_infCte_before_infCteSupl(self):
        xml = to_xml_bytes({"CTe": {"infCteSupl": {}, "infCte": {}}})
        kids = _children(xml)
        assert kids.index("infCte") < kids.index("infCteSupl")


class TestInfCte:
    def test_ide_before_emit(self):
        xml = to_xml_bytes({"infCte": {
            "emit": {"CNPJ": "1"}, "ide": {"cUF": "35"},
        }})
        kids = _children(xml)
        assert kids.index("ide") < kids.index("emit")

    def test_emit_before_rem(self):
        xml = to_xml_bytes({"infCte": {
            "rem": {"CNPJ": "1"}, "ide": {"cUF": "35"}, "emit": {"CNPJ": "1"},
        }})
        kids = _children(xml)
        assert kids.index("emit") < kids.index("rem")

    def test_dest_before_vPrest(self):
        xml = to_xml_bytes({"infCte": {
            "vPrest": {"vTPrest": "0"}, "dest": {"CNPJ": "1"},
            "ide": {"cUF": "35"},
        }})
        kids = _children(xml)
        assert kids.index("dest") < kids.index("vPrest")

    def test_infRespTec_after_autXML(self):
        xml = to_xml_bytes({"infCte": {
            "infRespTec": {"CNPJ": "1"}, "autXML": {"CNPJ": "1"},
            "ide": {"cUF": "35"},
        }})
        kids = _children(xml)
        assert kids.index("autXML") < kids.index("infRespTec")


# ---------------------------------------------------------------------------
# CT-e — rem / exped / receb / dest (IE before xNome)
# ---------------------------------------------------------------------------

class TestRemExpedRecebDestCTe:
    def test_rem_CNPJ_first(self):
        xml = to_xml_bytes({"rem": {"xNome": "X", "IE": "1", "CNPJ": "1"}})
        kids = _children(xml)
        assert kids[0] == "CNPJ"

    def test_rem_IE_before_xNome(self):
        xml = to_xml_bytes({"rem": {"CNPJ": "1", "xNome": "X", "IE": "1"}})
        kids = _children(xml)
        assert kids.index("IE") < kids.index("xNome")

    def test_exped_IE_before_xNome(self):
        xml = to_xml_bytes({"exped": {"CNPJ": "1", "xNome": "X", "IE": "1"}})
        kids = _children(xml)
        assert kids.index("IE") < kids.index("xNome")

    def test_receb_IE_before_xNome(self):
        xml = to_xml_bytes({"receb": {"CNPJ": "1", "xNome": "X", "IE": "1"}})
        kids = _children(xml)
        assert kids.index("IE") < kids.index("xNome")

    def test_infCte_dest_IE_before_xNome(self):
        xml = to_xml_bytes({"infCte": {"dest": {"CNPJ": "1", "xNome": "X", "IE": "1"}}})
        kids = _children(xml, "dest")
        assert kids.index("IE") < kids.index("xNome")


# ---------------------------------------------------------------------------
# CT-e — vPrest / Comp / imp
# ---------------------------------------------------------------------------

class TestVPrest:
    def test_vTPrest_first(self):
        xml = to_xml_bytes({"vPrest": {"Comp": {"xNome": "X"}, "vRec": "0", "vTPrest": "100"}})
        kids = _children(xml)
        assert kids[0] == "vTPrest"

    def test_vRec_before_Comp(self):
        xml = to_xml_bytes({"vPrest": {"Comp": {"xNome": "X"}, "vTPrest": "100", "vRec": "0"}})
        kids = _children(xml)
        assert kids.index("vRec") < kids.index("Comp")


class TestComp:
    def test_xNome_before_vComp(self):
        xml = to_xml_bytes({"Comp": {"vComp": "100", "xNome": "Frete"}})
        kids = _children(xml)
        assert kids.index("xNome") < kids.index("vComp")


class TestImpCTe:
    def test_ICMS_first(self):
        xml = to_xml_bytes({"imp": {"vTotTrib": "0", "ICMS": {}}})
        kids = _children(xml)
        assert kids[0] == "ICMS"

    def test_vTotTrib_after_ICMSUFIni(self):
        xml = to_xml_bytes({"imp": {"ICMS": {}, "vTotTrib": "0",
                                     "ICMSUFFim": {}, "ICMSUFIni": {}}})
        kids = _children(xml)
        assert kids.index("ICMSUFIni") < kids.index("vTotTrib")

    def test_IBS_before_CBS(self):
        xml = to_xml_bytes({"imp": {"ICMS": {}, "CBS": {}, "IBS": {}}})
        kids = _children(xml)
        assert kids.index("IBS") < kids.index("CBS")


class TestConsSitCTe:
    def test_tpAmb_first(self):
        xml = to_xml_bytes({"consSitCTe": {"chCTe": "111", "xServ": "CONSULTAR", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids[0] == "tpAmb"

    def test_xServ_before_chCTe(self):
        xml = to_xml_bytes({"consSitCTe": {"chCTe": "111", "xServ": "CONSULTAR", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids.index("xServ") < kids.index("chCTe")


class TestEvCancCTe:
    def test_descEvento_first(self):
        xml = to_xml_bytes({"evCancCTe": {"xJust": "J", "nProt": "135", "descEvento": "Cancelamento"}})
        kids = _children(xml)
        assert kids[0] == "descEvento"

    def test_nProt_before_xJust(self):
        xml = to_xml_bytes({"evCancCTe": {"descEvento": "Cancelamento", "xJust": "J", "nProt": "135"}})
        kids = _children(xml)
        assert kids.index("nProt") < kids.index("xJust")


# ---------------------------------------------------------------------------
# MDF-e — enviMDFe / MDFe / infMDFe
# ---------------------------------------------------------------------------

class TestEnviMDFe:
    def test_idLote_before_MDFe(self):
        xml = to_xml_bytes({"enviMDFe": {"MDFe": {}, "idLote": "1"}})
        kids = _children(xml)
        assert kids.index("idLote") < kids.index("MDFe")


class TestMDFe:
    def test_infMDFe_before_infMDFeSupl(self):
        xml = to_xml_bytes({"MDFe": {"infMDFeSupl": {}, "infMDFe": {}}})
        kids = _children(xml)
        assert kids.index("infMDFe") < kids.index("infMDFeSupl")


class TestInfMDFe:
    def test_ide_before_emit(self):
        xml = to_xml_bytes({"infMDFe": {"emit": {"CNPJ": "1"}, "ide": {"cUF": "35"}}})
        kids = _children(xml)
        assert kids.index("ide") < kids.index("emit")

    def test_emit_before_infModal(self):
        xml = to_xml_bytes({"infMDFe": {
            "infModal": {}, "emit": {"CNPJ": "1"}, "ide": {"cUF": "35"},
        }})
        kids = _children(xml)
        assert kids.index("emit") < kids.index("infModal")

    def test_infDoc_before_tot(self):
        xml = to_xml_bytes({"infMDFe": {
            "tot": {"qCTe": "1"}, "infDoc": {}, "ide": {"cUF": "35"},
        }})
        kids = _children(xml)
        assert kids.index("infDoc") < kids.index("tot")

    def test_infRespTec_after_autXML(self):
        xml = to_xml_bytes({"infMDFe": {
            "ide": {"cUF": "35"},
            "infRespTec": {"CNPJ": "1"}, "autXML": {"CNPJ": "1"},
        }})
        kids = _children(xml)
        assert kids.index("autXML") < kids.index("infRespTec")


# ---------------------------------------------------------------------------
# MDF-e — infDoc / tot
# ---------------------------------------------------------------------------

class TestInfDoc:
    def test_infMunDescarga_is_only_child(self):
        # MDF-e 3.00: infMunCarrega moved into ide; infDoc only has infMunDescarga
        xml = to_xml_bytes({"infDoc": {"infMunDescarga": {"cMunDescarga": "1"}}})
        kids = _children(xml)
        assert kids == ["infMunDescarga"]

    def test_infMDFe_ide_infMunCarrega_before_dhIniViagem(self):
        xml = to_xml_bytes({"infMDFe": {"ide": {
            "dhIniViagem": "T",
            "infMunCarrega": {"cMunCarrega": "1", "xMunCarrega": "SP"},
            "cUF": "35", "tpAmb": "1", "tpEmit": "1", "mod": "58",
            "serie": "1", "nMDF": "1", "cMDF": "1", "cDV": "0",
            "modal": "1", "dhEmi": "T", "tpEmis": "1",
            "procEmi": "0", "verProc": "1.0", "UFIni": "SP", "UFFim": "RJ",
        }}})
        kids = _children(xml, "ide")
        assert kids.index("infMunCarrega") < kids.index("dhIniViagem")

    def test_infMunDescarga_cMunDescarga_first(self):
        xml = to_xml_bytes({"infMunDescarga": {
            "infNFe": {}, "xMunDescarga": "SP", "cMunDescarga": "1",
        }})
        kids = _children(xml)
        assert kids[0] == "cMunDescarga"

    def test_infMunDescarga_xMunDescarga_before_infNFe(self):
        xml = to_xml_bytes({"infMunDescarga": {
            "infNFe": {}, "cMunDescarga": "1", "xMunDescarga": "SP",
        }})
        kids = _children(xml)
        assert kids.index("xMunDescarga") < kids.index("infNFe")


class TestInfMDFeTot:
    def test_qCTe_first(self):
        xml = to_xml_bytes({"infMDFe": {"tot": {
            "qCarga": "10", "cUnid": "01", "vCarga": "100",
            "qMDFe": "0", "qNFe": "0", "qCTe": "1",
        }}})
        kids = _children(xml, "tot")
        assert kids[0] == "qCTe"

    def test_vCarga_before_cUnid(self):
        xml = to_xml_bytes({"infMDFe": {"tot": {
            "qCTe": "1", "qNFe": "0", "qMDFe": "0",
            "cUnid": "01", "vCarga": "100", "qCarga": "10",
        }}})
        kids = _children(xml, "tot")
        assert kids.index("vCarga") < kids.index("cUnid")


# ---------------------------------------------------------------------------
# MDF-e — consSitMDFe / consNaoEncMDFe
# ---------------------------------------------------------------------------

class TestConsSitMDFe:
    def test_tpAmb_first(self):
        xml = to_xml_bytes({"consSitMDFe": {"chMDFe": "111", "xServ": "CONSULTAR", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids[0] == "tpAmb"

    def test_xServ_before_chMDFe(self):
        xml = to_xml_bytes({"consSitMDFe": {"chMDFe": "111", "xServ": "CONSULTAR", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids.index("xServ") < kids.index("chMDFe")


class TestConsNaoEncMDFe:
    def test_tpAmb_first(self):
        xml = to_xml_bytes({"consNaoEncMDFe": {"CNPJ": "1", "xServ": "CONS-NAO-ENC", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids[0] == "tpAmb"

    def test_xServ_before_CNPJ(self):
        xml = to_xml_bytes({"consNaoEncMDFe": {"CNPJ": "1", "xServ": "CONS-NAO-ENC", "tpAmb": "2"}})
        kids = _children(xml)
        assert kids.index("xServ") < kids.index("CNPJ")


# ---------------------------------------------------------------------------
# MDF-e — eventoMDFe / evEncMDFe / evCancMDFe / evIncCondutorMDFe
# ---------------------------------------------------------------------------

class TestEventoMDFe:
    def test_infEvento_first(self):
        xml = to_xml_bytes({"eventoMDFe": {"Signature": {}, "infEvento": {"cOrgao": "35"}}})
        kids = _children(xml)
        assert kids[0] == "infEvento"

    def test_infEvento_cOrgao_first(self):
        xml = to_xml_bytes({"eventoMDFe": {"infEvento": {
            "detEvento": {}, "nSeqEvento": "1",
            "tpEvento": "110111", "dhEvento": "T",
            "chMDFe": "1", "CNPJ": "1", "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids[0] == "cOrgao"

    def test_infEvento_chMDFe_before_dhEvento(self):
        xml = to_xml_bytes({"eventoMDFe": {"infEvento": {
            "dhEvento": "T", "chMDFe": "1",
            "tpEvento": "110111", "nSeqEvento": "1",
            "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids.index("chMDFe") < kids.index("dhEvento")

    def test_infEvento_detEvento_last(self):
        xml = to_xml_bytes({"eventoMDFe": {"infEvento": {
            "detEvento": {}, "nSeqEvento": "1",
            "tpEvento": "110111", "dhEvento": "T",
            "chMDFe": "1", "CNPJ": "1", "tpAmb": "2", "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert kids[-1] == "detEvento"


class TestEvEncMDFe:
    def test_descEvento_first(self):
        xml = to_xml_bytes({"evEncMDFe": {
            "cMun": "1", "cUF": "35", "dtEnc": "2025-01-01",
            "nProt": "135", "descEvento": "Encerramento",
        }})
        kids = _children(xml)
        assert kids[0] == "descEvento"

    def test_nProt_before_dtEnc(self):
        xml = to_xml_bytes({"evEncMDFe": {
            "descEvento": "Encerramento",
            "dtEnc": "2025-01-01", "nProt": "135",
            "cUF": "35", "cMun": "1",
        }})
        kids = _children(xml)
        assert kids.index("nProt") < kids.index("dtEnc")

    def test_cUF_before_cMun(self):
        xml = to_xml_bytes({"evEncMDFe": {
            "descEvento": "Encerramento", "nProt": "135",
            "dtEnc": "2025-01-01", "cMun": "1", "cUF": "35",
        }})
        kids = _children(xml)
        assert kids.index("cUF") < kids.index("cMun")


class TestEvCancMDFe:
    def test_descEvento_first(self):
        xml = to_xml_bytes({"evCancMDFe": {"xJust": "J", "nProt": "135", "descEvento": "Cancelamento"}})
        kids = _children(xml)
        assert kids[0] == "descEvento"

    def test_nProt_before_xJust(self):
        xml = to_xml_bytes({"evCancMDFe": {"descEvento": "Cancelamento", "xJust": "J", "nProt": "135"}})
        kids = _children(xml)
        assert kids.index("nProt") < kids.index("xJust")


class TestevIncCondutorMDFe:
    def test_descEvento_before_condutor(self):
        xml = to_xml_bytes({"evIncCondutorMDFe": {
            "condutor": {"xNome": "X", "CPF": "1"},
            "descEvento": "Inclusão Condutor",
        }})
        kids = _children(xml)
        assert kids.index("descEvento") < kids.index("condutor")

    def test_condutor_xNome_before_CPF(self):
        xml = to_xml_bytes({"condutor": {"CPF": "1", "xNome": "X"}})
        kids = _children(xml)
        assert kids.index("xNome") < kids.index("CPF")
