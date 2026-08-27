"""Minimal fiscal document payload builders for HOMOLOGAÇÃO integration tests.

Uses CNPJ 11647612000197 (A O CARVALHO TECH, PI/SVRS).
build_nfe/build_nfce/build_cte accept `cuf` and `cmunfg` to target any authorizer.
"""

from __future__ import annotations

import datetime
import random

_CNPJ = "11647612000197"
_CUF_PI = "22"
_CMUN_TERESINA = "2211001"
_ENV = "2"
_CORGAO_SVRS = "91"
_TZ = datetime.timezone(datetime.timedelta(hours=-3))
_CPF_MOTORISTA = "11144477735"

UF_PARAMS: dict[str, tuple[str, str, str, str]] = {
    "PI": ("22", "2211001", "Teresina", "PI"),
    "AM": ("13", "1302603", "Manaus", "AM"),
    "BA": ("29", "2927408", "Salvador", "BA"),
    "GO": ("52", "5208707", "Goiania", "GO"),
    "MG": ("31", "3106200", "Belo Horizonte", "MG"),
    "MS": ("50", "5002704", "Campo Grande", "MS"),
    "MT": ("51", "5103403", "Cuiaba", "MT"),
    "PE": ("26", "2611606", "Recife", "PE"),
    "PR": ("41", "4106902", "Curitiba", "PR"),
    "RS": ("43", "4314902", "Porto Alegre", "RS"),
    "SP": ("35", "3550308", "Sao Paulo", "SP"),
    "RJ": ("33", "3304557", "Rio de Janeiro", "RJ"),
}


def _now() -> datetime.datetime:
    return datetime.datetime.now(tz=_TZ)


def _dh() -> str:
    return _now().strftime("%Y-%m-%dT%H:%M:%S-03:00")


def _date() -> str:
    return _now().strftime("%Y-%m-%d")


def _aamm() -> str:
    n = _now()
    return f"{n.year % 100:02d}{n.month:02d}"


def _mod11_check(digits: str) -> str:
    total = sum(int(d) * (2 + i % 8) for i, d in enumerate(reversed(digits)))
    r = 11 - (total % 11)
    return "0" if r >= 10 else str(r)


def _chave(cuf: str, mod: str, serie: str | int, ndoc: str | int,
           tp_emis: str = "1", cnpj: str = _CNPJ) -> tuple[str, str, str]:
    """Return (chave_44, cnf_8digits, cdv)."""
    aamm = _aamm()
    cnf = f"{random.randint(1, 99_999_999):08d}"
    base = f"{cuf}{aamm}{cnpj}{mod}{int(serie):03d}{int(ndoc):09d}{tp_emis}{cnf}"
    cdv = _mod11_check(base)
    return base + cdv, cnf, cdv


def _endereco(cmun: str, xmun: str, uf: str, cep: str = "00000000") -> dict:
    return {
        "xLgr": "RUA TESTE HOMOLOGACAO",
        "nro": "1",
        "xBairro": "CENTRO",
        "cMun": cmun,
        "xMun": xmun,
        "UF": uf,
        "CEP": "64057060",
        "cPais": "1058",
        "xPais": "Brasil",
    }


def _uf_endereco() -> dict:
    return _endereco(_CMUN_TERESINA, "Teresina", "PI", "64000000")


