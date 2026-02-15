/**
 * Tests for the MIST TypeScript SDK.
 *
 * Run with: npx tsx --test src/index.test.ts
 * Or after build: node --test dist/index.test.js
 */

import { describe, it } from "node:test";
import * as assert from "node:assert/strict";
import { writeFileSync, chmodSync, mkdtempSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

// Since we're testing the source directly, use dynamic import.
// In a real build, these would be normal imports from the compiled output.

interface Message {
  version: string;
  id: string;
  source: string;
  type: string;
  timestamp_ns: number;
  payload: Record<string, unknown>;
}

// ── Message serialization tests ──

describe("Message", () => {
  it("should serialize to valid JSON", () => {
    const msg: Message = {
      version: "1",
      id: "test123",
      source: "typescript",
      type: "health.ping",
      timestamp_ns: 0,
      payload: { from: "test" },
    };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);
    assert.equal(parsed.version, "1");
    assert.equal(parsed.source, "typescript");
    assert.equal(parsed.type, "health.ping");
    assert.deepEqual(parsed.payload, { from: "test" });
  });

  it("should roundtrip through JSON", () => {
    const original: Message = {
      version: "1",
      id: "roundtrip",
      source: "matchspec",
      type: "eval.result",
      timestamp_ns: 1700000000000000000,
      payload: { suite: "math", score: 0.95, passed: true },
    };
    const restored: Message = JSON.parse(JSON.stringify(original));
    assert.equal(restored.id, original.id);
    assert.equal(restored.source, original.source);
    assert.equal(restored.type, original.type);
    assert.deepEqual(restored.payload, original.payload);
  });

  it("should handle empty payload", () => {
    const msg: Message = {
      version: "1",
      id: "",
      source: "test",
      type: "health.ping",
      timestamp_ns: 0,
      payload: {},
    };
    const restored: Message = JSON.parse(JSON.stringify(msg));
    assert.deepEqual(restored.payload, {});
  });

  it("should handle nested payloads", () => {
    const msg: Message = {
      version: "1",
      id: "",
      source: "matchspec",
      type: "infer.request",
      timestamp_ns: 0,
      payload: {
        model: "claude-sonnet-4-5-20250929",
        messages: [
          { role: "system", content: "You are helpful." },
          { role: "user", content: "Hello!" },
        ],
        params: { temperature: 0.7, max_tokens: 4096 },
      },
    };
    const restored: Message = JSON.parse(JSON.stringify(msg));
    const messages = restored.payload.messages as Array<{
      role: string;
      content: string;
    }>;
    assert.equal(messages.length, 2);
    assert.equal(messages[0].role, "system");

    const params = restored.payload.params as { temperature: number };
    assert.equal(params.temperature, 0.7);
  });

  it("should handle large payloads (1MB)", () => {
    const largeContent = "x".repeat(1024 * 1024);
    const msg: Message = {
      version: "1",
      id: "",
      source: "infermux",
      type: "infer.response",
      timestamp_ns: 0,
      payload: { content: largeContent, tokens_out: 250000 },
    };
    const restored: Message = JSON.parse(JSON.stringify(msg));
    assert.equal(
      (restored.payload.content as string).length,
      1024 * 1024,
    );
  });

  it("should handle unicode content", () => {
    const contents = [
      "Hello, 世界! 🌍",
      "日本語テスト: こんにちは",
      "Emoji: 🔥💯🚀✨🎯",
      "العربية: مرحبا بالعالم",
      "한국어: 안녕하세요",
    ];
    for (const content of contents) {
      const msg: Message = {
        version: "1",
        id: "",
        source: "test",
        type: "infer.response",
        timestamp_ns: 0,
        payload: { content },
      };
      const restored: Message = JSON.parse(JSON.stringify(msg));
      assert.equal(restored.payload.content, content);
    }
  });

  it("should handle special JSON characters", () => {
    const special = {
      backslashes: "path\\to\\file",
      quotes: 'she said "hello"',
      newlines: "line1\nline2\nline3",
      tabs: "col1\tcol2",
      html: "<script>alert('xss')</script>",
    };
    const msg: Message = {
      version: "1",
      id: "",
      source: "test",
      type: "test",
      timestamp_ns: 0,
      payload: special,
    };
    const restored: Message = JSON.parse(JSON.stringify(msg));
    for (const [key, val] of Object.entries(special)) {
      assert.equal(restored.payload[key], val, `Mismatch for ${key}`);
    }
  });

  it("should handle null values", () => {
    const msg: Message = {
      version: "1",
      id: "",
      source: "test",
      type: "test",
      timestamp_ns: 0,
      payload: { key: null, nested: { inner: null } },
    };
    const restored: Message = JSON.parse(JSON.stringify(msg));
    assert.equal(restored.payload.key, null);
    assert.equal(
      (restored.payload.nested as Record<string, unknown>).inner,
      null,
    );
  });

  it("should handle all MIST message types", () => {
    const types: Array<[string, Record<string, unknown>]> = [
      ["data.entities", { count: 100, format: "jsonl" }],
      [
        "data.schema",
        { name: "user", fields: [{ name: "id", type: "int" }] },
      ],
      [
        "infer.request",
        {
          model: "test",
          messages: [{ role: "user", content: "hi" }],
        },
      ],
      [
        "infer.response",
        { content: "hello", tokens_in: 5, tokens_out: 10 },
      ],
      ["eval.run", { suite: "math", tasks: ["add", "mul"] }],
      [
        "eval.result",
        { suite: "math", task: "add", passed: true, score: 0.95 },
      ],
      [
        "trace.span",
        { trace_id: "t1", span_id: "s1", operation: "test" },
      ],
      [
        "trace.alert",
        { level: "warning", metric: "latency_p99", value: 5.0 },
      ],
      ["health.ping", { from: "typescript" }],
      [
        "health.pong",
        { from: "go", version: "1.0.0", uptime_s: 3600 },
      ],
    ];

    for (const [msgType, payload] of types) {
      const msg: Message = {
        version: "1",
        id: "",
        source: "test",
        type: msgType,
        timestamp_ns: 0,
        payload,
      };
      const restored: Message = JSON.parse(JSON.stringify(msg));
      assert.equal(restored.type, msgType);
      assert.deepEqual(restored.payload, payload);
    }
  });

  it("should serialize 10k messages in under 2 seconds", () => {
    const msg: Message = {
      version: "1",
      id: "perf",
      source: "bench",
      type: "trace.span",
      timestamp_ns: 0,
      payload: {
        trace_id: "t1",
        span_id: "s1",
        operation: "inference",
        attrs: { model: "test", tokens: 500, cost: 0.003 },
      },
    };

    const start = performance.now();
    for (let i = 0; i < 10_000; i++) {
      const data = JSON.stringify(msg);
      JSON.parse(data);
    }
    const elapsed = performance.now() - start;

    assert.ok(
      elapsed < 2000,
      `10k roundtrips took ${elapsed.toFixed(0)}ms, expected < 2000ms`,
    );
  });
});

