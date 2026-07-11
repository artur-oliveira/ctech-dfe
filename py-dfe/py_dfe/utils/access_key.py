def _ak(access_key: str, from_i: int, to_i: int):
    assert len(access_key) == 44, 'Access key MUST have 44 digits'
    return access_key[from_i:to_i]


def parse_uf(access_key: str):
    return _ak(access_key, 0, 2)


def parse_cnpj(access_key: str):
    return _ak(access_key, 6, 20)
