import json
from typing import Optional

import socks


def telethon_proxy_from_json(raw: str) -> Optional[tuple]:
    raw = (raw or "").strip()
    if not raw:
        return None
    data = json.loads(raw)
    protocol = (data.get("protocol") or "").strip().lower()
    host = (data.get("host") or "").strip()
    port = int(data.get("port") or 0)
    if not host or port <= 0:
        return None
    proxy_type = socks.HTTP if protocol in {"http", "https"} else socks.SOCKS5
    username = (data.get("username") or "").strip() or None
    password = (data.get("password") or "").strip() or None
    return (proxy_type, host, port, True, username, password)
