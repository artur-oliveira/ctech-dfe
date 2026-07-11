from enum import Enum

from py_dfe.exceptions import DFeError, EndpointNotFoundError


class UF(str, Enum):
    AC = "AC"
    AL = "AL"
    AM = "AM"
    AP = "AP"
    BA = "BA"
    CE = "CE"
    DF = "DF"
    ES = "ES"
    GO = "GO"
    MA = "MA"
    MG = "MG"
    MS = "MS"
    MT = "MT"
    PA = "PA"
    PB = "PB"
    PE = "PE"
    PI = "PI"
    PR = "PR"
    RJ = "RJ"
    RN = "RN"
    RO = "RO"
    RR = "RR"
    RS = "RS"
    SC = "SC"
    SE = "SE"
    SP = "SP"
    TO = "TO"
    EX = "EX"


class Environment(str, Enum):
    PRODUCTION = "prod"
    HOMOLOGATION = "hom"


class DocType(str, Enum):
    NFE = "nfe"
    NFCE = "nfce"
    CTE = "cte"
    MDFE = "mdfe"


class NFeService(str, Enum):
    AUTORIZACAO = "NFeAutorizacao"
    RET_AUTORIZACAO = "NFeRetAutorizacao"
    INUTILIZACAO = "NfeInutilizacao"
    CONSULTA_PROTOCOLO = "NfeConsultaProtocolo"
    STATUS_SERVICO = "NfeStatusServico"
    CONSULTA_CADASTRO = "NfeConsultaCadastro"
    RECEPCAO_EVENTO = "RecepcaoEvento"
    DISTRIBUICAO_DFE = "NFeDistribuicaoDFe"


class NfCeService(str, Enum):
    AUTORIZACAO = "NFeAutorizacao"
    RET_AUTORIZACAO = "NFeRetAutorizacao"
    INUTILIZACAO = "NfeInutilizacao"
    CONSULTA_PROTOCOLO = "NfeConsultaProtocolo"
    STATUS_SERVICO = "NfeStatusServico"
    RECEPCAO_EVENTO = "RecepcaoEvento"


class CTeService(str, Enum):
    RECEPCAO_SINC = "CTeRecepcaoSinc"
    RECEPCAO_OS = "CTeRecepcaoOS"
    RECEPCAO_GTVE = "CTeRecepcaoGTVe"
    RECEPCAO_SIMP = "CTeRecepcaoSimp"
    CONSULTA = "CTeConsulta"
    STATUS_SERVICO = "CTeStatusServico"
    RECEPCAO_EVENTO = "CTeRecepcaoEvento"
    DISTRIBUICAO_DFE = "CTeDistribuicaoDFe"


class MDFeService(str, Enum):
    RECEPCAO_SINC = "MDFeRecepcaoSinc"
    CONSULTA = "MDFeConsulta"
    STATUS_SERVICO = "MDFeStatusServico"
    CONS_NAO_ENC = "MDFeConsNaoEnc"
    DISTRIBUICAO_DFE = "MDFeDistribuicaoDFe"
    RECEPCAO_EVENTO = "MDFeRecepcaoEvento"


UF_IBGE: dict[str, int] = {
    "RO": 11,
    "AC": 12,
    "AM": 13,
    "RR": 14,
    "PA": 15,
    "AP": 16,
    "TO": 17,
    "MA": 21,
    "PI": 22,
    "CE": 23,
    "RN": 24,
    "PB": 25,
    "PE": 26,
    "AL": 27,
    "SE": 28,
    "BA": 29,
    "MG": 31,
    "ES": 32,
    "RJ": 33,
    "SP": 35,
    "PR": 41,
    "SC": 42,
    "RS": 43,
    "MS": 50,
    "MT": 51,
    "GO": 52,
    "DF": 53,
    "EX": 91,
}


def get_uf_ibge(uf):
    if uf in UF_IBGE:
        return UF_IBGE[uf]
    raise EndpointNotFoundError(f"No IBGE code for UF {uf!r}")


