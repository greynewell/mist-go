"""Tests for the MIST Python message type."""

import json
import time

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


def test_message_defaults():
    msg = Message()
    assert msg.version == "1"
    assert msg.id == ""
    assert msg.source == ""
    assert msg.type == ""
    assert msg.timestamp_ns == 0
    assert msg.payload == {}


def test_message_all_fields():
    msg = Message(
        version="1",
        id="abc123def456",
        source="matchspec",
        type="eval.result",
        timestamp_ns=1700000000000000000,
        payload={"suite": "math", "score": 0.95, "passed": True},
    )
    data = json.loads(msg.to_json())
    assert data["version"] == "1"
    assert data["id"] == "abc123def456"
    assert data["source"] == "matchspec"
    assert data["type"] == "eval.result"
    assert data["timestamp_ns"] == 1700000000000000000
    assert data["payload"]["score"] == 0.95
    assert data["payload"]["passed"] is True


def test_message_empty_payload():
    msg = Message(source="test", type="health.ping", payload={})
    restored = Message.from_json(msg.to_json())
    assert restored.payload == {}


def test_message_nested_payload():
    nested = {
        "model": "claude-sonnet-4-5-20250929",
        "messages": [
            {"role": "system", "content": "You are helpful."},
            {"role": "user", "content": "Hello!"},
        ],
        "params": {"temperature": 0.7, "max_tokens": 4096},
        "meta": {"trace_id": "t1", "tags": ["eval", "stress"]},
    }
    msg = Message(source="matchspec", type="infer.request", payload=nested)
    restored = Message.from_json(msg.to_json())

    assert restored.payload["model"] == "claude-sonnet-4-5-20250929"
    assert len(restored.payload["messages"]) == 2
    assert restored.payload["messages"][0]["role"] == "system"
    assert restored.payload["params"]["temperature"] == 0.7
    assert restored.payload["meta"]["tags"] == ["eval", "stress"]


def test_message_large_payload():
    """1MB payload should roundtrip correctly."""
    large_content = "x" * (1024 * 1024)
    msg = Message(
        source="infermux",
        type="infer.response",
        payload={"content": large_content, "tokens_out": 250000},
    )
    restored = Message.from_json(msg.to_json())
    assert len(restored.payload["content"]) == 1024 * 1024
    assert restored.payload["content"] == large_content


def test_message_unicode_payload():
    payloads = [
        {"content": "Hello, 世界! 🌍"},
        {"content": "日本語テスト: こんにちは"},
        {"content": "Emoji: 🔥💯🚀✨🎯"},
        {"content": "العربية: مرحبا بالعالم"},
        {"content": "한국어: 안녕하세요"},
    ]
    for p in payloads:
        msg = Message(source="test", type="infer.response", payload=p)
        restored = Message.from_json(msg.to_json())
        assert restored.payload["content"] == p["content"], f"Failed for: {p['content'][:20]}"


def test_message_special_json_chars():
    special = {
        "backslashes": "path\\to\\file",
        "quotes": 'she said "hello"',
        "newlines": "line1\nline2\nline3",
        "tabs": "col1\tcol2",
        "html": "<script>alert('xss')</script>",
        "ampersand": "a&b&c",
    }
    msg = Message(source="test", type="test", payload=special)
    restored = Message.from_json(msg.to_json())
    for key, val in special.items():
        assert restored.payload[key] == val, f"Mismatch for {key}"


def test_message_numeric_precision():
    payload = {
        "large_int": 9999999999999,
        "negative": -42,
        "zero": 0,
        "float_precise": 0.123456789012345,
        "scientific": 1.5e10,
    }
    msg = Message(source="test", type="test", payload=payload)
    restored = Message.from_json(msg.to_json())
    assert restored.payload["large_int"] == 9999999999999
    assert restored.payload["negative"] == -42
    assert restored.payload["float_precise"] == 0.123456789012345


def test_message_null_values():
    payload = {"key": None, "nested": {"inner": None}}
    msg = Message(source="test", type="test", payload=payload)
    restored = Message.from_json(msg.to_json())
    assert restored.payload["key"] is None
    assert restored.payload["nested"]["inner"] is None


def test_message_from_json_missing_fields():
    """Partial JSON should use defaults for missing fields."""
    raw = '{"source":"partial","payload":{}}'
    msg = Message.from_json(raw)
    assert msg.source == "partial"
    assert msg.version == "1"
    assert msg.id == ""
    assert msg.type == ""
    assert msg.timestamp_ns == 0


def test_message_high_volume_roundtrip():
    """10k messages should roundtrip without corruption."""
    for i in range(10_000):
        msg = Message(
            source=f"sender-{i % 10}",
            type="trace.span",
            payload={
                "trace_id": f"t-{i}",
                "span_id": f"s-{i}",
                "operation": "test",
                "iter": i,
            },
        )
        restored = Message.from_json(msg.to_json())
        assert restored.payload["trace_id"] == f"t-{i}"
        assert restored.payload["iter"] == i


def test_message_serialization_performance():
    """Serialization should handle 10k msgs in under 2 seconds."""
    msg = Message(
        source="bench",
        type="trace.span",
        payload={
            "trace_id": "t1",
            "span_id": "s1",
            "operation": "inference",
            "attrs": {"model": "test", "tokens": 500, "cost": 0.003},
        },
    )

    start = time.monotonic()
    for _ in range(10_000):
        data = msg.to_json()
        Message.from_json(data)
    elapsed = time.monotonic() - start

    assert elapsed < 2.0, f"10k roundtrips took {elapsed:.2f}s, expected < 2s"


def test_message_all_mist_types():
    """Every MIST message type should roundtrip."""
    types = [
        ("data.entities", {"count": 100, "format": "jsonl"}),
        ("data.schema", {"name": "user", "fields": [{"name": "id", "type": "int"}]}),
        ("infer.request", {"model": "test", "messages": [{"role": "user", "content": "hi"}]}),
        ("infer.response", {"content": "hello", "tokens_in": 5, "tokens_out": 10}),
        ("eval.run", {"suite": "math", "tasks": ["add", "mul"]}),
        ("eval.result", {"suite": "math", "task": "add", "passed": True, "score": 0.95}),
        ("trace.span", {"trace_id": "t1", "span_id": "s1", "operation": "test"}),
        ("trace.alert", {"level": "warning", "metric": "latency_p99", "value": 5.0}),
        ("health.ping", {"from": "python"}),
        ("health.pong", {"from": "go", "version": "1.0.0", "uptime_s": 3600}),
    ]
    for msg_type, payload in types:
        msg = Message(source="test", type=msg_type, payload=payload)
        restored = Message.from_json(msg.to_json())
        assert restored.type == msg_type
        assert restored.payload == payload


def test_message_to_json_is_valid_json():
    msg = Message(source="test", type="health.ping", payload={"from": "test"})
    data = msg.to_json()
    parsed = json.loads(data)
    assert isinstance(parsed, dict)
    assert "version" in parsed
    assert "payload" in parsed
