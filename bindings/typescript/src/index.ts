/**
 * MIST SDK for TypeScript/Node.js.
 *
 * Wraps the mist-go binary for cross-language interop. Each MIST tool
 * ships as a platform-specific Go binary invoked as a child process.
 *
 * @example
 * ```ts
 * import { Client } from "@mist-stack/sdk";
 *
 * const client = new Client("matchspec");
 * const version = await client.version();
 * ```
 */

import { execFile } from "node:child_process";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { arch, platform } from "node:os";

export interface Message {
  version: string;
  id: string;
  source: string;
  type: string;
  timestamp_ns: number;
  payload: Record<string, unknown>;
}

export class MistError extends Error {
  constructor(
    message: string,
    public readonly exitCode?: number,
  ) {
    super(message);
    this.name = "MistError";
  }
}

function binaryName(tool: string): string {
  const os = platform() === "win32" ? "windows" : platform();
  const archMap: Record<string, string> = {
    x64: "amd64",
    arm64: "arm64",
  };
  const goArch = archMap[arch()] ?? arch();
  const ext = platform() === "win32" ? ".exe" : "";
  return `${tool}-${os}-${goArch}${ext}`;
}

function findBinary(tool: string): string {
  const envDir = process.env.MIST_BIN_DIR;
  if (envDir) {
    const p = join(envDir, binaryName(tool));
    if (existsSync(p)) return p;
  }

  const pkgBin = join(__dirname, "..", "bin", binaryName(tool));
  if (existsSync(pkgBin)) return pkgBin;

  return tool;
}

export class Client {
  private binary: string;
  private timeout: number;

  constructor(
    public readonly tool: string = "mist",
    options?: { timeout?: number },
  ) {
    this.binary = findBinary(tool);
    this.timeout = options?.timeout ?? 30_000;
  }

  call(args: string[], stdin?: string): Promise<string> {
    return new Promise((resolve, reject) => {
      const child = execFile(
        this.binary,
        args,
        { timeout: this.timeout, maxBuffer: 10 * 1024 * 1024 },
        (error, stdout, stderr) => {
          if (error) {
            reject(
              new MistError(
                stderr?.trim() || error.message,
                error.code ? Number(error.code) : undefined,
              ),
            );
            return;
          }
          resolve(stdout);
        },
      );

      if (stdin && child.stdin) {
        child.stdin.write(stdin);
        child.stdin.end();
      }
    });
  }

  async send(
    msgType: string,
    payload: Record<string, unknown>,
    source = "typescript",
  ): Promise<Message> {
    const msg: Message = {
      version: "1",
      id: "",
      source,
      type: msgType,
      timestamp_ns: 0,
      payload,
    };

    const out = await this.call(
      ["--transport", "stdio"],
      JSON.stringify(msg),
    );

    const lines = out.trim().split("\n");
    const last = lines[lines.length - 1];
    return last ? JSON.parse(last) : msg;
  }

  async version(): Promise<string> {
    const out = await this.call(["version"]);
    return out.trim();
  }
}
