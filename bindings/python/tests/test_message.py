"""Tests for the MIST Python message type."""

from mist._runner import Message


def test_message_to_json():
    msg = Message(source="python", type="health.ping", payload={"from": "test"})
    data = msg.to_json()
    assert '"source":"python"' in data.replace(" ", "")
    assert '"health.ping"' in data


def test_message_from_json():
    raw = '{"version":"1","id":"abc","source":"go","type":"health.pong","timestamp_ns":0,"payload":{"from":"go"}}'
    msg = Message.from_json(raw)
    assert msg.source == "go"
    assert msg.type == "health.pong"
    assert msg.payload["from"] == "go"


def test_message_roundtrip():
    original = Message(source="test", type="eval.run", payload={"suite": "math"})
    restored = Message.from_json(original.to_json())
    assert restored.source == original.source
    assert restored.type == original.type
    assert restored.payload == original.payload
