"""SEFAZ endpoint URLs organized by document type, authorizer and environment."""

from __future__ import annotations

_Env = dict[str, str]
_Auth = dict[str, _Env]
_Registry = dict[str, _Auth]


def _ep(prod: str, hom: str, paths: dict[str, str]) -> _Auth:
    """Build endpoint dict from base URLs + relative paths (same for prod/hom)."""
    return {
        "prod": {k: prod.rstrip("/") + v for k, v in paths.items()},
        "hom": {k: hom.rstrip("/") + v for k, v in paths.items()},
    }


def _ep2(prod_paths: dict[str, str], hom_paths: dict[str, str]) -> _Auth:
    """Build endpoint dict with different paths per environment."""
    return {"prod": prod_paths, "hom": hom_paths}


_NF_FRAG_PATH = {
    "NfeInutilizacao": "/NFeInutilizacao4",
    "NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
    "NfeStatusServico": "/NFeStatusServico4",
    "NfeConsultaCadastro": "/CadConsultaCadastro4",
    "RecepcaoEvento": "/NFeRecepcaoEvento4",
    "NFeAutorizacao": "/NFeAutorizacao4",
    "NFeRetAutorizacao": "/NFeRetAutorizacao4",
}


def _asmx_frag(service_path: str) -> str:
    name = service_path.lstrip("/")
    return f"/{name}/{name}.asmx"


_CAD_SVRS = "https://cad.svrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx"

_NFE: _Registry = {

    "AM": _ep(
        "https://nfe.sefaz.am.gov.br/services2/services",
        "https://homnfe.sefaz.am.gov.br/services2/services",
        {
            "NfeInutilizacao": "/NfeInutilizacao4",
            "NfeConsultaProtocolo": "/NfeConsulta4",
            "NfeStatusServico": "/NfeStatusServico4",
            "NfeConsultaCadastro": "/CadConsultaCadastro4",
            "RecepcaoEvento": "/RecepcaoEvento4",
            "NFeAutorizacao": "/NfeAutorizacao4",
            "NFeRetAutorizacao": "/NfeRetAutorizacao4",
        },
    ),
    "BA": _ep(
        "https://nfe.sefaz.ba.gov.br/webservices",
        "https://hnfe.sefaz.ba.gov.br/webservices",
        {k: _asmx_frag(v) for k, v in _NF_FRAG_PATH.items()},
    ),
    "GO": _ep(
        "https://nfe.sefaz.go.gov.br/nfe/services",
        "https://homolog.sefaz.go.gov.br/nfe/services",
        _NF_FRAG_PATH,
    ),
    "MG": _ep(
        "https://nfe.fazenda.mg.gov.br/nfe2/services",
        "https://hnfe.fazenda.mg.gov.br/nfe2/services",
        _NF_FRAG_PATH,
    ),
    "MS": _ep(
        "https://nfe.sefaz.ms.gov.br/ws",
        "https://hom.nfe.sefaz.ms.gov.br/ws",
        _NF_FRAG_PATH,
    ),
    "MT": _ep(
        "https://nfe.sefaz.mt.gov.br/nfews/v2/services",
        "https://homologacao.sefaz.mt.gov.br/nfews/v2/services",
        {
            "NfeInutilizacao": "/NfeInutilizacao4",
            "NfeConsultaProtocolo": "/NfeConsulta4",
            "NfeStatusServico": "/NfeStatusServico4",
            "NfeConsultaCadastro": "/CadConsultaCadastro4",
            "RecepcaoEvento": "/RecepcaoEvento4",
            "NFeAutorizacao": "/NfeAutorizacao4",
            "NFeRetAutorizacao": "/NfeRetAutorizacao4",
        },
    ),
    "PE": _ep(
        "https://nfe.sefaz.pe.gov.br/nfe-service/services",
        "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services",
        _NF_FRAG_PATH,
    ),
    "PR": _ep(
        "https://nfe.sefa.pr.gov.br/nfe",
        "https://homologacao.nfe.sefa.pr.gov.br/nfe",
        _NF_FRAG_PATH,
    ),
    "RS": _ep2(
        {
            "NfeInutilizacao": "https://nfe.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfe.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfe.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "NfeConsultaCadastro": _CAD_SVRS,
            "RecepcaoEvento": "https://nfe.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
            "NFeAutorizacao": "https://nfe.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfe.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
        },
        {
            "NfeInutilizacao": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "NfeConsultaCadastro": _CAD_SVRS,
            "RecepcaoEvento": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
            "NFeAutorizacao": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
        },
    ),
    "SP": _ep(
        "https://nfe.fazenda.sp.gov.br/ws",
        "https://homologacao.nfe.fazenda.sp.gov.br/ws",
        {
            "NfeInutilizacao": "/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "/nfeconsultaprotocolo4.asmx",
            "NfeStatusServico": "/nfestatusservico4.asmx",
            "NfeConsultaCadastro": "/cadconsultacadastro4.asmx",
            "RecepcaoEvento": "/nferecepcaoevento4.asmx",
            "NFeAutorizacao": "/nfeautorizacao4.asmx",
            "NFeRetAutorizacao": "/nferetautorizacao4.asmx",
        },
    ),

    "SVRS": _ep2(
        {
            "NfeInutilizacao": "https://nfe.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfe.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfe.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "NfeConsultaCadastro": _CAD_SVRS,
            "RecepcaoEvento": "https://nfe.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
            "NFeAutorizacao": "https://nfe.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfe.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
        },
        {
            "NfeInutilizacao": "https://nfe-homologacao.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "NfeConsultaCadastro": _CAD_SVRS,
            "RecepcaoEvento": "https://nfe-homologacao.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
            "NFeAutorizacao": "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
        },
    ),

    "AN": _ep2(
        {
            "NFeDistribuicaoDFe": "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx",
            "RecepcaoEvento": "https://www.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx",
        },
        {
            "NFeDistribuicaoDFe": "https://hom1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx",
            "RecepcaoEvento": "https://hom1.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx",
        },
    ),
}

