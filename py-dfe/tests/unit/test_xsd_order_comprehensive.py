"""Comprehensive ordering tests — one test class per root/webservice.

Each class builds a JSON with ALL possible fields in wrong/reversed order and
asserts the complete correct XSD-defined order is maintained after serialisation.

Pattern reused from test_xsd_order.py.
"""

import pytest
from lxml import etree

from py_dfe.xmlops.builder import to_xml_bytes


# ---------------------------------------------------------------------------
# Helpers (same as test_xsd_order.py)
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


# =============================================================================
# NF-e / NFC-e — enviNFe
# =============================================================================

class TestEnviNFeComprehensive:
    """Test enviNFe root: all infNFe children in wrong order -> correct XML order."""

    def test_infNFe_full_order(self):
        """Full infNFe with all top-level children in reversed order."""
        xml = to_xml_bytes({"infNFe": {
            "@versao": "4.00",
            # Reversed from XSD order to prove reordering works
            "infSolicNFF": {"xSolic": "x"},
            "infRespTec": {"CNPJ": "1", "xContato": "X", "email": "e@e.com", "fone": "1"},
            "cana": {"safra": "2024/2025", "ref": "R"},
            "compra": {"xNEmp": "N"},
            "exporta": {"UFSaidaPais": "SP"},
            "infAdic": {"infCpl": "obs"},
            "infIntermed": {"CNPJ": "1", "idCadIntTran": "X"},
            "pag": {"detPag": {"tPag": "01", "vPag": "10.00"}},
            "cobr": {"fat": {"nFat": "1"}},
            "transp": {"modFrete": "9"},
            "total": {"ICMSTot": {"vBC": "0", "vNF": "100"}},
            "det": [{"prod": {"cProd": "P", "xProd": "X"}}],
            "autXML": [{"CNPJ": "1"}],
            "entrega": {"CNPJ": "1"},
            "retirada": {"CNPJ": "1"},
            "dest": {"CNPJ": "2"},
            "avulsa": {"CNPJ": "1", "xOrgao": "X", "matr": "1", "xAgente": "A", "UF": "SP", "repEmi": "X"},
            "emit": {"CNPJ": "1", "xNome": "E", "CRT": "1"},
            "ide": {
                "cUF": "35", "cNF": "1", "natOp": "V", "mod": "55", "serie": "1",
                "nNF": "1", "dhEmi": "T", "tpNF": "1", "idDest": "1", "cMunFG": "1",
                "tpImp": "1", "tpEmis": "1", "cDV": "1", "tpAmb": "2",
                "finNFe": "1", "indFinal": "0", "indPres": "0", "procEmi": "0", "verProc": "1.0",
            },
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, [
            "ide", "emit", "avulsa", "dest", "retirada", "entrega", "autXML",
            "det", "total", "transp", "cobr", "pag",
            "infIntermed", "infAdic", "exporta", "compra", "cana",
            "infRespTec", "infSolicNFF",
        ])
        # spot-checks
        assert kids.index("ide") < kids.index("emit")
        assert kids.index("emit") < kids.index("avulsa")
        assert kids.index("det") < kids.index("total")
        assert kids.index("total") < kids.index("transp")
        assert kids.index("pag") < kids.index("infIntermed")
        assert kids[-1] == "infSolicNFF"

    def test_ide_full_order(self):
        """infNFe:ide all fields reversed -> correct order."""
        xml = to_xml_bytes({"infNFe": {"ide": {
            "xJust": "J",
            "dhCont": "T",
            "verProc": "1.0",
            "procEmi": "0",
            "indIntermed": "0",
            "indPres": "0",
            "indFinal": "0",
            "tpNFCredito": "1",
            "tpNFDebito": "1",
            "finNFe": "1",
            "tpAmb": "2",
            "cDV": "1",
            "tpEmis": "1",
            "tpImp": "1",
            "cMunFGIBS": "1",
            "cMunFG": "1",
            "idDest": "1",
            "tpNF": "1",
            "dhSaiEnt": "T",
            "dPrevEntrega": "2025-01-01",
            "dhEmi": "T",
            "nNF": "1",
            "serie": "1",
            "mod": "55",
            "natOp": "V",
            "cNF": "1",
            "cUF": "35",
        }}})
        kids = _children(xml, "ide")
        assert _order_is_correct(kids, [
            "cUF", "cNF", "natOp", "mod", "serie", "nNF",
            "dhEmi", "dhSaiEnt", "dPrevEntrega", "tpNF", "idDest", "cMunFG", "cMunFGIBS",
            "tpImp", "tpEmis", "cDV", "tpAmb", "finNFe",
            "tpNFDebito", "tpNFCredito",
            "indFinal", "indPres", "indIntermed", "procEmi", "verProc",
            "dhCont", "xJust",
        ])
        assert kids[0] == "cUF"
        assert kids.index("dhEmi") < kids.index("dhSaiEnt")
        assert kids.index("dPrevEntrega") < kids.index("tpNF")
        assert kids.index("cMunFG") < kids.index("cMunFGIBS")
        assert kids.index("finNFe") < kids.index("tpNFDebito")
        assert kids.index("xJust") > kids.index("verProc")

    def test_prod_full_order(self):
        """prod with all fields including new gCred, tpCredPresIBSZFM, indBemMovelUsado."""
        xml = to_xml_bytes({"prod": {
            "nRECOPI": "X",
            "comb": {"cProdANP": "1"},
            "arma": [{"tpArma": "1", "nSerie": "1", "nCano": "1", "descr": "D"}],
            "med": {"cProdANVISA": "1"},
            "veicProd": {"tpOp": "1"},
            "infProdEmb": {"xEmb": "X", "qVolEmb": "1", "uEmb": "UN"},
            "infProdNFF": {"cProdFisco": "1", "cOperNFF": "1"},
            "rastro": [{"nLote": "L", "qLote": "1", "dFab": "2025-01", "dVal": "2026-01", "cAgreg": "1"}],
            "nFCI": "X",
            "nItemPed": "1",
            "xPed": "P",
            "detExport": [{"nDraw": "1"}],
            "DI": [{"nDI": "1", "dDI": "2025-01-01", "xLocDesemb": "L", "UFDesemb": "SP",
                    "dDesemb": "2025-01-01", "tpViaTransp": "1", "tpIntermedio": "1",
                    "cExportador": "E"}],
            "indBemMovelUsado": "0",
            "indTot": "1",
            "vOutro": "0",
            "vDesc": "0",
            "vSeg": "0",
            "vFrete": "0",
            "vUnTrib": "1",
            "qTrib": "1",
            "uTrib": "UN",
            "cBarraTrib": "SEM GTIN",
            "cEANTrib": "SEM GTIN",
            "vProd": "10",
            "vUnCom": "10",
            "qCom": "1",
            "uCom": "UN",
            "CFOP": "5102",
            "EXTIPI": "01",
            "tpCredPresIBSZFM": "1",
            "gCred": {"cCredPresumido": "C", "pCredPresumido": "1", "vCredPresumido": "1"},
            "cBenef": "C",
            "CNPJFab": "1",
            "indEscala": "S",
            "CEST": "1",
            "NVE": "A1234B",
            "NCM": "12345678",
            "xProd": "Produto",
            "cBarra": "SEM GTIN",
            "cEAN": "SEM GTIN",
            "cProd": "P001",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, [
            "cProd", "cEAN", "cBarra", "xProd", "NCM", "NVE", "CEST",
            "indEscala", "CNPJFab", "cBenef", "gCred", "tpCredPresIBSZFM", "EXTIPI",
            "CFOP", "uCom", "qCom", "vUnCom", "vProd",
            "cEANTrib", "cBarraTrib", "uTrib", "qTrib", "vUnTrib",
            "vFrete", "vSeg", "vDesc", "vOutro", "indTot", "indBemMovelUsado",
            "DI", "detExport", "xPed", "nItemPed", "nFCI", "rastro",
            "infProdNFF", "infProdEmb", "veicProd", "med", "arma", "comb", "nRECOPI",
        ])
        assert kids[0] == "cProd"
        assert kids.index("cBenef") < kids.index("gCred")
        assert kids.index("gCred") < kids.index("tpCredPresIBSZFM")
        assert kids.index("indTot") < kids.index("indBemMovelUsado")

    def test_detPag_new_structure(self):
        """detPag uses new XSD structure: indPag tPag xPag vPag dPag CNPJPag UFPag card."""
        xml = to_xml_bytes({"detPag": {
            "card": {"tpIntegra": "1", "CNPJ": "1", "tBand": "01", "cAut": "X"},
            "UFPag": "SP",
            "CNPJPag": "1",
            "dPag": "2025-01-01",
            "vPag": "10.00",
            "xPag": "PIX",
            "tPag": "17",
            "indPag": "0",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, ["indPag", "tPag", "xPag", "vPag", "dPag", "CNPJPag", "UFPag", "card"])
        assert kids[0] == "indPag"
        assert kids[-1] == "card"

    def test_infAdic_new_structure(self):
        """infAdic now has infAdFisco, infCpl, obsCont, obsFisco, procRef."""
        xml = to_xml_bytes({"infAdic": {
            "procRef": [{"nProc": "1", "indProc": "0", "tpAto": "1"}],
            "obsFisco": [{"xTexto": "F"}],
            "obsCont": [{"xTexto": "C"}],
            "infCpl": "obs complementar",
            "infAdFisco": "Fisco",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, ["infAdFisco", "infCpl", "obsCont", "obsFisco", "procRef"])
        assert kids[0] == "infAdFisco"
        assert kids.index("infCpl") < kids.index("obsCont")

    def test_transp_vol_at_end(self):
        """vol should be last in transp, after balsa."""
        xml = to_xml_bytes({"transp": {
            "vol": [{"qVol": "1", "pesoL": "1", "pesoB": "2"}],
            "balsa": "B",
            "vagao": "V",
            "reboque": [{"placa": "ABC1234", "UF": "SP"}],
            "veicTransp": {"placa": "XYZ9999", "UF": "SP"},
            "retTransp": {"vServ": "100", "vBCRet": "0", "pICMSRet": "0",
                          "vICMSRet": "0", "CFOP": "5352", "cMunFG": "1"},
            "transporta": {"CNPJ": "1"},
            "modFrete": "9",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, [
            "modFrete", "transporta", "retTransp",
            "veicTransp", "reboque", "vagao", "balsa", "vol",
        ])
        assert kids[0] == "modFrete"
        assert kids[-1] == "vol"

    def test_new_icms_types(self):
        """ICMS02, ICMS15, ICMS53 new types have correct ordering."""
        xml02 = to_xml_bytes({"ICMS02": {
            "vICMSMono": "1", "adRemICMS": "1", "qBCMono": "1", "CST": "02", "orig": "0",
        }})
        kids02 = _children(xml02)
        assert kids02 == ["orig", "CST", "qBCMono", "adRemICMS", "vICMSMono"]

        xml15 = to_xml_bytes({"ICMS15": {
            "motRedAdRem": "1",
            "pRedAdRem": "1",
            "vICMSMonoReten": "1",
            "adRemICMSReten": "1",
            "qBCMonoReten": "1",
            "vICMSMono": "1",
            "adRemICMS": "1",
            "qBCMono": "1",
            "CST": "15",
            "orig": "0",
        }})
        kids15 = _children(xml15)
        assert _order_is_correct(kids15, [
            "orig", "CST", "qBCMono", "adRemICMS", "vICMSMono",
            "qBCMonoReten", "adRemICMSReten", "vICMSMonoReten", "pRedAdRem", "motRedAdRem",
        ])
        assert kids15[0] == "orig"

        xml53 = to_xml_bytes({"ICMS53": {
            "adRemICMSDif": "1",
            "qBCMonoDif": "1",
            "vICMSMono": "1",
            "vICMSMonoDif": "1",
            "pDif": "1",
            "vICMSMonoOp": "1",
            "adRemICMS": "1",
            "qBCMono": "1",
            "CST": "53",
            "orig": "0",
        }})
        kids53 = _children(xml53)
        assert _order_is_correct(kids53, [
            "orig", "CST", "qBCMono", "adRemICMS", "vICMSMonoOp",
            "pDif", "vICMSMonoDif", "vICMSMono", "qBCMonoDif", "adRemICMSDif",
        ])

    def test_icms10_new_tail(self):
        """ICMS10 appends vICMSSTDeson, motDesICMSST at end."""
        xml = to_xml_bytes({"ICMS10": {
            "motDesICMSST": "9",
            "vICMSSTDeson": "0",
            "vFCPST": "0", "pFCPST": "0", "vBCFCPST": "0",
            "vFCP": "0", "pFCP": "0", "vBCFCP": "0",
            "vICMSST": "0", "pICMSST": "0",
            "vBCST": "0", "pRedBCST": "0", "pMVAST": "0", "modBCST": "4",
            "vICMS": "0", "pICMS": "0", "vBC": "0", "modBC": "3",
            "CST": "10", "orig": "0",
        }})
        kids = _children(xml)
        assert kids[0] == "orig"
        assert kids.index("vICMSST") < kids.index("vICMSSTDeson")
        assert kids.index("vICMSSTDeson") < kids.index("motDesICMSST")

    def test_icms51_new_fields(self):
        """ICMS51 has cBenefRBC after pRedBC and new FCP tail fields."""
        xml = to_xml_bytes({"ICMS51": {
            "vFCPEfet": "0",
            "vFCPDif": "0",
            "pFCPDif": "0",
            "vFCP": "0", "pFCP": "0", "vBCFCP": "0",
            "vICMS": "0",
            "vICMSDif": "0", "pDif": "0", "vICMSOp": "0",
            "pICMS": "0", "vBC": "0",
            "cBenefRBC": "C",
            "pRedBC": "0",
            "modBC": "3",
            "CST": "51", "orig": "0",
        }})
        kids = _children(xml)
        assert kids[0] == "orig"
        assert kids.index("pRedBC") < kids.index("cBenefRBC")
        assert kids.index("cBenefRBC") < kids.index("vBC")
        assert kids.index("vFCP") < kids.index("pFCPDif")
        assert kids.index("pFCPDif") < kids.index("vFCPDif")
        assert kids.index("vFCPDif") < kids.index("vFCPEfet")


# =============================================================================
# NF-e — envEvento
# =============================================================================

class TestEnvEventoNFeComprehensive:
    """Test envEvento + evento:infEvento + detEvento in wrong order."""

    def test_envEvento_full_order(self):
        """envEvento: idLote before evento."""
        xml = to_xml_bytes({"envEvento": {
            "evento": [{"infEvento": {
                "detEvento": {"descEvento": "Cancelamento", "nProt": "135", "xJust": "J"},
                "verEvento": "1.00",
                "nSeqEvento": "1",
                "tpEvento": "110111",
                "dhEvento": "T",
                "chNFe": "1" * 44,
                "CPF": "12345678901",
                "tpAmb": "2",
                "cOrgao": "35",
            }}],
            "idLote": "1",
        }})
        kids = _children(xml)
        assert kids.index("idLote") < kids.index("evento")

    def test_infEvento_full_order(self):
        """evento:infEvento children in reversed order -> correct."""
        xml = to_xml_bytes({"evento": {"infEvento": {
            "detEvento": {"descEvento": "Cancelamento"},
            "verEvento": "1.00",
            "nSeqEvento": "1",
            "tpEvento": "110111",
            "dhEvento": "2025-01-01T00:00:00-03:00",
            "chNFe": "1" * 44,
            "CNPJ": "12345678000195",
            "tpAmb": "2",
            "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert _order_is_correct(kids, [
            "cOrgao", "tpAmb", "CNPJ", "chNFe",
            "dhEvento", "tpEvento", "nSeqEvento", "verEvento", "detEvento",
        ])
        assert kids[0] == "cOrgao"
        assert kids[-1] == "detEvento"

    def test_detEvento_full_order(self):
        """detEvento: descEvento, nProt, xJust."""
        xml = to_xml_bytes({"detEvento": {
            "xJust": "Erro de emissão",
            "nProt": "135000000001",
            "descEvento": "Cancelamento",
        }})
        kids = _children(xml)
        assert kids[0] == "descEvento"
        assert kids.index("nProt") < kids.index("xJust")


# =============================================================================
# NF-e — consStatServ
# =============================================================================

class TestConsStatServComprehensive:
    """Test consStatServ ordering."""

    def test_full_order(self):
        xml = to_xml_bytes({"consStatServ": {
            "xServ": "STATUS", "cUF": "35", "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "cUF", "xServ"]
        assert kids[0] == "tpAmb"
        assert kids[-1] == "xServ"


# =============================================================================
# NF-e — consSitNFe
# =============================================================================

class TestConsSitNFeComprehensive:
    """Test consSitNFe ordering."""

    def test_full_order(self):
        xml = to_xml_bytes({"consSitNFe": {
            "chNFe": "1" * 44,
            "xServ": "CONSULTAR",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "xServ", "chNFe"]
        assert kids[0] == "tpAmb"
        assert kids[-1] == "chNFe"


# =============================================================================
# NF-e — inutNFe
# =============================================================================

class TestInutNFeComprehensive:
    """Test inutNFe:infInut ordering with all fields reversed."""

    def test_infInut_full_order(self):
        xml = to_xml_bytes({"inutNFe": {"infInut": {
            "xJust": "Justificativa de inutilizacao",
            "nNFFin": "100",
            "nNFIni": "1",
            "serie": "1",
            "mod": "55",
            "CNPJ": "12345678000195",
            "ano": "25",
            "cUF": "35",
            "xServ": "INUTILIZAR",
            "tpAmb": "2",
        }}})
        kids = _children(xml, "infInut")
        assert _order_is_correct(kids, [
            "tpAmb", "xServ", "cUF", "ano",
            "CNPJ", "mod", "serie", "nNFIni", "nNFFin", "xJust",
        ])
        assert kids[0] == "tpAmb"
        assert kids[-1] == "xJust"
        assert kids.index("cUF") < kids.index("ano")
        assert kids.index("mod") < kids.index("serie")


# =============================================================================
# NF-e — ConsCad
# =============================================================================

class TestConsCadComprehensive:
    """Test ConsCad:infCons ordering with all fields reversed."""

    def test_infCons_full_order(self):
        xml = to_xml_bytes({"ConsCad": {"infCons": {
            "IE": "123456789",
            "CPF": "12345678901",
            "CNPJ": "12345678000195",
            "UF": "SP",
            "xServ": "CONS-CAD",
        }}})
        kids = _children(xml, "infCons")
        assert _order_is_correct(kids, ["xServ", "UF", "CNPJ", "CPF", "IE"])
        assert kids[0] == "xServ"
        assert kids.index("UF") < kids.index("CNPJ")
        assert kids.index("CNPJ") < kids.index("CPF")


# =============================================================================
# NF-e — distDFeInt
# =============================================================================

class TestDistDFeIntComprehensive:
    """Test distDFeInt ordering with all optional children reversed."""

    def test_distDFeInt_full_order(self):
        xml = to_xml_bytes({"distDFeInt": {
            "consChNFe": {"chNFe": "1" * 44},
            "consNSU": {"NSU": "000000000000001"},
            "distNSU": {"ultNSU": "000000000000000"},
            "CPF": "12345678901",
            "CNPJ": "12345678000195",
            "cUFAutor": "35",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, [
            "tpAmb", "cUFAutor", "CNPJ", "CPF", "distNSU", "consNSU", "consChNFe",
        ])
        assert kids[0] == "tpAmb"
        assert kids.index("CNPJ") < kids.index("distNSU")
        assert kids.index("distNSU") < kids.index("consNSU")


# =============================================================================
# CT-e — enviCTe
# =============================================================================

class TestEnviCTeComprehensive:
    """Test enviCTe and infCte:ide / infCte children ordering."""

    def test_enviCTe_full_order(self):
        xml = to_xml_bytes({"enviCTe": {"CTe": {}, "idLote": "1"}})
        kids = _children(xml)
        assert kids.index("idLote") < kids.index("CTe")

    def test_infCte_full_order(self):
        xml = to_xml_bytes({"infCte": {
            "infSolicNFF": {"xSolic": "X"},
            "infRespTec": {"CNPJ": "1", "xContato": "X", "email": "e@e.com", "fone": "1"},
            "autXML": [{"CNPJ": "1"}],
            "infCteComp": {},
            "infCteSub": {},
            "infCteNorm": {},
            "imp": {"ICMS": {}},
            "vPrest": {"vTPrest": "100", "vRec": "100"},
            "dest": {"CNPJ": "2"},
            "receb": {"CNPJ": "3"},
            "exped": {"CNPJ": "4"},
            "rem": {"CNPJ": "5"},
            "emit": {"CNPJ": "1", "IE": "1", "xNome": "E", "CRT": "1"},
            "compl": {"xObs": "obs"},
            "ide": {
                "cUF": "35", "cCT": "1", "CFOP": "5352", "natOp": "V",
                "mod": "57", "serie": "1", "nCT": "1", "dhEmi": "T",
                "tpImp": "1", "tpEmis": "1", "cDV": "1", "tpAmb": "2",
                "tpCTe": "0", "procEmi": "0", "verProc": "1.0",
                "cMunEnv": "1", "xMunEnv": "SP", "UFEnv": "SP",
                "modal": "01", "indIEToma": "1",
            },
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, [
            "ide", "compl", "emit", "rem", "exped", "receb", "dest",
            "vPrest", "imp", "infCteNorm", "infCteSub", "infCteComp",
            "autXML", "infRespTec", "infSolicNFF",
        ])
        assert kids[0] == "ide"
        assert kids.index("emit") < kids.index("rem")
        assert kids[-1] == "infSolicNFF"

    def test_infCte_ide_full_order(self):
        """infCte:ide with tpServ after modal."""
        xml = to_xml_bytes({"infCte": {"ide": {
            "xMunFim": "X",
            "cMunFim": "1",
            "UFFim": "SP",
            "xMunIni": "X",
            "cMunIni": "1",
            "UFIni": "SP",
            "xJust": "J",
            "dhCont": "T",
            "toma4": {},
            "toma3": {},
            "indCarga": "1",
            "indIEToma": "1",
            "dhIniViagem": "T",
            "tpServ": "1",
            "modal": "01",
            "UFEnv": "SP",
            "xMunEnv": "SP",
            "cMunEnv": "1",
            "indGlobalizado": "1",
            "verProc": "1.0",
            "procEmi": "0",
            "tpCTe": "0",
            "tpAmb": "2",
            "cDV": "1",
            "tpEmis": "1",
            "tpImp": "1",
            "dhEmi": "T",
            "nCT": "1",
            "serie": "1",
            "mod": "57",
            "natOp": "V",
            "CFOP": "5352",
            "cCT": "1",
            "cUF": "35",
        }}})
        kids = _children(xml, "ide")
        assert _order_is_correct(kids, [
            "cUF", "cCT", "CFOP", "natOp", "mod", "serie", "nCT",
            "dhEmi", "tpImp", "tpEmis", "cDV", "tpAmb", "tpCTe",
            "procEmi", "verProc", "indGlobalizado",
            "cMunEnv", "xMunEnv", "UFEnv",
            "modal", "tpServ", "dhIniViagem", "indIEToma",
            "indCarga", "toma3", "toma4",
            "dhCont", "xJust",
            "UFIni", "cMunIni", "xMunIni", "UFFim", "cMunFim", "xMunFim",
        ])
        assert kids[0] == "cUF"
        assert kids.index("modal") < kids.index("tpServ")
        assert kids.index("tpServ") < kids.index("dhIniViagem")


# =============================================================================
# CT-e — consStatServCTe
# =============================================================================

class TestConsStatServCTeComprehensive:
    """Test consStatServCTe ordering."""

    def test_full_order(self):
        xml = to_xml_bytes({"consStatServCTe": {
            "xServ": "STATUS", "cUF": "35", "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "cUF", "xServ"]
        assert kids[0] == "tpAmb"
        assert kids[-1] == "xServ"


# =============================================================================
# CT-e — consSitCTe
# =============================================================================

class TestConsSitCTeComprehensive:
    """Test consSitCTe ordering."""

    def test_full_order(self):
        xml = to_xml_bytes({"consSitCTe": {
            "chCTe": "1" * 44,
            "xServ": "CONSULTAR",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "xServ", "chCTe"]
        assert kids[0] == "tpAmb"
        assert kids[-1] == "chCTe"


# =============================================================================
# CT-e — eventoCTe
# =============================================================================

class TestEventoCTeComprehensive:
    """Test eventoCTe:infEvento and evCancCTe ordering."""

    def test_infEvento_full_order(self):
        xml = to_xml_bytes({"eventoCTe": {"infEvento": {
            "detEvento": {"descEvento": "Cancelamento"},
            "nSeqEvento": "1",
            "tpEvento": "110111",
            "dhEvento": "2025-01-01T00:00:00-03:00",
            "chCTe": "1" * 44,
            "CNPJ": "12345678000195",
            "tpAmb": "2",
            "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert _order_is_correct(kids, [
            "cOrgao", "tpAmb", "CNPJ", "chCTe",
            "dhEvento", "tpEvento", "nSeqEvento", "detEvento",
        ])
        assert kids[0] == "cOrgao"
        assert kids[-1] == "detEvento"
        assert kids.index("chCTe") < kids.index("dhEvento")

    def test_evCancCTe_full_order(self):
        xml = to_xml_bytes({"evCancCTe": {
            "xJust": "Justificativa",
            "nProt": "135000000001",
            "descEvento": "Cancelamento",
        }})
        kids = _children(xml)
        assert kids[0] == "descEvento"
        assert kids.index("nProt") < kids.index("xJust")


# =============================================================================
# MDF-e — enviMDFe
# =============================================================================

class TestEnviMDFeComprehensive:
    """Test enviMDFe and infMDFe children ordering."""

    def test_infMDFe_full_order(self):
        """infMDFe with all children in reversed order."""
        xml = to_xml_bytes({"infMDFe": {
            "infPAA": {"CNPJPAA": "1"},
            "infSolicNFF": {"xSolic": "X"},
            "infRespTec": {"CNPJ": "1", "xContato": "X", "email": "e@e.com", "fone": "1"},
            "infAdic": {"infCpl": "obs"},
            "autXML": [{"CNPJ": "1"}],
            "lacres": [{"nLacre": "1"}],
            "tot": {"qCTe": "1", "qNFe": "0", "qMDFe": "0", "vCarga": "100", "cUnid": "01", "qCarga": "1"},
            "prodPred": {"tpCarga": "01", "xProd": "Produto"},
            "seg": [{"infResp": {"respSeg": "1", "CNPJ": "1"}}],
            "infDoc": {"infMunDescarga": [{"cMunDescarga": "1", "xMunDescarga": "SP"}]},
            "infModal": {},
            "emit": {"CNPJ": "1", "xNome": "E", "enderEmit": {"xLgr": "R", "nro": "1",
                      "xBairro": "B", "cMun": "1", "xMun": "SP", "UF": "SP"}},
            "ide": {
                "cUF": "35", "tpAmb": "2", "tpEmit": "1", "tpTransp": "1",
                "mod": "58", "serie": "1", "nMDF": "1", "cMDF": "1", "cDV": "1",
                "modal": "01", "dhEmi": "T", "tpEmis": "1", "procEmi": "0",
                "verProc": "1.0", "UFIni": "SP", "UFFim": "SP",
                "dhIniViagem": "T",
            },
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, [
            "ide", "emit", "infModal", "infDoc",
            "seg", "prodPred", "tot", "lacres", "autXML",
            "infAdic", "infRespTec", "infSolicNFF", "infPAA",
        ])
        assert kids[0] == "ide"
        assert kids.index("infDoc") < kids.index("seg")
        assert kids.index("autXML") < kids.index("infAdic")
        assert kids[-1] == "infPAA"

    def test_infMDFe_ide_full_order(self):
        """infMDFe:ide with infMunCarrega and infPercurso between UFFim and dhIniViagem."""
        xml = to_xml_bytes({"infMDFe": {"ide": {
            "indCarregaPosterior": "1",
            "indCanalVerde": "1",
            "dhIniViagem": "T",
            "infPercurso": [{"UFPer": "RJ"}],
            "infMunCarrega": [{"cMunCarrega": "1", "xMunCarrega": "SP"}],
            "UFFim": "SP",
            "UFIni": "SP",
            "verProc": "1.0",
            "procEmi": "0",
            "tpEmis": "1",
            "dhEmi": "T",
            "modal": "01",
            "cDV": "1",
            "cMDF": "1",
            "nMDF": "1",
            "serie": "1",
            "mod": "58",
            "tpTransp": "1",
            "tpEmit": "1",
            "tpAmb": "2",
            "cUF": "35",
        }}})
        kids = _children(xml, "ide")
        assert _order_is_correct(kids, [
            "cUF", "tpAmb", "tpEmit", "tpTransp", "mod", "serie", "nMDF",
            "cMDF", "cDV", "modal", "dhEmi", "tpEmis",
            "procEmi", "verProc", "UFIni", "UFFim",
            "infMunCarrega", "infPercurso",
            "dhIniViagem", "indCanalVerde", "indCarregaPosterior",
        ])
        assert kids[0] == "cUF"
        assert kids.index("UFFim") < kids.index("infMunCarrega")
        assert kids.index("infMunCarrega") < kids.index("infPercurso")
        assert kids.index("infPercurso") < kids.index("dhIniViagem")

    def test_infDoc_only_infMunDescarga(self):
        """infDoc should now contain only infMunDescarga (infMunCarrega moved to ide)."""
        xml = to_xml_bytes({"infDoc": {
            "infMunDescarga": [{"cMunDescarga": "1", "xMunDescarga": "SP"}],
        }})
        kids = _children(xml)
        assert kids == ["infMunDescarga"]

    def test_infMDFe_infAdic(self):
        """MDF-e infAdic uses context key infMDFe:infAdic -> only infAdFisco, infCpl."""
        xml = to_xml_bytes({"infMDFe": {
            "infAdic": {"infCpl": "obs", "infAdFisco": "Fisco"},
        }})
        kids = _children(xml, "infAdic")
        assert _order_is_correct(kids, ["infAdFisco", "infCpl"])
        assert kids[0] == "infAdFisco"


# =============================================================================
# MDF-e — consStatServMDFe
# =============================================================================

class TestConsStatServMDFeComprehensive:
    """Test consStatServMDFe ordering."""

    def test_full_order(self):
        xml = to_xml_bytes({"consStatServMDFe": {
            "xServ": "STATUS",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "xServ"]
        assert kids[0] == "tpAmb"
        assert kids[-1] == "xServ"


# =============================================================================
# MDF-e — consSitMDFe
# =============================================================================

class TestConsSitMDFeComprehensive:
    """Test consSitMDFe ordering."""

    def test_full_order(self):
        xml = to_xml_bytes({"consSitMDFe": {
            "chMDFe": "1" * 44,
            "xServ": "CONSULTAR",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert kids == ["tpAmb", "xServ", "chMDFe"]
        assert kids[0] == "tpAmb"
        assert kids[-1] == "chMDFe"


# =============================================================================
# MDF-e — consNaoEncMDFe
# =============================================================================

class TestConsNaoEncMDFeComprehensive:
    """Test consNaoEncMDFe ordering."""

    def test_full_order_cnpj(self):
        xml = to_xml_bytes({"consNaoEncMDFe": {
            "CNPJ": "12345678000195",
            "xServ": "CONS-NAO-ENC",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, ["tpAmb", "xServ", "CNPJ"])
        assert kids[0] == "tpAmb"
        assert kids.index("xServ") < kids.index("CNPJ")

    def test_full_order_cpf(self):
        xml = to_xml_bytes({"consNaoEncMDFe": {
            "CPF": "12345678901",
            "xServ": "CONS-NAO-ENC",
            "tpAmb": "2",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, ["tpAmb", "xServ", "CPF"])
        assert kids[0] == "tpAmb"


# =============================================================================
# MDF-e — eventoMDFe
# =============================================================================

class TestEventoMDFeComprehensive:
    """Test eventoMDFe:infEvento and ev* children ordering."""

    def test_infEvento_full_order(self):
        xml = to_xml_bytes({"eventoMDFe": {"infEvento": {
            "detEvento": {},
            "nSeqEvento": "1",
            "tpEvento": "110111",
            "dhEvento": "2025-01-01T00:00:00-03:00",
            "chMDFe": "1" * 44,
            "CNPJ": "12345678000195",
            "tpAmb": "2",
            "cOrgao": "35",
        }}})
        kids = _children(xml, "infEvento")
        assert _order_is_correct(kids, [
            "cOrgao", "tpAmb", "CNPJ", "chMDFe",
            "dhEvento", "tpEvento", "nSeqEvento", "detEvento",
        ])
        assert kids[0] == "cOrgao"
        assert kids[-1] == "detEvento"
        assert kids.index("chMDFe") < kids.index("dhEvento")

    def test_evEncMDFe_full_order(self):
        xml = to_xml_bytes({"evEncMDFe": {
            "cMun": "3550308",
            "cUF": "35",
            "dtEnc": "2025-01-01",
            "nProt": "135000000001",
            "descEvento": "Encerramento",
        }})
        kids = _children(xml)
        assert _order_is_correct(kids, ["descEvento", "nProt", "dtEnc", "cUF", "cMun"])
        assert kids[0] == "descEvento"
        assert kids.index("nProt") < kids.index("dtEnc")
        assert kids.index("cUF") < kids.index("cMun")

    def test_evCancMDFe_full_order(self):
        xml = to_xml_bytes({"evCancMDFe": {
            "xJust": "Justificativa",
            "nProt": "135000000001",
            "descEvento": "Cancelamento",
        }})
        kids = _children(xml)
        assert kids[0] == "descEvento"
        assert kids.index("nProt") < kids.index("xJust")

    def test_evIncCondutorMDFe_full_order(self):
        xml = to_xml_bytes({"evIncCondutorMDFe": {
            "condutor": {"CPF": "12345678901", "xNome": "Motorista"},
            "descEvento": "Inclusao Condutor",
        }})
        kids = _children(xml)
        assert kids[0] == "descEvento"
        assert kids.index("descEvento") < kids.index("condutor")
        # condutor children
        kids_cond = _children(xml, "condutor")
        assert kids_cond.index("xNome") < kids_cond.index("CPF")
