"""Synthetic authorized NFC-e XML for DANFC-e tests. No real CNPJ/CPF."""

from __future__ import annotations

_NS = "http://www.portalfiscal.inf.br/nfe"
_CHAVE = "35260612345678000199650010000000011000000017"


def _items(n: int) -> str:
    rows = []
    for i in range(1, n + 1):
        rows.append(
            f"""<det nItem="{i}">
  <prod>
    <cProd>P{i:03d}</cProd>
    <xProd>PRODUTO TESTE {i}</xProd>
    <uCom>UN</uCom>
    <qCom>2.0000</qCom>
    <vUnCom>10.0000000000</vUnCom>
    <vProd>20.00</vProd>
  </prod>
</det>"""
        )
    return "\n".join(rows)


def sample_nfe_proc(
    *, tp_emis: str = "1", tp_amb: str = "1", with_dest: bool = True, n_items: int = 2
) -> str:
    dest = (
        """<dest>
    <CPF>12345678909</CPF>
    <xNome>CONSUMIDOR TESTE</xNome>
  </dest>"""
        if with_dest
        else ""
    )
    total_prod = f"{20.00 * n_items:.2f}"
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="{_NS}" versao="4.00">
  <NFe>
    <infNFe Id="NFe{_CHAVE}" versao="4.00">
      <ide>
        <cUF>35</cUF>
        <mod>65</mod>
        <serie>1</serie>
        <nNF>1</nNF>
        <dhEmi>2026-06-25T10:30:00-03:00</dhEmi>
        <tpAmb>{tp_amb}</tpAmb>
        <tpEmis>{tp_emis}</tpEmis>
      </ide>
      <emit>
        <CNPJ>12345678000199</CNPJ>
        <xNome>EMPRESA TESTE LTDA</xNome>
        <enderEmit>
          <xLgr>RUA EXEMPLO</xLgr>
          <nro>100</nro>
          <xBairro>CENTRO</xBairro>
          <xMun>SAO PAULO</xMun>
          <UF>SP</UF>
          <CEP>01000000</CEP>
        </enderEmit>
      </emit>
      {dest}
      {_items(n_items)}
      <total>
        <ICMSTot>
          <vProd>{total_prod}</vProd>
          <vFrete>0.00</vFrete>
          <vSeg>0.00</vSeg>
          <vDesc>0.00</vDesc>
          <vOutro>0.00</vOutro>
          <vNF>{total_prod}</vNF>
          <vTotTrib>5.00</vTotTrib>
        </ICMSTot>
      </total>
      <pag>
        <detPag>
          <tPag>01</tPag>
          <vPag>{total_prod}</vPag>
        </detPag>
        <vTroco>0.00</vTroco>
      </pag>
      <infAdic>
        <infAdFisco>Mensagem fiscal de teste</infAdFisco>
        <infCpl>Obrigado pela preferencia</infCpl>
      </infAdic>
    </infNFe>
    <infNFeSupl>
      <qrCode>https://www.fazenda.sp.gov.br/nfce/qrcode?p={_CHAVE}|2|{tp_amb}|1|ABCDEF1234567890ABCDEF1234567890ABCDEF12</qrCode>
      <urlChave>https://www.fazenda.sp.gov.br/nfce/consulta</urlChave>
    </infNFeSupl>
  </NFe>
  <protNFe versao="4.00">
    <infProt>
      <chNFe>{_CHAVE}</chNFe>
      <nProt>135260000000017</nProt>
      <dhRecbto>2026-06-25T10:30:05-03:00</dhRecbto>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso da NF-e</xMotivo>
    </infProt>
  </protNFe>
</nfeProc>"""


_CHAVE_55 = "35260612345678000199550010000000011000000017"


def _items55(n: int) -> str:
    rows = []
    for i in range(1, n + 1):
        rows.append(
            f"""<det nItem="{i}">
  <prod>
    <cProd>P{i:03d}</cProd>
    <xProd>PRODUTO TESTE {i}</xProd>
    <NCM>61099000</NCM>
    <CFOP>5102</CFOP>
    <uCom>UN</uCom>
    <qCom>2.0000</qCom>
    <vUnCom>10.0000000000</vUnCom>
    <vProd>20.00</vProd>
  </prod>
  <imposto>
    <ICMS>
      <ICMS00>
        <CST>00</CST>
        <vBC>20.00</vBC>
        <pICMS>18.00</pICMS>
        <vICMS>3.60</vICMS>
      </ICMS00>
    </ICMS>
  </imposto>