def build_nfe(
        serie: int = 1,
        nnf: int | None = None,
        uf: str = "PI",
        mod: str = '55',
        fin_nfe: str = "1",
        nfref: list[dict] | None = None,
        vols: list[dict] | None = None,
        reboques: list[dict] | None = None,
        inf_adic: dict | None = None,
        resp_tec: dict | None = None,
) -> tuple[dict, str]:
    """Minimal NF-e (mod=55). Pass uf= to target a specific authorizer's cUF.
    fin_nfe/nfref cobrem devolução e complementar (ide/NFref).
    Returns (payload, chave_44)."""
    cuf, cmunfg, xmun, uf_sig = UF_PARAMS.get(uf, UF_PARAMS["PI"])
    if nnf is None:
        nnf = random.randint(800_000_001, 899_999_999)
    chave, cnf, cdv = _chave(cuf, mod, serie, nnf)
    dh = _dh()
    ender = _endereco(cmunfg, xmun, uf_sig)

    payload = {
        "enviNFe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "idLote": str(random.randint(1, 999_999_999)),
            "indSinc": "1",
            "NFe": {
                "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                "infNFe": {
                    "@versao": "4.00",
                    "@Id": f"NFe{chave}",
                    "ide": {
                        "cUF": cuf,
                        "cNF": cnf,
                        "natOp": "VENDA",
                        "mod": mod,
                        "serie": str(serie),
                        "nNF": str(nnf),
                        "dhEmi": dh,
                        "tpNF": "1",
                        "idDest": "1",
                        "cMunFG": cmunfg,
                        "tpImp": "1",
                        "tpEmis": "1",
                        "cDV": cdv,
                        "tpAmb": _ENV,
                        "finNFe": fin_nfe,
                        "indFinal": "0",
                        "indPres": "0",
                        "procEmi": "0",
                        "verProc": "py-dfe-1.0",
                    },
                    "emit": {
                        "CNPJ": _CNPJ,
                        "xNome": "A O CARVALHO TECH",
                        "xFant": "CTECH",
                        "enderEmit": ender,
                        "IE": "123456789",
                        "CRT": "1",
                    },
                    "dest": {
                        "CNPJ": _CNPJ,
                        "xNome": "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
                        "enderDest": ender,
                        "indIEDest": "9",
                    },
                    "det": {
                        "@nItem": "1",
                        "prod": {
                            "cProd": "001",
                            "cEAN": "SEM GTIN",
                            "xProd": "PRODUTO TESTE HOMOLOGACAO",
                            "NCM": "84713012",
                            "CFOP": "5102",
                            "uCom": "UN",
                            "qCom": "1.0000",
                            "vUnCom": "1.00",
                            "vProd": "1.00",
                            "cEANTrib": "SEM GTIN",
                            "uTrib": "UN",
                            "qTrib": "1.0000",
                            "vUnTrib": "1.00",
                            "indTot": "1",
                        },
                        "imposto": {
                            "ICMS": {"ICMSSN900": {"orig": "0", "CSOSN": "900"}},
                            "PIS": {"PISNT": {"CST": "04"}},
                            "COFINS": {"COFINSNT": {"CST": "04"}},
                        },
                    },
                    "total": {
                        "ICMSTot": {
                            "vBC": "0.00", "vICMS": "0.00", "vICMSDeson": "0.00",
                            "vFCP": "0.00", "vBCST": "0.00", "vST": "0.00",
                            "vFCPST": "0.00", "vFCPSTRet": "0.00",
                            "vProd": "1.00", "vFrete": "0.00", "vSeg": "0.00",
                            "vDesc": "0.00", "vII": "0.00", "vIPI": "0.00",
                            "vIPIDevol": "0.00", "vPIS": "0.00", "vCOFINS": "0.00",
                            "vOutro": "0.00", "vNF": "1.00", "vTotTrib": "0.00",
                        }
                    },
                    "transp": {"modFrete": "9"},
                    "pag": {"detPag": {"tPag": "90", "vPag": "0.00"}},
                    "infAdic": {
                        "infCpl": "EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
                    },
                },
            },
        }
    }
    if nfref:
        payload["enviNFe"]["NFe"]["infNFe"]["ide"]["NFref"] = nfref
    if resp_tec:
        payload["enviNFe"]["NFe"]["infNFe"].setdefault("infRespTec", {}).update(resp_tec)
    if inf_adic:
        payload["enviNFe"]["NFe"]["infNFe"]["infAdic"].update(inf_adic)
    if vols or reboques:
        transp = payload["enviNFe"]["NFe"]["infNFe"].setdefault("transp", {"modFrete": "9"})
        if reboques:
            transp["reboque"] = reboques
        if vols:
            transp["vol"] = vols
    return payload, chave