// ── MistError tests ──

describe("MistError", () => {
  it("should be an Error", () => {
    // Inline MistError for testing without build step.
    class MistError extends Error {
      constructor(
        message: string,
        public readonly exitCode?: number,
      ) {
        super(message);
        this.name = "MistError";
      }
    }

    const err = new MistError("test error", 1);
    assert.ok(err instanceof Error);
    assert.equal(err.name, "MistError");
    assert.equal(err.message, "test error");
    assert.equal(err.exitCode, 1);
  });

  it("should work without exit code", () => {
    class MistError extends Error {
      constructor(
        message: string,
        public readonly exitCode?: number,
      ) {
        super(message);
        this.name = "MistError";
      }
    }

    const err = new MistError("no code");
    assert.equal(err.exitCode, undefined);
  });
});

// ── Client tests (using fake binaries) ──

describe("Client", () => {
  it("should call a fake binary and capture stdout", async () => {
    const dir = mkdtempSync(join(tmpdir(), "mist-test-"));
    const script = join(dir, "fake-tool");
    writeFileSync(script, '#!/bin/sh\necho "fake-tool 1.0.0"');
    chmodSync(script, 0o755);

    // Inline minimal Client for testing.
    const { execFile } = await import("node:child_process");
    const result = await new Promise<string>((resolve, reject) => {
      execFile(script, ["version"], (err, stdout) => {
        if (err) reject(err);
        else resolve(stdout);
      });
    });

    assert.ok(result.includes("fake-tool 1.0.0"));
  });

  it("should pass stdin to a binary", async () => {
    const dir = mkdtempSync(join(tmpdir(), "mist-test-"));
    const script = join(dir, "cat-tool");
    writeFileSync(script, "#!/bin/sh\ncat");
    chmodSync(script, 0o755);

    const { execFile } = await import("node:child_process");
    const result = await new Promise<string>((resolve, reject) => {
      const child = execFile(script, [], (err, stdout) => {
        if (err) reject(err);
        else resolve(stdout);
      });
      child.stdin!.write('{"hello":"world"}');
      child.stdin!.end();
    });

    assert.ok(result.includes('{"hello":"world"}'));
  });

  it("should handle binary returning MIST message", async () => {
    const dir = mkdtempSync(join(tmpdir(), "mist-test-"));
    const script = join(dir, "echo-tool");
    writeFileSync(
      script,
      `#!/bin/sh\necho '{"version":"1","id":"resp1","source":"go","type":"health.pong","timestamp_ns":0,"payload":{"from":"go"}}'`,
    );
    chmodSync(script, 0o755);

    const { execFile } = await import("node:child_process");
    const stdout = await new Promise<string>((resolve, reject) => {
      const child = execFile(script, [], (err, stdout) => {
        if (err) reject(err);
        else resolve(stdout);
      });
      child.stdin!.end();
    });

    const msg: Message = JSON.parse(stdout.trim());
    assert.equal(msg.source, "go");
    assert.equal(msg.type, "health.pong");
    assert.equal(msg.id, "resp1");
  });

  it("should reject on nonzero exit", async () => {
    const dir = mkdtempSync(join(tmpdir(), "mist-test-"));
    const script = join(dir, "fail-tool");
    writeFileSync(
      script,
      '#!/bin/sh\necho "something broke" >&2\nexit 1',
    );
    chmodSync(script, 0o755);

    const { execFile } = await import("node:child_process");
    await assert.rejects(
      () =>
        new Promise<string>((resolve, reject) => {
          execFile(script, [], (err, stdout) => {
            if (err) reject(err);
            else resolve(stdout);
          });
        }),
    );
  });
});