DOC_TYPE_CODE: dict[str, str] = {
    "nfe": "55",
    "nfce": "65",
    "cte": "57",
    "mdfe": "58",
}


def get_doc_type_code(doc_type):
    if doc_type in DOC_TYPE_CODE:
        return DOC_TYPE_CODE[doc_type]
    raise DFeError(400, "invalid_doc_type", f"No doc type code for {doc_type!r}")


SCHEMA_VERSION: dict[str, str] = {
    "nfe": "4.00",
    "nfce": "4.00",
    "cte": "4.00",
    "mdfe": "3.00",
}

# Some services use a payload schema version different from the doc-type version.
# MT (and potentially other strict UFs) validate versaoDados against the actual payload schema.
SOAP_HEADER_VERSION_OVERRIDE: dict[str, str] = {
    "NfeConsultaCadastro": "2.00",
}

DOC_NAMESPACE: dict[str, str] = {
    "nfe": "http://www.portalfiscal.inf.br/nfe",
    "nfce": "http://www.portalfiscal.inf.br/nfe",
    "cte": "http://www.portalfiscal.inf.br/cte",
    "mdfe": "http://www.portalfiscal.inf.br/mdfe",
}

SOAP_ELEMENTS: dict[str, dict[str, str]] = {
    "nfe": {"header": "nfeCabecMsg", "body": "nfeDadosMsg", "result": "nfeResultMsg"},
    "nfce": {"header": "nfeCabecMsg", "body": "nfeDadosMsg", "result": "nfeResultMsg"},
    "cte": {"header": "cteCabecMsg", "body": "cteDadosMsg", "result": "cteResultMsg"},
    "mdfe": {
        "header": "mdfeCabecMsg",
        "body": "mdfeDadosMsg",
        "result": "mdfeResultMsg",
    },
}

NFE_WSDL_SERVICE: dict[str, str] = {
    "NFeAutorizacao": "NFeAutorizacao4",
    "NFeRetAutorizacao": "NFeRetAutorizacao4",
    "NfeInutilizacao": "NFeInutilizacao4",
    "NfeConsultaProtocolo": "NFeConsultaProtocolo4",
    "NfeStatusServico": "NFeStatusServico4",
    "NfeConsultaCadastro": "CadConsultaCadastro4",
    "RecepcaoEvento": "NFeRecepcaoEvento4",
    "NFeDistribuicaoDFe": "NFeDistribuicaoDFe",
}

CTE_WSDL_SERVICE: dict[str, str] = {
    "CTeRecepcaoSinc": "CTeRecepcaoSincV4",
    "CTeRecepcaoOS": "CTeRecepcaoOSV4",
    "CTeRecepcaoGTVe": "CTeRecepcaoGTVeV4",
    "CTeRecepcaoSimp": "CTeRecepcaoSimpV4",
    "CTeConsulta": "CTeConsultaV4",
    "CTeStatusServico": "CTeStatusServicoV4",
    "CTeRecepcaoEvento": "CTeRecepcaoEventoV4",
    "CTeDistribuicaoDFe": "CTeDistribuicaoDFe",
}

MDFE_WSDL_SERVICE: dict[str, str] = {
    "MDFeRecepcaoSinc": "MDFeRecepcaoSinc",
    "MDFeConsulta": "MDFeConsulta",
    "MDFeStatusServico": "MDFeStatusServico",
    "MDFeConsNaoEnc": "MDFeConsNaoEnc",
    "MDFeDistribuicaoDFe": "MDFeDistribuicaoDFe",
    "MDFeRecepcaoEvento": "MDFeRecepcaoEvento",
}

WSDL_SERVICE_BY_DOCTYPE: dict[str, dict[str, str]] = {
    "nfe": NFE_WSDL_SERVICE,
    "nfce": NFE_WSDL_SERVICE,
    "cte": CTE_WSDL_SERVICE,
    "mdfe": MDFE_WSDL_SERVICE,
}