_NFE_UF_AUTH: dict[str, str] = {
    "AM": "AM", "BA": "BA", "GO": "GO", "MG": "MG", "MS": "MS",
    "MT": "MT", "PE": "PE", "PR": "PR", "RS": "RS", "SP": "SP",
    **{uf: "SVRS" for uf in [
        "AC", "AL", "AP", "CE", "DF", "ES", "MA", "PA", "PB",
        "PI", "RJ", "RN", "RO", "RR", "SC", "SE", "TO", "EX",
    ]},
}

_NFCE: _Registry = {
    "AM": _ep(
        "https://nfce.sefaz.am.gov.br/nfce-services/services",
        "https://homnfce.sefaz.am.gov.br/nfce-services/services",
        {
            "NFeAutorizacao": "/NfeAutorizacao4",
            "NFeRetAutorizacao": "/NfeRetAutorizacao4",
            "NfeInutilizacao": "/NfeInutilizacao4",
            "NfeConsultaProtocolo": "/NfeConsulta4",
            "NfeStatusServico": "/NfeStatusServico4",
            "RecepcaoEvento": "/RecepcaoEvento4",
        },
    ),

    "GO": _ep(
        "https://nfe.sefaz.go.gov.br/nfe/services",
        "https://homolog.sefaz.go.gov.br/nfe/services",
        {
            "NFeAutorizacao": "/NFeAutorizacao4",
            "NFeRetAutorizacao": "/NFeRetAutorizacao4",
            "NfeInutilizacao": "/NFeInutilizacao4",
            "NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
            "NfeStatusServico": "/NFeStatusServico4",
            "NfeConsultaCadastro": "/CadConsultaCadastro4",
            "RecepcaoEvento": "/NFeRecepcaoEvento4",
        },
    ),
    "MS": _ep(
        "https://nfce.sefaz.ms.gov.br/ws",
        "https://hom.nfce.sefaz.ms.gov.br/ws",
        {
            "NFeAutorizacao": "/NFeAutorizacao4",
            "NFeRetAutorizacao": "/NFeRetAutorizacao4",
            "NfeInutilizacao": "/NFeInutilizacao4",
            "NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
            "NfeStatusServico": "/NFeStatusServico4",
            "NfeConsultaCadastro": "/CadConsultaCadastro4",
            "RecepcaoEvento": "/NFeRecepcaoEvento4",
        },
    ),
    "MT": _ep(
        "https://nfce.sefaz.mt.gov.br/nfcews/services",
        "https://homologacao.sefaz.mt.gov.br/nfcews/services",
        {
            "NFeAutorizacao": "/NfeAutorizacao4",
            "NFeRetAutorizacao": "/NfeRetAutorizacao4",
            "NfeInutilizacao": "/NfeInutilizacao4",
            "NfeConsultaProtocolo": "/NfeConsulta4",
            "NfeStatusServico": "/NfeStatusServico4",
            "RecepcaoEvento": "/RecepcaoEvento4",
        },
    ),
    "PR": _ep(
        "https://nfce.sefa.pr.gov.br/nfce",
        "https://homologacao.nfce.sefa.pr.gov.br/nfce",
        {
            "NFeAutorizacao": "/NFeAutorizacao4",
            "NFeRetAutorizacao": "/NFeRetAutorizacao4",
            "NfeInutilizacao": "/NFeInutilizacao4",
            "NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
            "NfeStatusServico": "/NFeStatusServico4",
            "NfeConsultaCadastro": "/CadConsultaCadastro4",
            "RecepcaoEvento": "/NFeRecepcaoEvento4",
        },
    ),
    "RS": _ep2(
        {
            "NFeAutorizacao": "https://nfce.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfce.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
            "NfeInutilizacao": "https://nfce.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfce.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfce.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "RecepcaoEvento": "https://nfce.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
        },
        {
            "NFeAutorizacao": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
            "NfeInutilizacao": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "RecepcaoEvento": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
        },
    ),
    "SP": _ep(
        "https://nfce.fazenda.sp.gov.br/ws",
        "https://homologacao.nfce.fazenda.sp.gov.br/ws",
        {
            "NFeAutorizacao": "/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "/NFeRetAutorizacao4.asmx",
            "NfeInutilizacao": "/NFeInutilizacao4.asmx",
            "NfeConsultaProtocolo": "/NFeConsultaProtocolo4.asmx",
            "NfeStatusServico": "/NFeStatusServico4.asmx",
            "RecepcaoEvento": "/NFeRecepcaoEvento4.asmx",
        },
    ),
    "SVRS": _ep2(
        {
            "NFeAutorizacao": "https://nfce.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfce.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
            "NfeInutilizacao": "https://nfce.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfce.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfce.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "RecepcaoEvento": "https://nfce.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
        },
        {
            "NFeAutorizacao": "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
            "NFeRetAutorizacao": "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
            "NfeInutilizacao": "https://nfce-homologacao.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
            "NfeConsultaProtocolo": "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
            "NfeStatusServico": "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
            "RecepcaoEvento": "https://nfce-homologacao.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
        },
    ),
}