def build_nfe_cancelamento(chave: str, nprot: str) -> dict:
    return {
        "envEvento": {
            "@versao": "1.00",
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "idLote": "1234567890",
            "evento": {
                "@versao": "1.00",
                "infEvento": {
                    "@Id": f"ID110111{chave}01",
                    "cOrgao": chave[0:2],
                    "tpAmb": _ENV,
                    "CNPJ": _CNPJ,
                    "chNFe": chave,
                    "dhEvento": _dh(),
                    "tpEvento": "110111",
                    "nSeqEvento": "1",
                    "verEvento": "1.00",
                    "detEvento": {
                        "@versao": "1.00",
                        "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                        "descEvento": "Cancelamento",
                        "nProt": nprot,
                        "xJust": "CANCELAMENTO EMITIDO EM AMBIENTE DE HOMOLOGACAO TESTE",
                    },
                },
            },
        }
    }


def build_nfe_inutilizacao(serie: int = 997) -> dict:
    """Build NF-e inutilização request for a single number in a rarely-used series."""
    ano = _now().strftime("%y")
    nni = nnf = str(random.randint(900_000_001, 999_999_999))
    id_str = (
        f"ID{_CUF_PI}{ano}{_CNPJ}55"
        f"{serie:03d}{int(nni):09d}{int(nnf):09d}"
    )
    return {
        "inutNFe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "infInut": {
                "@Id": id_str,
                "tpAmb": _ENV,
                "xServ": "INUTILIZAR",
                "cUF": _CUF_PI,
                "ano": ano,
                "CNPJ": _CNPJ,
                "mod": "55",
                "serie": str(serie),
                "nNFIni": nni,
                "nNFFin": nnf,
                "xJust": "INUTILIZACAO NUMERACAO AMBIENTE DE HOMOLOGACAO TESTE",
            },
        }
    }


def build_nfe_consulta(chave: str) -> dict:
    return {
        "consSitNFe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "tpAmb": _ENV,
            "xServ": "CONSULTAR",
            "chNFe": chave,
        }
    }


def build_nfce(serie: int = 1, nnf: int | None = None, uf: str = "PI") -> tuple[dict, str]:
    """Minimal NFC-e (mod=65). Pass uf= to target a specific authorizer.
    Returns (payload, chave_44)."""
    cuf, cmunfg, xmun, uf_sig = UF_PARAMS.get(uf, UF_PARAMS["PI"])
    if nnf is None:
        nnf = random.randint(800_000_001, 899_999_999)
    chave, cnf, cdv = _chave(cuf, "65", serie, nnf)
    dh = _dh()
    ender = _endereco(cmunfg, xmun, uf_sig)

    payload = {
        "enviNFe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "idLote": str(random.randint(1, 999_999_999)),
            "indSinc": "1",
            "NFe": {
                "@xmlns": "http://www.portalfiscal.inf.br/nfe",
                "infNFe": {
                    "@versao": "4.00",
                    "@Id": f"NFe{chave}",
                    "ide": {
                        "cUF": cuf,
                        "cNF": cnf,
                        "natOp": "VENDA A CONSUMIDOR",
                        "mod": "65",
                        "serie": str(serie),
                        "nNF": str(nnf),
                        "dhEmi": dh,
                        "tpNF": "1",
                        "idDest": "1",
                        "cMunFG": cmunfg,
                        "tpImp": "4",
                        "tpEmis": "1",
                        "cDV": cdv,
                        "tpAmb": _ENV,
                        "finNFe": "1",
                        "indFinal": "1",
                        "indPres": "1",
                        "procEmi": "0",
                        "verProc": "py-dfe-1.0",
                    },
                    "emit": {
                        "CNPJ": _CNPJ,
                        "xNome": "A O CARVALHO TECH",
                        "xFant": "CTECH",
                        "enderEmit": ender,
                        "IE": "123456789",
                        "CRT": "1",
                    },
                    "dest": {
                        "CNPJ": _CNPJ,
                        "xNome": "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
                        "enderDest": ender,
                        "indIEDest": "9",
                    },
                    "det": {
                        "@nItem": "1",
                        "prod": {
                            "cProd": "001",
                            "cEAN": "SEM GTIN",
                            "xProd": "PRODUTO TESTE HOMOLOGACAO NFC-E",
                            "NCM": "84713012",
                            "CFOP": "5102",
                            "uCom": "UN",
                            "qCom": "1.0000",
                            "vUnCom": "1.00",
                            "vProd": "1.00",
                            "cEANTrib": "SEM GTIN",
                            "uTrib": "UN",
                            "qTrib": "1.0000",
                            "vUnTrib": "1.00",
                            "indTot": "1",
                        },
                        "imposto": {
                            "ICMS": {"ICMSSN400": {"orig": "0", "CSOSN": "400"}},
                            "PIS": {"PISNT": {"CST": "07"}},
                            "COFINS": {"COFINSNT": {"CST": "07"}},
                        },
                    },
                    "total": {
                        "ICMSTot": {
                            "vBC": "0.00", "vICMS": "0.00", "vICMSDeson": "0.00",
                            "vFCP": "0.00", "vBCST": "0.00", "vST": "0.00",
                            "vFCPST": "0.00", "vFCPSTRet": "0.00",
                            "vProd": "1.00", "vFrete": "0.00", "vSeg": "0.00",
                            "vDesc": "0.00", "vII": "0.00", "vIPI": "0.00",
                            "vIPIDevol": "0.00", "vPIS": "0.00", "vCOFINS": "0.00",
                            "vOutro": "0.00", "vNF": "1.00", "vTotTrib": "0.00",
                        }
                    },
                    "transp": {"modFrete": "9"},
                    "pag": {"detPag": {"tPag": "01", "vPag": "1.00"}},
                    "infAdic": {
                        "infCpl": "EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
                    },
                },
            },
        }
    }
    return payload, chave


