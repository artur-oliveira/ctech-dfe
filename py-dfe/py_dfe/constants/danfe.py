"""Constants for auxiliary fiscal document (DANFE) generation."""

# Render service key (routed in _NFServiceClient.call, no SEFAZ call).
SERVICE_GERAR_DANFE = "GerarDanfe"

# DANFC-e layout variants.
LAYOUT_COMPLETO = "completo"
LAYOUT_RESUMIDO = "resumido"
VALID_LAYOUTS = frozenset({LAYOUT_COMPLETO, LAYOUT_RESUMIDO})

# ide/tpEmis (NFC-e). Only "9" (offline contingency) triggers the 2-vias layout.
TP_EMIS_NORMAL = "1"
TP_EMIS_CONTINGENCIA_OFFLINE = "9"

# ide/tpAmb.
TP_AMB_PRODUCAO = "1"
TP_AMB_HOMOLOGACAO = "2"

# Document model code for NFC-e.
MODELO_NFCE = "65"

# Fixed copy (manual_danfce.md).
TEXT_DOC_AUXILIAR = "Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica"
TEXT_CONTINGENCIA_L1 = "EMITIDA EM CONTINGÊNCIA"
TEXT_CONTINGENCIA_L2 = "Pendente de autorização"
TEXT_HOMOLOGACAO = "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL"
TEXT_CONSUMIDOR_NAO_IDENTIFICADO = "CONSUMIDOR NÃO IDENTIFICADO"
VIA_CONSUMIDOR = "Via Consumidor"
VIA_ESTABELECIMENTO = "Via do Estabelecimento"
TEXT_WATERMARK_CANCELADA = "CANCELADA"

# tPag → label (SEFAZ payment-type table, manual §3.1.3).
TPAG_LABELS = {
    "01": "Dinheiro",
    "02": "Cheque",
    "03": "Cartão de Crédito",
    "04": "Cartão de Débito",
    "05": "Crédito Loja",
    "10": "Vale Alimentação",
    "11": "Vale Refeição",
    "12": "Vale Presente",
    "13": "Vale Combustível",
    "15": "Boleto Bancário",
    "16": "Depósito Bancário",
    "17": "Pagamento Instantâneo (PIX) - Dinâmico",
    "18": "Transferência bancária, Carteira Digital",
    "19": "Programa de fidelidade, Cashback, Crédito Virtual",
    "20": "Pagamento Instantâneo (PIX) - Estático",
    "21": "Crédito em loja",
    "22": "Pagamento Eletrônico não Informado",
    "90": "Sem pagamento",
    "99": "Outros",
}

# ---------------------------------------------------------------------------
# DANF-e (NF-e modelo 55) — manual_danfe.md (MOC 7.0 Anexo II)
# ---------------------------------------------------------------------------

# Document model code for NF-e.
MODELO_NFE = "55"

# DANF-e layout variants (the `layout` payload key, mod-55 valid set).
LAYOUT_RETRATO = "retrato"
LAYOUT_PAISAGEM = "paisagem"
LAYOUT_SIMPLIFICADO = "simplificado"
LAYOUT_ETIQUETA = "etiqueta"
VALID_DANFE_NFE_LAYOUTS = frozenset(
    {LAYOUT_RETRATO, LAYOUT_PAISAGEM, LAYOUT_SIMPLIFICADO, LAYOUT_ETIQUETA}
)
DEFAULT_DANFE_NFE_LAYOUT = LAYOUT_RETRATO

# Layout → template filename (under danfe/templates/).
DANFE_NFE_TEMPLATES = {
    LAYOUT_RETRATO: "danfe_retrato.html",
    LAYOUT_PAISAGEM: "danfe_paisagem.html",
    LAYOUT_SIMPLIFICADO: "danfe_simplificado.html",
    LAYOUT_ETIQUETA: "danfe_etiqueta.html",
}
# Roll/auto-height layouts use fit_height=True; A4 layouts use fit_height=False.
ROLL_LAYOUTS = frozenset({LAYOUT_SIMPLIFICADO, LAYOUT_ETIQUETA})

# ide/tpEmis (NF-e; manual §3.9 + MOC Anexo I cap.2). TP_EMIS_NORMAL="1" above.
TP_EMIS_FS = "2"
TP_EMIS_SCAN = "3"          # deprecated; printed like a normal emission
TP_EMIS_EPEC = "4"
TP_EMIS_FSDA = "5"
TP_EMIS_SVC_AN = "6"
TP_EMIS_SVC_RS = "7"
# Normal-like: single chave barcode + protocolo de autorização (§3.9.1).
TP_EMIS_NORMAL_LIKE = frozenset(
    {TP_EMIS_NORMAL, TP_EMIS_SCAN, TP_EMIS_SVC_AN, TP_EMIS_SVC_RS}
)
# FS-like: second "Dados da NF-e" barcode, protocolo suppressed (§3.9.2).
TP_EMIS_FS_LIKE = frozenset({TP_EMIS_FS, TP_EMIS_FSDA})

# ide/tpNF.
TP_NF_ENTRADA = "0"
TP_NF_SAIDA = "1"
TP_NF_LABELS = {TP_NF_ENTRADA: "ENTRADA", TP_NF_SAIDA: "SAÍDA"}

