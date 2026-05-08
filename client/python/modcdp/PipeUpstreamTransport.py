from __future__ import annotations

from .UpstreamTransport import UpstreamTransport


class PipeUpstreamTransport(UpstreamTransport):
    mode = "pipe"
    endpoint_kind = "raw_cdp"

    def connect(self) -> None:
        raise NotImplementedError("upstream.mode='pipe' is not implemented by the Python client yet.")
