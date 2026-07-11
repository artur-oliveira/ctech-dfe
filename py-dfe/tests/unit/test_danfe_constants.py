from py_dfe.constants import danfe as c


def test_service_and_layout_constants():
    assert c.SERVICE_GERAR_DANFE == "GerarDanfe"
    assert c.LAYOUT_COMPLETO == "completo"
    assert c.LAYOUT_RESUMIDO == "resumido"
    assert c.VALID_LAYOUTS == frozenset({"completo", "resumido"})


def test_emission_and_env_constants():
    assert c.TP_EMIS_NORMAL == "1"
    assert c.TP_EMIS_CONTINGENCIA_OFFLINE == "9"
    assert c.TP_AMB_PRODUCAO == "1"
    assert c.TP_AMB_HOMOLOGACAO == "2"
    assert c.MODELO_NFCE == "65"


def test_payment_labels():
    assert c.TPAG_LABELS["01"] == "Dinheiro"
    assert c.TPAG_LABELS["03"] == "Cartão de Crédito"
    assert c.TPAG_LABELS["17"] == "Pagamento Instantâneo (PIX) - Dinâmico"
    assert c.TPAG_LABELS["99"] == "Outros"


def test_error_codes_exist():
    from py_dfe import exceptions as e
    assert e.DANFE_INVALID_XML
    assert e.DANFE_UNSUPPORTED_MODEL
    assert e.DANFE_MISSING_QRCODE
    assert e.DANFE_RENDER_FAILED
    assert e.CERT_REQUIRED


def test_danfe_nfe_layout_constants():
    assert c.MODELO_NFE == "55"
    assert c.DEFAULT_DANFE_NFE_LAYOUT == c.LAYOUT_RETRATO
    assert c.VALID_DANFE_NFE_LAYOUTS == {
        c.LAYOUT_RETRATO, c.LAYOUT_PAISAGEM,
        c.LAYOUT_SIMPLIFICADO, c.LAYOUT_ETIQUETA,
    }
    # Every layout maps to a template; roll layouts are a subset.
    assert set(c.DANFE_NFE_TEMPLATES) == c.VALID_DANFE_NFE_LAYOUTS
    assert c.ROLL_LAYOUTS == {c.LAYOUT_SIMPLIFICADO, c.LAYOUT_ETIQUETA}


def test_danfe_nfe_tpemis_groups():
    assert c.TP_EMIS_NORMAL in c.TP_EMIS_NORMAL_LIKE
    assert {c.TP_EMIS_SVC_AN, c.TP_EMIS_SVC_RS, c.TP_EMIS_SCAN} <= c.TP_EMIS_NORMAL_LIKE
    assert c.TP_EMIS_FS_LIKE == {c.TP_EMIS_FS, c.TP_EMIS_FSDA}
    assert c.TP_EMIS_EPEC not in c.TP_EMIS_NORMAL_LIKE
    assert c.TP_EMIS_EPEC not in c.TP_EMIS_FS_LIKE


def test_danfe_invalid_barcode_code():
    from py_dfe.exceptions import DANFE_INVALID_BARCODE
    assert DANFE_INVALID_BARCODE == "danfe invalid barcode"
