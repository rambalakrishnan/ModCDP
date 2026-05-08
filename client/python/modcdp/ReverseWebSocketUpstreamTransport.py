from __future__ import annotations

from .UpstreamTransport import UpstreamTransport


DEFAULT_REVERSEWS_BIND = "127.0.0.1:29292"


class ReverseWebSocketUpstreamTransport(UpstreamTransport):
    mode = "reversews"
    endpoint_kind = "modcdp_server"

    def __init__(self, bind: str = DEFAULT_REVERSEWS_BIND) -> None:
        super().__init__()
        self.url = bind

    def connect(self) -> None:
        raise NotImplementedError("upstream.mode='reversews' is not implemented by the Python client yet.")
