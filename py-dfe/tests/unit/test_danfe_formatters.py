from py_dfe.danfe import formatters as f


def test_money_br():
    assert f.money_br("1234.5") == "1.234,50"
    assert f.money_br("0") == "0,00"
    assert f.money_br(60.9) == "60,90"
    assert f.money_br(None) == "0,00"
    assert f.money_br("") == "0,00"
    assert f.money_br("1000000") == "1.000.000,00"


def test_dt_local():
    assert f.dt_local("2026-06-25T10:30:00-03:00") == "25/06/2026 10:30:00"
    assert f.dt_local("") == ""
    assert f.dt_local(None) == ""


def test_date_br():
    assert f.date_br("2026-06-25T10:30:00-03:00") == "25/06/2026"
    assert f.date_br("") == ""
    assert f.date_br(None) == ""


def test_time_br():
    assert f.time_br("2026-06-25T10:30:00-03:00") == "10:30:00"
    assert f.time_br("") == ""
    assert f.time_br(None) == ""


def test_mask_cnpj():
    assert f.mask_cnpj("12345678000199") == "12.345.678/0001-99"


def test_mask_cpf():
    assert f.mask_cpf("12345678909") == "123.456.789-09"


def test_chave_blocks():
    key = "28170800156225000131650110000151341562040824"[:44]
    out = f.chave_blocks(key)
    assert out.count(" ") == 10
    assert all(len(b) == 4 for b in out.split(" "))
    assert out.replace(" ", "") == key


def test_mask_cpf_cnpj_picks_format_by_length():
    assert f.mask_cpf_cnpj("12345678000199") == "12.345.678/0001-99"
    assert f.mask_cpf_cnpj("12345678909") == "123.456.789-09"
    assert f.mask_cpf_cnpj("") == ""
    assert f.mask_cpf_cnpj("123") == "123"  # unknown length → unchanged


def test_num_nf_groups_nine_digits():
    assert f.num_nf("1") == "000.000.001"
    assert f.num_nf("123456789") == "123.456.789"
    assert f.num_nf("") == ""


def test_mask_cep():
    assert f.mask_cep("01000000") == "01000-000"
    assert f.mask_cep("123") == "123"


def test_pct():
    assert f.pct("18.00") == "18,00"
    assert f.pct(None) == "0,00"