_NFCE_UF_AUTH: dict[str, str] = {
    "AM": "AM", "GO": "GO", "MS": "MS", "MT": "MT",
    "PR": "PR", "RS": "RS", "SP": "SP",
    **{uf: "SVRS" for uf in [
        "AC", "AL", "AP", "BA", "CE", "DF", "ES", "MA", "MG",
        "PA", "PB", "PE", "PI", "RJ", "RN", "RO", "RR",
        "SC", "SE", "TO", "EX",
    ]},
}

_CTE_FRAG = {
    "CTeRecepcaoSinc": "/CTeRecepcaoSincV4",
    "CTeRecepcaoOS": "/CTeRecepcaoOSV4",
    "CTeRecepcaoGTVe": "/CTeRecepcaoGTVeV4",
    "CTeRecepcaoSimp": "/CTeRecepcaoSimpV4",
    "CTeConsulta": "/CTeConsultaV4",
    "CTeStatusServico": "/CTeStatusServicoV4",
    "CTeRecepcaoEvento": "/CTeRecepcaoEventoV4",
}

_CTE: _Registry = {
    "MG": _ep(
        "https://cte.fazenda.mg.gov.br/cte/services",
        "https://hcte.fazenda.mg.gov.br/cte/services",
        _CTE_FRAG,
    ),
    "MS": _ep(
        "https://producao.cte.ms.gov.br/ws",
        "https://homologacao.cte.ms.gov.br/ws",
        _CTE_FRAG,
    ),
    "MT": _ep2(

        {
            "CTeConsulta": "https://cte.sefaz.mt.gov.br/ctews2/services/CTeConsultaV4",
            "CTeRecepcaoEvento": "https://cte.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoEventoV4",
            "CTeStatusServico": "https://cte.sefaz.mt.gov.br/ctews2/services/CTeStatusServicoV4",
            "CTeRecepcaoSinc": "https://cte.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoSincV4",
            "CTeRecepcaoGTVe": "https://cte.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoGTVeV4",
            "CTeRecepcaoOS": "https://cte.sefaz.mt.gov.br/ctews/services/CTeRecepcaoOSV4",
            "CTeRecepcaoSimp": "https://cte.sefaz.mt.gov.br/cte-ws/services/CTeRecepcaoSimpV4",
        },
        {
            "CTeConsulta": "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeConsultaV4",
            "CTeRecepcaoEvento": "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoEventoV4",
            "CTeStatusServico": "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeStatusServicoV4",
            "CTeRecepcaoSinc": "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoSincV4",
            "CTeRecepcaoGTVe": "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoGTVeV4",
            "CTeRecepcaoOS": "https://homologacao.sefaz.mt.gov.br/ctews/services/CTeRecepcaoOSV4",
            "CTeRecepcaoSimp": "https://homologacao.sefaz.mt.gov.br/cte-ws/services/CTeRecepcaoSimpV4",
        },
    ),
    "PR": _ep(
        "https://cte.fazenda.pr.gov.br/cte4",
        "https://homologacao.cte.fazenda.pr.gov.br/cte4",
        _CTE_FRAG,
    ),

    "SP": _ep(
        "https://nfe.fazenda.sp.gov.br/CTeWS/WS",
        "https://homologacao.nfe.fazenda.sp.gov.br/CTeWS/WS",
        {k: f"{v.replace('V4', 'V4')}.asmx" for k, v in _CTE_FRAG.items()},
    ),

    "SVRS": _ep2(
        {k: f"https://cte.svrs.rs.gov.br/ws{v}{v}.asmx" for k, v in _CTE_FRAG.items()},
        {k: f"https://cte-homologacao.svrs.rs.gov.br/ws{v}{v}.asmx" for k, v in _CTE_FRAG.items()},
    ),
    "AN": _ep2(
        {"CTeDistribuicaoDFe": "https://www1.cte.fazenda.gov.br/CTeDistribuicaoDFe/CTeDistribuicaoDFe.asmx"},
        {"CTeDistribuicaoDFe": "https://hom1.cte.fazenda.gov.br/CTeDistribuicaoDFe/CTeDistribuicaoDFe.asmx"},
    ),
}

