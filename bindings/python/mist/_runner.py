"""Subprocess wrapper for MIST Go binaries.

This module provides a Python-native interface to any MIST tool binary.
The Go binary is invoked as a subprocess, communicating over stdio using
the MIST JSON message protocol.
"""

from __future__ import annotations

import json
import os
import platform
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


class MistError(Exception):
    """Raised when a MIST tool returns an error."""


@dataclass
class Message:
    """A MIST protocol message."""

    version: str = "1"
    id: str = ""
    source: str = ""
    type: str = ""
    timestamp_ns: int = 0
    payload: dict[str, Any] = field(default_factory=dict)

    def to_json(self) -> str:
        return json.dumps(
            {
                "version": self.version,
                "id": self.id,
                "source": self.source,
                "type": self.type,
                "timestamp_ns": self.timestamp_ns,
                "payload": self.payload,
            }
        )

    @classmethod
    def from_json(cls, data: str) -> Message:
        d = json.loads(data)
        return cls(
            version=d.get("version", "1"),
            id=d.get("id", ""),
            source=d.get("source", ""),
            type=d.get("type", ""),
            timestamp_ns=d.get("timestamp_ns", 0),
            payload=d.get("payload", {}),
        )


def _binary_name(tool: str) -> str:
    """Resolve the platform-specific binary name."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    arch_map = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}
    arch = arch_map.get(machine, machine)

    ext = ".exe" if system == "windows" else ""
    return f"{tool}-{system}-{arch}{ext}"


def _find_binary(tool: str) -> Path:
    """Locate the tool binary. Search order:
    1. MIST_BIN_DIR environment variable
    2. Bundled in this package's bin/ directory
    3. System PATH
    """
    env_dir = os.environ.get("MIST_BIN_DIR")
    if env_dir:
        p = Path(env_dir) / _binary_name(tool)
        if p.exists():
            return p

    pkg_bin = Path(__file__).parent / "bin" / _binary_name(tool)
    if pkg_bin.exists():
        return pkg_bin

    # Fall back to bare tool name on PATH.
    return Path(tool)


class Client:
    """Runs a MIST tool binary and communicates via stdio."""

    def __init__(self, tool: str = "mist", timeout: float = 30.0):
        self.tool = tool
        self.timeout = timeout
        self._binary = _find_binary(tool)

    def call(self, args: list[str], stdin: str | None = None) -> str:
        """Run the tool with arguments and optional stdin, return stdout."""
        try:
            result = subprocess.run(
                [str(self._binary)] + args,
                input=stdin,
                capture_output=True,
                text=True,
                timeout=self.timeout,
            )
        except FileNotFoundError:
            raise MistError(f"binary not found: {self._binary}")
        except subprocess.TimeoutExpired:
            raise MistError(f"timeout after {self.timeout}s")

        if result.returncode != 0:
            raise MistError(result.stderr.strip() or f"exit code {result.returncode}")

        return result.stdout

    def send(self, msg_type: str, payload: dict[str, Any], source: str = "python") -> Message:
        """Send a message via stdio transport and return the response."""
        msg = Message(source=source, type=msg_type, payload=payload)
        out = self.call(["--transport", "stdio"], stdin=msg.to_json())
        if out.strip():
            return Message.from_json(out.strip().split("\n")[-1])
        return Message()

    def version(self) -> str:
        """Get the tool version."""
        return self.call(["version"]).strip()