def build_nfce_cancelamento(chave: str, nprot: str) -> dict:
    return {
        "envEvento": {
            "@versao": "1.00",
            "@xmlns": "http://www.portalfiscal.inf.br/nfe",
            "idLote": str(random.randint(1, 999_999_999)),
            "evento": {
                "@versao": "1.00",
                "infEvento": {
                    "@Id": f"ID110111{chave}01",
                    "cOrgao": chave[0:2],
                    "tpAmb": _ENV,
                    "CNPJ": _CNPJ,
                    "chNFe": chave,
                    "dhEvento": _dh(),
                    "tpEvento": "110111",
                    "nSeqEvento": "1",
                    "verEvento": "1.00",
                    "detEvento": {
                        "@versao": "1.00",
                        "descEvento": "Cancelamento",
                        "nProt": nprot,
                        "xJust": "CANCELAMENTO NFC-E AMBIENTE DE HOMOLOGACAO TESTE",
                    },
                },
            },
        }
    }


def _cte_ref_chave() -> str:
    """Generate a valid-format CT-e key for use as a document reference in MDF-e."""
    chave, _, _ = _chave(_CUF_PI, "57", 1, random.randint(1, 899_999_999))
    return chave


def build_cte(serie: int = 1, nct: int | None = None, uf: str = "PI") -> tuple[dict, str]:
    """Minimal CT-e (mod=57). Pass uf= to target a specific authorizer.
    Returns (payload, chave_44)."""
    cuf, cmunfg, xmun, uf_sig = UF_PARAMS.get(uf, UF_PARAMS["PI"])
    if nct is None:
        nct = random.randint(800_000_001, 899_999_999)
    chave, cnf, cdv = _chave(cuf, "57", serie, nct)
    dh = _dh()
    ender = _endereco(cmunfg, xmun, uf_sig)

    payload = {
        "CTe": {
            "@xmlns": "http://www.portalfiscal.inf.br/cte",
            "infCte": {
                "@versao": "4.00",
                "@Id": f"CTe{chave}",
                "ide": {
                    "cUF": cuf,
                    "cCT": cnf,
                    "CFOP": "5352",
                    "natOp": "PRESTACAO DE SERVICO DE TRANSPORTE",
                    "mod": "57",
                    "serie": str(serie),
                    "nCT": str(nct),
                    "dhEmi": dh,
                    "tpImp": "1",
                    "tpEmis": "1",
                    "cDV": cdv,
                    "tpAmb": _ENV,
                    "tpCTe": "0",
                    "procEmi": "0",
                    "verProc": "py-dfe-1.0",
                    "cMunEnv": cmunfg,
                    "xMunEnv": xmun,
                    "UFEnv": uf_sig,
                    "modal": "01",
                    "tpServ": "0",
                    "cMunIni": cmunfg,
                    "xMunIni": xmun,
                    "UFIni": uf_sig,
                    "cMunFim": cmunfg,
                    "xMunFim": xmun,
                    "UFFim": uf_sig,
                    "retira": "0",
                    "indIEToma": "9",
                    "toma3": {"toma": "3"},
                },
                "emit": {
                    "CNPJ": _CNPJ,
                    "IE": "123456789",
                    "xNome": "A O CARVALHO TECH",
                    "xFant": "CTECH",
                    "enderEmit": {
                        "xLgr": "RUA TESTE HOMOLOGACAO",
                        "nro": "1",
                        "xBairro": "CENTRO",
                        "cMun": cmunfg,
                        "xMun": xmun,
                        "CEP": "64057060",
                        "UF": uf_sig,
                        "fone": "5586988888888"
                    },
                    "CRT": "1",
                },
                "rem": {
                    "CNPJ": _CNPJ,
                    "IE": "123456789",
                    "xNome": "REMETENTE HOMOLOGACAO",
                    "fone": "86999999999",
                    "enderReme": {
                        "xLgr": "RUA TESTE HOMOLOGACAO",
                        "nro": "1",
                        "xBairro": "CENTRO",
                        "cMun": cmunfg,
                        "xMun": xmun,
                        "CEP": "64057060",
                        "UF": uf_sig,
                        "cPais": "1058",
                        "xPais": "Brasil",
                    },
                },
                "dest": {
                    "CNPJ": _CNPJ,
                    "IE": "123456789",
                    "xNome": "CT-E EMITIDO EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
                    "fone": "86999999999",
                    "enderDest": {
                        "xLgr": "RUA TESTE HOMOLOGACAO",
                        "nro": "1",
                        "xBairro": "CENTRO",
                        "cMun": cmunfg,
                        "xMun": xmun,
                        "CEP": "64057060",
                        "UF": uf_sig,
                        "cPais": "1058",
                        "xPais": "Brasil",
                    }
                },
                "vPrest": {
                    "vTPrest": "10.00",
                    "vRec": "10.00",
                    "Comp": {"xNome": "FRETE", "vComp": "10.00"},
                },
                "imp": {
                    "ICMS": {"ICMSSN": {"CST": "90", "indSN": "1"}},
                    "vTotTrib": "0.00",
                },
                "infCTeNorm": {
                    "infCarga": {
                        "vCarga": "10.00",
                        "proPred": "TESTES",
                        "xOutCat": "TESTE",
                        "infQ": {
                            "cUnid": "02",
                            "tpMed": "PESO BRUTO",
                            "qCarga": "10.0000",
                        },
                    },
                    "infDoc": {
                        "infOutros": {
                            "tpDoc": "99",
                            "descOutros": "SERVICO TRANSPORTE TESTE HOMOLOGACAO",
                            "nDoc": "001",
                            "dEmi": _date(),
                            "vDocFisc": "10.00",
                        },
                    },
                    "infModal": {
                        "@versaoModal": "4.00",
                        "rodo": {"RNTRC": "12345678"},
                    },
                },
            },
        }
    }
    return payload, chave


