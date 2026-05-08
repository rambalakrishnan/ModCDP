from __future__ import annotations

import json
from typing import Any

from websocket import create_connection

from .UpstreamTransport import UpstreamTransport


class WebSocketUpstreamTransport(UpstreamTransport):
    mode = "ws"
    endpoint_kind = "raw_cdp"

    def __init__(self, url: str, timeout_s: float = 10) -> None:
        super().__init__()
        self.url = url
        self.timeout_s = timeout_s
        self.ws: Any | None = None

    def connect(self) -> None:
        self.ws = create_connection(self.url, timeout=self.timeout_s)

    def send(self, message: dict[str, Any]) -> None:
        if self.ws is None:
            raise RuntimeError("CDP websocket is not connected.")
        self.ws.send(json.dumps(message))

    def recv(self) -> Any:
        if self.ws is None:
            raise RuntimeError("CDP websocket is not connected.")
        return self.ws.recv()

    def close(self) -> None:
        if self.ws is not None:
            self.ws.close()
        self.ws = None
