import datetime

_TZ = datetime.timezone(datetime.timedelta(hours=-3))


def _now() -> datetime.datetime:
    return datetime.datetime.now(tz=_TZ)


def dh():
    return _now().strftime("%Y-%m-%dT%H:%M:%S-03:00")
