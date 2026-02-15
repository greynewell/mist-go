---
title: Language Bindings
slug: bindings
order: 5
---

# Language Bindings

MIST tools are Go binaries with bindings for Python and TypeScript.
Bindings wrap the binary as a subprocess, communicating over the MIST
protocol via stdio.

## Python

```bash
pip install mist-sdk
```

```python
from mist import Client

client = Client("matchspec")
version = client.version()
result = client.send("eval.run", {"suite": "math"})
```

## TypeScript

```bash
npm install @mist-stack/sdk
```

```typescript
import { Client } from "@mist-stack/sdk";

const client = new Client("matchspec");
const version = await client.version();
const result = await client.send("eval.run", { suite: "math" });
```

## Binary Resolution

Both SDKs look for the Go binary in this order:

1. `MIST_BIN_DIR` environment variable
2. Bundled `bin/` directory inside the package
3. System PATH

## Building Bindings Packages

The release workflow cross-compiles Go binaries and bundles them into
platform-specific Python wheels and npm packages.