# transp/modFrete (manual §3.1.10).
MOD_FRETE_LABELS = {
    "0": "0 - Remetente",
    "1": "1 - Destinatário",
    "2": "2 - Terceiros",
    "3": "3 - Próprio (Remetente)",
    "4": "4 - Próprio (Destinatário)",
    "9": "9 - Sem Frete",
}

# Fixed copy (manual_danfe.md).
TEXT_DANFE = "DANFE"
TEXT_DANFE_DESC = "DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA"
TEXT_DANFE_SIMPLIFICADO = "DANFE Simplificado"
TEXT_DANFE_ETIQUETA = "DANFE Simplificado - Etiqueta"
TEXT_NFE_HOMOLOGACAO = "SEM VALOR FISCAL"
TEXT_NFE_CONTINGENCIA = (
    "DANFE EM CONTINGÊNCIA - IMPRESSO EM DECORRÊNCIA DE PROBLEMAS TÉCNICOS"
)
TEXT_PROTOCOLO = "PROTOCOLO DE AUTORIZAÇÃO DE USO"
TEXT_PROTOCOLO_EPEC = "PROTOCOLO DE AUTORIZAÇÃO DO EPEC"
TEXT_CONSULTA_NFE = (
    "Consulta de autenticidade no portal nacional da NF-e "
    "www.nfe.fazenda.gov.br/portal ou no site da Sefaz Autorizadora"
)
TEXT_DADOS_NFE = "DADOS DA NF-E"

# Footer (both DANF-e and DANFC-e). Override per-request with payload "site".
TEXT_GERADO_POR = "Gerado por"
DEFAULT_FOOTER_SITE = "https://dfe.aoctech.app"

# ---------------------------------------------------------------------------
# DAMDFE (MDF-e modelo 58) — manual_damdfe.md (MOC 3.00b Anexo II)
# ---------------------------------------------------------------------------

# Render service key (routed in MDFeServiceClient.call, no SEFAZ call).
SERVICE_GERAR_DAMDFE = "GerarDamdfe"

# Render-only services need no A1 certificate (checked in handler).
RENDER_ONLY_SERVICES = frozenset({SERVICE_GERAR_DANFE, SERVICE_GERAR_DAMDFE})

# Document model code for MDF-e.
MODELO_MDFE = "58"

# DAMDFE layout variants (reuse the retrato/paisagem keys above).
VALID_DAMDFE_LAYOUTS = frozenset({LAYOUT_RETRATO, LAYOUT_PAISAGEM})
DEFAULT_DAMDFE_LAYOUT = LAYOUT_RETRATO
DAMDFE_TEMPLATES = {
    LAYOUT_RETRATO: "damdfe_retrato.html",
    LAYOUT_PAISAGEM: "damdfe_paisagem.html",
}

# ide/tpEmis (MDF-e). Normal reuses TP_EMIS_NORMAL="1".
TP_EMIS_MDFE_NORMAL = "1"
TP_EMIS_MDFE_CONTINGENCIA = "2"

# ide/modal (TModalMD).
MODAL_RODOVIARIO = "1"
MODAL_AEREO = "2"
MODAL_AQUAVIARIO = "3"
MODAL_FERROVIARIO = "4"
MODAL_LABELS = {
    MODAL_RODOVIARIO: "Rodoviário",
    MODAL_AEREO: "Aéreo",
    MODAL_AQUAVIARIO: "Aquaviário",
    MODAL_FERROVIARIO: "Ferroviário",
}

# ide/tpEmit (TEmit).
TP_EMIT_MDFE_LABELS = {
    "1": "Prestador de Serviço de Transporte",
    "2": "Transportador de Carga Própria",
    "3": "Prestador de Serviço de Transporte (CT-e Globalizado)",
}

# ide/tpTransp (TTransp).
TP_TRANSP_MDFE_LABELS = {
    "1": "ETC",
    "2": "TAC",
    "3": "CTC",
}

# prodPred/tpCarga.
TP_CARGA_LABELS = {
    "01": "Granel sólido",
    "02": "Granel líquido",
    "03": "Frigorificada",
    "04": "Conteinerizada",
    "05": "Carga Geral",
    "06": "Neogranel",
    "07": "Perigosa (granel sólido)",
    "08": "Perigosa (granel líquido)",
    "09": "Perigosa (carga frigorificada)",
    "10": "Perigosa (conteinerizada)",
    "11": "Perigosa (carga geral)",
}

# tot/cUnid (weight unit).
C_UNID_LABELS = {"01": "KG", "02": "TON"}

# Document key types listed under infDoc/infMunDescarga.
DOC_TIPO_NFE = "NF-e"
DOC_TIPO_CTE = "CT-e"
DOC_TIPO_MDFE = "MDF-e"

# Fixed copy (manual_damdfe.md).
TEXT_DAMDFE = "DAMDFE"
TEXT_DAMDFE_DESC = (
    "DOCUMENTO AUXILIAR DO MANIFESTO ELETRÔNICO DE DOCUMENTOS FISCAIS"
)
TEXT_MDFE_CONTINGENCIA = "EMISSÃO EM CONTINGÊNCIA"
TEXT_MDFE_HOMOLOGACAO = (
    "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL"
)
TEXT_PROTOCOLO_MDFE = "PROTOCOLO DE AUTORIZAÇÃO DE USO"
TEXT_DAMDFE_CONSULTA = (
    "Consulta em https://dfe-portal.svrs.rs.gov.br/mdfe/consulta"
)
