"""Tests for the MIST Python runner (binary resolution, client, errors)."""

import os
import platform
import stat
import tempfile

import pytest

from mist._runner import Client, MistError, _binary_name, _find_binary


def test_binary_name_format():
    name = _binary_name("matchspec")
    system = platform.system().lower()
    assert name.startswith(f"matchspec-{system}-")
    assert "amd64" in name or "arm64" in name


def test_binary_name_contains_tool():
    for tool in ["matchspec", "infermux", "schemaflux", "tokentrace"]:
        name = _binary_name(tool)
        assert name.startswith(tool)


def test_find_binary_env_dir(tmp_path):
    """MIST_BIN_DIR should take priority."""
    name = _binary_name("testool")
    bin_path = tmp_path / name
    bin_path.write_text("#!/bin/sh\necho test")
    bin_path.chmod(bin_path.stat().st_mode | stat.S_IEXEC)

    os.environ["MIST_BIN_DIR"] = str(tmp_path)
    try:
        found = _find_binary("testool")
        assert found == bin_path
    finally:
        del os.environ["MIST_BIN_DIR"]


def test_find_binary_falls_back_to_name():
    """When no binary exists, should fall back to bare tool name."""
    found = _find_binary("nonexistent-tool-xyz")
    assert str(found) == "nonexistent-tool-xyz"


def test_client_init():
    client = Client("matchspec", timeout=10.0)
    assert client.tool == "matchspec"
    assert client.timeout == 10.0


def test_client_call_missing_binary():
    client = Client("definitely-not-a-real-binary-xyz")
    with pytest.raises(MistError, match="binary not found"):
        client.call(["version"])


def test_client_call_with_echo(tmp_path):
    """Create a fake binary that echoes input."""
    script = tmp_path / "fake-tool"
    script.write_text('#!/bin/sh\necho "fake-tool 1.0.0"')
    script.chmod(script.stat().st_mode | stat.S_IEXEC)

    client = Client("fake-tool")
    client._binary = script
    result = client.call(["version"])
    assert "fake-tool 1.0.0" in result


def test_client_version_with_fake_binary(tmp_path):
    script = tmp_path / "fake-tool"
    script.write_text('#!/bin/sh\necho "mist-tool 0.1.0"')
    script.chmod(script.stat().st_mode | stat.S_IEXEC)

    client = Client("fake-tool")
    client._binary = script
    assert client.version() == "mist-tool 0.1.0"


def test_client_call_nonzero_exit(tmp_path):
    script = tmp_path / "fail-tool"
    script.write_text('#!/bin/sh\necho "something broke" >&2\nexit 1')
    script.chmod(script.stat().st_mode | stat.S_IEXEC)

    client = Client("fail-tool")
    client._binary = script
    with pytest.raises(MistError, match="something broke"):
        client.call(["any-arg"])


def test_client_call_timeout(tmp_path):
    script = tmp_path / "slow-tool"
    script.write_text("#!/bin/sh\nsleep 10")
    script.chmod(script.stat().st_mode | stat.S_IEXEC)

    client = Client("slow-tool", timeout=0.1)
    client._binary = script
    with pytest.raises(MistError, match="timeout"):
        client.call(["any"])


def test_client_call_stdin(tmp_path):
    """Binary should receive stdin."""
    script = tmp_path / "cat-tool"
    script.write_text("#!/bin/sh\ncat")
    script.chmod(script.stat().st_mode | stat.S_IEXEC)

    client = Client("cat-tool")
    client._binary = script
    result = client.call([], stdin='{"hello":"world"}')
    assert '{"hello":"world"}' in result


def test_client_send_with_echo_binary(tmp_path):
    """send() should serialize a message, pass to stdin, parse stdout."""
    script = tmp_path / "echo-tool"
    # Echo back a valid MIST message.
    script.write_text(
        '#!/bin/sh\n'
        'echo \'{"version":"1","id":"resp1","source":"go","type":"health.pong","timestamp_ns":0,"payload":{"from":"go"}}\''
    )
    script.chmod(script.stat().st_mode | stat.S_IEXEC)

    client = Client("echo-tool")
    client._binary = script
    resp = client.send("health.ping", {"from": "python"})
    assert resp.source == "go"
    assert resp.type == "health.pong"
    assert resp.id == "resp1"


def test_mist_error_message():
    err = MistError("test error")
    assert str(err) == "test error"
    assert isinstance(err, Exception)


def test_binary_name_no_extension_on_unix():
    if platform.system() != "Windows":
        name = _binary_name("tool")
        assert not name.endswith(".exe")
