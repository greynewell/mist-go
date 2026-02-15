"""MIST SDK for Python.

Wraps the mist-go binary for cross-language interop. Each MIST tool
ships as a platform-specific Go binary bundled inside this package.

    from mist import Client

    client = Client()
    result = client.send("health.ping", {"from": "python"})
"""

from mist._runner import Client, Message, MistError

__all__ = ["Client", "Message", "MistError"]
__version__ = "0.0.1"
