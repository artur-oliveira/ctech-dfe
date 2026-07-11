import base64
import gzip


def decompress(gzstr: str) -> 'str':
    return gzip.decompress(base64.b64decode(gzstr)).decode('utf-8')
