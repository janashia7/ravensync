# Contributing

Ravensync is a single Go module with a clean internal package structure.
This guide explains the architecture, data flow, and where new features belong.

## Architecture

```
Layer 0 — Entry
  └─ cmd/ravensync          main.go — cobra root

Layer 1 — CLI
  └─ internal/cli            init, serve, doctor, stats, version, theme

Layer 2 — Core
  ├─ internal/agent          RAG loop, conversation history, LLM orchestration
  ├─ internal/llm            LLM provider interface (OpenAI, Ollama, Gemini, etc.)
  ├─ internal/memory         Encrypted store (SQLite + AES-256-GCM), embeddings
  └─ internal/crypto         Argon2id key derivation, AES-256-GCM encrypt/decrypt

Layer 3 — Connectors
  ├─ internal/connector      Telegram bot adapter
  └─ internal/ui             TUI dashboard (bubbletea), event bus

Layer 4 — Support
  ├─ internal/config         YAML config load/save
  └─ internal/metrics        Request counting, latency tracking
```

## Package Layout

| Package | Responsibility |
|---------|---------------|
| `cmd/ravensync` | Binary entry point |
| `internal/cli` | All cobra commands and TUI theming |
| `internal/agent` | RAG pipeline: embed → search → context → LLM → store |
| `internal/llm` | `Provider` interface, OpenAI-compatible client |
| `internal/memory` | `Store` (SQLite + encryption), `Embedder` interface, cosine search |
| `internal/crypto` | `Encrypt`, `Decrypt`, `DeriveKey`, `GenerateSalt` |
| `internal/connector` | Platform adapters (Telegram) |
| `internal/ui` | Bubbletea TUI dashboard, `EventBus` for real-time events |
| `internal/config` | `Config` struct, YAML serialization |
| `internal/metrics` | `Collector` for request stats |

## Where Does My Feature Go?

| Question | Package |
|----------|---------|
| Does it add/change a CLI command or TUI element? | `internal/cli` or `internal/ui` |
| Does it change how the agent processes messages? | `internal/agent` |
| Does it add a new LLM provider? | `internal/llm` |
| Does it change memory storage or search? | `internal/memory` |
| Does it change encryption? | `internal/crypto` |
| Does it add a new chat platform? | `internal/connector` |
| Does it change config fields? | `internal/config` |
| **If none of these fit, challenge whether the feature should exist.** | |

## Data Flow

```
User (Console / Telegram)
  → Connector receives message
  → Agent.HandleMessage / HandleMessageStream (text + optional images)
    → Embed user message (Embedder)
    → Search memory store (cosine similarity, top-K)
    → Build LLM context: system prompt + memories + history + user turn
    → Call LLM provider (Complete or stream; vision turns use Complete)
    → Store conversation as encrypted memory
    → Return response
  → Connector sends reply
```

## Development

```sh
make build       # build binary to bin/
make install     # install to GOPATH/bin
make test        # run all tests
make lint        # golangci-lint via `go run` (pinned version in Makefile)
```

Optional [pre-commit](https://pre-commit.com): `pip install pre-commit && pre-commit install` — runs `golangci-lint` on commit (see `.pre-commit-config.yaml`).

## Guidelines

- All packages live under `internal/` — nothing is exported as a library.
- The agent never does I/O directly — it goes through the store, embedder, and LLM interfaces.
- Connectors never touch the memory store directly — they call the agent.
- Keep the `crypto` package minimal and auditable.
- Tests go in `*_test.go` files next to the code they test.

## Code Style

- Run `gofmt` / `goimports` before committing.
- No unnecessary comments — code should be self-documenting.
- Error messages start lowercase, no trailing punctuation.