</det>"""
        )
    return "\n".join(rows)


def sample_nfe55_proc(
    *,
    tp_emis: str = "1",
    tp_amb: str = "1",
    n_items: int = 2,
    with_transp: bool = True,
    with_dup: bool = True,
    with_issqn: bool = False,
) -> str:
    total_prod = f"{20.00 * n_items:.2f}"
    transp = (
        """<transp>
    <modFrete>0</modFrete>
    <transporta>
      <xNome>TRANSPORTADORA TESTE LTDA</xNome>
      <CNPJ>98765432000188</CNPJ>
      <IE>ISENTO</IE>
      <xEnder>RUA DO FRETE, 50</xEnder>
      <xMun>SAO PAULO</xMun>
      <UF>SP</UF>
    </transporta>
    <veicTransp><placa>ABC1D23</placa><UF>SP</UF></veicTransp>
    <vol><qVol>1</qVol><esp>CAIXA</esp><pesoB>1.000</pesoB><pesoL>0.900</pesoL></vol>
  </transp>"""
        if with_transp
        else ""
    )
    cobr = (
        f"""<cobr>
    <fat><nFat>001</nFat><vOrig>{total_prod}</vOrig><vLiq>{total_prod}</vLiq></fat>
    <dup><nDup>001</nDup><dVenc>2026-07-25</dVenc><vDup>{total_prod}</vDup></dup>
  </cobr>"""
        if with_dup
        else ""
    )
    issqn = (
        """<ISSQNtot><vServ>0.00</vServ><vBC>0.00</vBC><vISS>0.00</vISS></ISSQNtot>"""
        if with_issqn
        else ""
    )
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="{_NS}" versao="4.00">
  <NFe>
    <infNFe Id="NFe{_CHAVE_55}" versao="4.00">
      <ide>
        <cUF>35</cUF>
        <natOp>VENDA DE MERCADORIA</natOp>
        <mod>55</mod>
        <serie>1</serie>
        <nNF>1</nNF>
        <dhEmi>2026-06-25T10:30:00-03:00</dhEmi>
        <dhSaiEnt>2026-06-25T10:35:00-03:00</dhSaiEnt>
        <tpNF>1</tpNF>
        <tpAmb>{tp_amb}</tpAmb>
        <tpEmis>{tp_emis}</tpEmis>
      </ide>
      <emit>
        <CNPJ>12345678000199</CNPJ>
        <xNome>EMPRESA TESTE LTDA</xNome>
        <xFant>EMPRESA TESTE</xFant>
        <enderEmit>
          <xLgr>RUA EXEMPLO</xLgr>
          <nro>100</nro>
          <xBairro>CENTRO</xBairro>
          <xMun>SAO PAULO</xMun>
          <UF>SP</UF>
          <CEP>01000000</CEP>
          <fone>1133334444</fone>
        </enderEmit>
        <IE>110042490114</IE>
        <CRT>3</CRT>
      </emit>
      <dest>
        <CNPJ>98765432000188</CNPJ>
        <xNome>CLIENTE TESTE LTDA</xNome>
        <enderDest>
          <xLgr>AV CLIENTE</xLgr>
          <nro>200</nro>
          <xBairro>JARDIM</xBairro>
          <xMun>CAMPINAS</xMun>
          <UF>SP</UF>
          <CEP>13000000</CEP>
        </enderDest>
        <IE>ISENTO</IE>
      </dest>
      {_items55(n_items)}
      <total>
        <ICMSTot>
          <vBC>{total_prod}</vBC>
          <vICMS>{20.00 * n_items * 0.18:.2f}</vICMS>
          <vBCST>0.00</vBCST>
          <vST>0.00</vST>
          <vProd>{total_prod}</vProd>
          <vFrete>0.00</vFrete>
          <vSeg>0.00</vSeg>
          <vDesc>0.00</vDesc>
          <vOutro>0.00</vOutro>
          <vIPI>0.00</vIPI>
          <vNF>{total_prod}</vNF>
          <vTotTrib>5.00</vTotTrib>
        </ICMSTot>
        {issqn}
      </total>
      {transp}
      {cobr}
      <infAdic>
        <infAdFisco>Informacao ao fisco de teste</infAdFisco>
        <infCpl>Documento emitido por ME optante pelo Simples Nacional</infCpl>
      </infAdic>
    </infNFe>
  </NFe>
  <protNFe versao="4.00">
    <infProt>
      <chNFe>{_CHAVE_55}</chNFe>
      <nProt>135260000000099</nProt>
      <dhRecbto>2026-06-25T10:30:05-03:00</dhRecbto>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso da NF-e</xMotivo>
    </infProt>
  </protNFe>
</nfeProc>"""


# ---------------------------------------------------------------------------
# DAMDFE (MDF-e modelo 58) fixture. Synthetic only — no real CNPJ/CPF.
# ---------------------------------------------------------------------------

_NS_MDFE = "http://www.portalfiscal.inf.br/mdfe"
_CHAVE_58 = "35260612345678000199580010000000011000000010"


def _mdfe_nfes(n: int) -> str:
    rows = []
    for i in range(1, n + 1):
        ch = f"35260612345678000199550010000000{i:03d}1000000017"
        rows.append(f"<infNFe><chNFe>{ch}</chNFe></infNFe>")
    return "\n".join(rows)