def build_cte_consulta(chave: str) -> dict:
    return {
        "consSitCTe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/cte",
            "tpAmb": _ENV,
            "xServ": "CONSULTAR",
            "chCTe": chave,
        }
    }


def build_cte_cancelamento(chave: str, nprot: str) -> dict:
    return {
        "eventoCTe": {
            "@versao": "4.00",
            "@xmlns": "http://www.portalfiscal.inf.br/cte",
            "infEvento": {
                "@Id": f"ID110111{chave}001",
                "cOrgao": chave[0:2],
                "tpAmb": _ENV,
                "CNPJ": _CNPJ,
                "chCTe": chave,
                "dhEvento": _dh(),
                "tpEvento": "110111",
                "nSeqEvento": "1",
                "detEvento": {
                    "@versaoEvento": "4.00",
                    'evCancCTe': {
                        "descEvento": "Cancelamento",
                        "nProt": nprot,
                        "xJust": "CANCELAMENTO CT-E AMBIENTE DE HOMOLOGACAO TESTE",
                    }
                },
            },
        }
    }


def build_mdfe(serie: int = 1, nmdf: int | None = None, inf_antt: dict | None = None) -> tuple[dict, str]:
    """Minimal MDF-e (mod=58) for SVRS/SP HOMOLOGAÇÃO. Returns (payload, chave_44).

    inf_antt estende infANTT (valePed, infContratante, infPag).
    """
    if nmdf is None:
        nmdf = random.randint(800_000_001, 899_999_999)
    chave, cnf, cdv = _chave(_CUF_PI, "58", serie, nmdf)
    dh = _dh()
    cte_ref = _cte_ref_chave()

    payload = {
        "enviMDFe": {
            "@versao": "3.00",
            "@xmlns": "http://www.portalfiscal.inf.br/mdfe",
            "idLote": str(random.randint(1, 999_999_999)),
            "MDFe": {
                "infMDFe": {
                    "@versao": "3.00",
                    "@Id": f"MDFe{chave}",
                    "ide": {
                        "cUF": _CUF_PI,
                        "tpAmb": _ENV,
                        "tpEmit": "1",
                        "tpTransp": "1",
                        "mod": "58",
                        "serie": str(serie),
                        "nMDF": str(nmdf),
                        "cMDF": cnf,
                        "cDV": cdv,
                        "modal": "01",
                        "dhEmi": dh,
                        "tpEmis": "1",
                        "procEmi": "0",
                        "verProc": "py-dfe-1.0",
                        "UFIni": "PI",
                        "UFFim": "PI",
                        "infMunCarrega": {
                            "cMunCarrega": _CMUN_TERESINA,
                            "xMunCarrega": "Teresina",
                        },
                    },
                    "emit": {
                        "CNPJ": _CNPJ,
                        "IE": "123456789",
                        "xNome": "A O CARVALHO TECH",
                        "xFant": "CTECH",
                        "enderEmit": _uf_endereco(),
                    },
                    "infModal": {
                        "@versao": "3.00",
                        "rodo": {
                            "infANTT": {"RNTRC": "12345678"},
                            "veicTracao": {
                                "cInt": "1",
                                "placa": "TST1234",
                                "tara": "10000",
                                "capKG": "30000",
                                "condutor": {
                                    "xNome": "MOTORISTA TESTE HOMOLOGACAO",
                                    "CPF": _CPF_MOTORISTA,
                                },
                                "tpRod": "01",
                                "tpCar": "00",
                                "UF": "PI",
                            },
                        },
                    },
                    "infDoc": {
                        "infMunDescarga": {
                            "cMunDescarga": _CMUN_TERESINA,
                            "xMunDescarga": "Teresina",
                            "infCTe": {"chCTe": cte_ref},
                        },
                    },
                    "tot": {
                        "qCTe": "1",
                        "vCarga": "10.00",
                        "cUnid": "02",
                        "qCarga": "10.0000",
                    },
                    "infAdic": {
                        "infCpl": "MDF-E EMITIDO EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
                    },
                },
            },
        }
    }
    if inf_antt:
        rodo = payload["MDFe"]["infMDFe"]["infModal"]["rodo"]
        rodo.setdefault("infANTT", {}).update(inf_antt)
    return payload, chave


