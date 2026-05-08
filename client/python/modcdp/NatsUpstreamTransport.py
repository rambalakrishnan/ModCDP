from __future__ import annotations

from .UpstreamTransport import UpstreamTransport


class NatsUpstreamTransport(UpstreamTransport):
    mode = "nats"
    endpoint_kind = "modcdp_server"

    def __init__(self, url: str | None = None) -> None:
        super().__init__()
        self.url = url

    def connect(self) -> None:
        raise NotImplementedError("upstream.mode='nats' is not implemented by the Python client yet.")