def sample_mdfe58_proc(
    *,
    tp_emis: str = "1",
    tp_amb: str = "1",
    modal: str = "1",
    n_docs: int = 2,
    with_seg: bool = True,
    with_prot: bool = True,
) -> str:
    modal_block = {
        "1": """<rodo>
          <infANTT><RNTRC>12345678</RNTRC></infANTT>
          <veicTracao>
            <placa>ABC1D23</placa><RENAVAM>123456789</RENAVAM>
            <tara>5000</tara><capKG>20000</capKG>
            <condutor><xNome>JOAO MOTORISTA</xNome><CPF>12345678909</CPF></condutor>
            <UF>SP</UF>
          </veicTracao>
        </rodo>""",
        "2": """<aereo><nac>BRA</nac><matr>PT1234</matr><nVoo>1234</nVoo>
          <cAerEmb>GRU</cAerEmb><cAerDes>GIG</cAerDes><dVoo>2026-06-25</dVoo></aereo>""",
        "3": """<aquav><irin>1234567</irin><tpEmb>0</tpEmb><cEmbar>EMB1</cEmbar>
          <xEmbar>NAVIO TESTE</xEmbar><nViag>01</nViag><cPrtEmb>BRSSZ</cPrtEmb>
          <cPrtDest>BRRIG</cPrtDest></aquav>""",
        "4": """<ferrov><trem><xPref>TREM01</xPref><xOri>SAO PAULO</xOri>
          <xDest>SANTOS</xDest><qVag>2</qVag></trem>
          <vag><serie>A</serie><nVag>001</nVag></vag></ferrov>""",
    }[modal]
    seg = (
        """<seg>
        <infResp><respSeg>1</respSeg><CNPJ>12345678000199</CNPJ></infResp>
        <infSeg><xSeg>SEGURADORA TESTE</xSeg><CNPJ>98765432000188</CNPJ></infSeg>
        <nApol>APOL123</nApol><nAver>AVER456</nAver>
      </seg>"""
        if with_seg
        else ""
    )
    prot = (
        f"""<protMDFe versao="3.00">
    <infProt>
      <tpAmb>{tp_amb}</tpAmb>
      <chMDFe>{_CHAVE_58}</chMDFe>
      <dhRecbto>2026-06-25T10:30:05-03:00</dhRecbto>
      <nProt>135260000000088</nProt>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso do MDF-e</xMotivo>
    </infProt>
  </protMDFe>"""
        if with_prot
        else ""
    )
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<mdfeProc xmlns="{_NS_MDFE}" versao="3.00">
  <MDFe>
    <infMDFe Id="MDFe{_CHAVE_58}" versao="3.00">
      <ide>
        <cUF>35</cUF>
        <tpAmb>{tp_amb}</tpAmb>
        <tpEmit>1</tpEmit>
        <tpTransp>1</tpTransp>
        <mod>58</mod>
        <serie>1</serie>
        <nMDF>1</nMDF>
        <cMDF>00000001</cMDF>
        <cDV>0</cDV>
        <modal>{modal}</modal>
        <dhEmi>2026-06-25T10:30:00-03:00</dhEmi>
        <tpEmis>{tp_emis}</tpEmis>
        <procEmi>0</procEmi>
        <verProc>1.0</verProc>
        <UFIni>SP</UFIni>
        <UFFim>RJ</UFFim>
        <infMunCarrega><cMunCarrega>3550308</cMunCarrega><xMunCarrega>SAO PAULO</xMunCarrega></infMunCarrega>
        <infPercurso><UFPer>MG</UFPer></infPercurso>
      </ide>
      <emit>
        <CNPJ>12345678000199</CNPJ>
        <IE>110042490114</IE>
        <xNome>TRANSPORTADORA TESTE LTDA</xNome>
        <xFant>TRANSP TESTE</xFant>
        <enderEmit>
          <xLgr>RUA DO TRANSPORTE</xLgr>
          <nro>500</nro>
          <xBairro>CENTRO</xBairro>
          <cMun>3550308</cMun>
          <xMun>SAO PAULO</xMun>
          <CEP>01000000</CEP>
          <UF>SP</UF>
          <fone>1133334444</fone>
        </enderEmit>
      </emit>
      <infModal versaoModal="3.00">
        {modal_block}
      </infModal>
      <infDoc>
        <infMunDescarga>
          <cMunDescarga>3304557</cMunDescarga>
          <xMunDescarga>RIO DE JANEIRO</xMunDescarga>
          {_mdfe_nfes(n_docs)}
        </infMunDescarga>
      </infDoc>
      {seg}
      <prodPred>
        <tpCarga>05</tpCarga>
        <xProd>CARGA GERAL TESTE</xProd>
        <NCM>22030000</NCM>
      </prodPred>
      <tot>
        <qNFe>{n_docs}</qNFe>
        <vCarga>10000.00</vCarga>
        <cUnid>01</cUnid>
        <qCarga>1500.0000</qCarga>
      </tot>
      <lacres><nLacre>LAC001</nLacre></lacres>
      <infAdic>
        <infAdFisco>Informacao ao fisco de teste</infAdFisco>
        <infCpl>Observacao complementar do MDF-e de teste</infCpl>
      </infAdic>
    </infMDFe>
    <infMDFeSupl>
      <qrCodMDFe>https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode?chMDFe={_CHAVE_58}&amp;tpAmb={tp_amb}</qrCodMDFe>
    </infMDFeSupl>
  </MDFe>
  {prot}
</mdfeProc>"""