def build_mdfe_consulta(chave: str) -> dict:
    return {
        "consSitMDFe": {
            "@versao": "3.00",
            "@xmlns": "http://www.portalfiscal.inf.br/mdfe",
            "tpAmb": _ENV,
            "xServ": "CONSULTAR",
            "chMDFe": chave,
        }
    }


def build_mdfe_cancelamento(chave: str, nprot: str) -> dict:
    return {
        "envEventoMDFe": {
            "@versao": "3.00",
            "@xmlns": "http://www.portalfiscal.inf.br/mdfe",
            "idLote": str(random.randint(1, 999_999_999)),
            "evento": {
                "@versao": "3.00",
                "infEvento": {
                    "@Id": f"ID110111{chave}01",
                    "cOrgao": chave[0:2],
                    "tpAmb": _ENV,
                    "CNPJ": _CNPJ,
                    "chMDFe": chave,
                    "dhEvento": _dh(),
                    "tpEvento": "110111",
                    "nSeqEvento": "1",
                    "detEvento": {
                        "@versao": "3.00",
                        "descEvento": "Cancelamento",
                        "nProt": nprot,
                        "xJust": "CANCELAMENTO MDF-E AMBIENTE DE HOMOLOGACAO TESTE",
                    },
                },
            },
        }
    }