NFE_WSDL_OPERATION: dict[str, str] = {
    "NFeAutorizacao": "nfeAutorizacaoLote",
    "NFeRetAutorizacao": "nfeRetAutorizacaoLote",
    "NfeInutilizacao": "nfeInutilizacaoNF",
    "NfeConsultaProtocolo": "nfeConsultaNF",
    "NfeStatusServico": "nfeStatusServicoNF",
    "NfeConsultaCadastro": "consultaCadastro",
    "RecepcaoEvento": "nfeRecepcaoEvento",
    "NFeDistribuicaoDFe": "nfeDistDFeInteresse",
}

CTE_WSDL_OPERATION: dict[str, str] = {
    "CTeStatusServico": "cteStatusServicoCT",
    "CTeRecepcaoSinc": "cteRecepcao",
    "CTeRecepcaoOS": "cteRecepcaoOS",
    "CTeRecepcaoGTVe": "cteRecepcaoGTVe",
    "CTeRecepcaoSimp": "cteRecepcaoSimp",
    "CTeConsulta": "cteConsultaCT",
    "CTeRecepcaoEvento": "cteRecepcaoEvento",
    "CTeDistribuicaoDFe": "cteDistDFeInteresse",
}

MDFE_WSDL_OPERATION: dict[str, str] = {
    "MDFeStatusServico": "mdfeStatusServicoMDF",
    "MDFeRecepcaoSinc": "mdfeRecepcao",
    "MDFeConsulta": "mdfeConsultaMDF",
    "MDFeConsNaoEnc": "mdfeConsNaoEnc",
    "MDFeRecepcaoEvento": "mdfeRecepcaoEvento",
    "MDFeDistribuicaoDFe": "mdfeDistDFeInteresse",
}

WSDL_OPERATION_BY_DOCTYPE: dict[str, dict[str, str]] = {
    "nfe": NFE_WSDL_OPERATION,
    "nfce": NFE_WSDL_OPERATION,
    "cte": CTE_WSDL_OPERATION,
    "mdfe": MDFE_WSDL_OPERATION,
}

# Services that always require a wrapped SOAP body (operation element wrapping the body element).
SOAP_WRAPPED_BODY_SERVICES: frozenset[str] = frozenset(
    {
        "NFeDistribuicaoDFe",
        "CTeDistribuicaoDFe",
    }
)

# Per-authorizer overrides for wrapped SOAP body (authorizer, service).
# MT's CadConsultaCadastro4 requires <consultaCadastro><nfeDadosMsg>...</nfeDadosMsg></consultaCadastro>.
SOAP_WRAPPED_BODY_OVERRIDES: frozenset[tuple[str, str]] = frozenset(
    {
        ("MT", "NfeConsultaCadastro"),
    }
)

# Códigos de tpEvento (tabela SEFAZ)
TP_EVENTO_CANCELAMENTO = "110111"
TP_EVENTO_CIENCIA_OPERACAO = "210210"
TP_EVENTO_CONFIRMACAO_OPERACAO = "210200"
TP_EVENTO_DESCONHECIMENTO_OPERACAO = "210220"
TP_EVENTO_NAO_REALIZACAO = "210240"

WSDL_OPERATION_OVERRIDES: dict[tuple[str, str, str], str] = {
    ("BA", "nfe", "RecepcaoEvento"): "nfeRecepcaoEventoNF",
    ("PR", "nfe", "RecepcaoEvento"): "nfeRecepcaoEventoNF",
    ("AN", "nfe", "RecepcaoEvento"): "nfeRecepcaoEventoNF",
    ("PR", "nfce", "RecepcaoEvento"): "nfeRecepcaoEventoNF",
    ("SP", "nfce", "RecepcaoEvento"): "nfeRecepcaoEventoNF",
    ("AM", "nfe", "NfeConsultaCadastro"): "consultaCadastro4",
    ("PE", "nfe", "NFeRetAutorizacao"): "NFeRetAutorizacaoLote",
    ("PR", "nfe", "NFeRetAutorizacao"): "NFeRetAutorizacaoLote",
    ("PR", "nfce", "NFeRetAutorizacao"): "NFeRetAutorizacaoLote",
}