_CTE_UF_AUTH: dict[str, str] = {
    "MG": "MG", "MS": "MS", "MT": "MT", "PR": "PR", "SP": "SP",
    **{uf: "SVRS" for uf in [
        "AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA",
        "PA", "PB", "PE", "PI", "RJ", "RN", "RO", "RR", "RS",
        "SC", "SE", "TO", "EX",
    ]},
}

_MDFE_SVRS_FRAG = {
    "MDFeRecepcaoEvento": "/MDFeRecepcaoEvento/MDFeRecepcaoEvento.asmx",
    "MDFeConsulta": "/MDFeConsulta/MDFeConsulta.asmx",
    "MDFeStatusServico": "/MDFeStatusServico/MDFeStatusServico.asmx",
    "MDFeConsNaoEnc": "/MDFeConsNaoEnc/MDFeConsNaoEnc.asmx",
    "MDFeDistribuicaoDFe": "/MDFeDistribuicaoDFe/MDFeDistribuicaoDFe.asmx",
    "MDFeRecepcaoSinc": "/MDFeRecepcaoSinc/MDFeRecepcaoSinc.asmx",
}

_MDFE: _Registry = {
    "SVRS": _ep(
        "https://mdfe.svrs.rs.gov.br/ws",
        "https://mdfe-homologacao.svrs.rs.gov.br/ws",
        _MDFE_SVRS_FRAG,
    ),
}

_MDFE_UF_AUTH: dict[str, str] = {uf: "SVRS" for uf in [
    "AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA",
    "MG", "MS", "MT", "PA", "PB", "PE", "PI", "PR", "RJ", "RN",
    "RO", "RR", "RS", "SC", "SE", "SP", "TO", "EX",
]}

_REGISTRY: dict[str, tuple[_Registry, dict[str, str]]] = {
    "nfe": (_NFE, _NFE_UF_AUTH),
    "nfce": (_NFCE, _NFCE_UF_AUTH),
    "cte": (_CTE, _CTE_UF_AUTH),
    "mdfe": (_MDFE, _MDFE_UF_AUTH),
}


def get_endpoint(doc_type: str, uf: str, environment: str, service: str) -> str:
    """Return the SEFAZ endpoint URL for the given parameters.

    For services that route to the Environment Nacional (NFeDistribuicaoDFe,
    CTeDistribuicaoDFe) pass uf='AN'.
    """
    endpoints, uf_auth = _REGISTRY[doc_type]
    authorizer = "AN" if uf == "AN" else uf_auth[uf]
    try:
        return endpoints[authorizer][environment][service]
    except KeyError:
        raise KeyError(
            f"Endpoint not found: doc_type={doc_type!r}, uf={uf!r}, "
            f"environment={environment!r}, service={service!r} "
            f"(authorizer={authorizer!r})"
        )


def list_services(doc_type: str, uf: str, environment: str) -> list[str]:
    """List all available services for the given combination."""
    endpoints, uf_auth = _REGISTRY[doc_type]
    authorizer = uf_auth.get(uf, "SVRS")
    return list(endpoints.get(authorizer, {}).get(environment, {}).keys())


_UF_AUTH: dict[str, dict[str, str]] = {
    "nfe": _NFE_UF_AUTH,
    "nfce": _NFCE_UF_AUTH,
    "cte": _CTE_UF_AUTH,
    "mdfe": _MDFE_UF_AUTH,
}


def get_authorizer(doc_type: str, uf: str) -> str:
    return _UF_AUTH.get(doc_type, {}).get(uf, uf)
