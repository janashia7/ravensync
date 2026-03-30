# Ravensync

[![CI](https://github.com/janashia7/ravensync/actions/workflows/ci.yml/badge.svg)](https://github.com/janashia7/ravensync/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/janashia7/ravensync)](https://goreportcard.com/report/github.com/janashia7/ravensync)
[![License: MIT](https://img.shields.io/badge/License-MIT-crimson.svg)](LICENSE)

Encrypted local memory for a personal assistant: **Telegram** bot + **console TUI**, **RAG** over past turns, and an **OpenAI-compatible** LLM API (including **Ollama**).

## What it does

- **Memory** — SQLite with AES-256-GCM; key from your password (Argon2id). Per-user keys for Telegram (`tg:<id>`).
- **Recall** — Embeddings + cosine search; short conversation history for follow-ups. Recent photo turns keep pixels for context (bounded per request).
- **Telegram** — Optional allowlist (`allowed_users` / `allowed_usernames`), streaming replies, reply/caption context, text documents, **photos** (needs a **vision** model), slash commands, optional `[CHOOSE: …]` inline buttons.
- **Console** — Same agent via the dashboard (`ravensync serve`).

**LLM runtime:** `llm_provider: ollama` talks to `http://localhost:11434/v1`. Any other value uses the same stack with an API key against the **OpenAI-compatible** HTTP API (default host is OpenAI’s). Use a compatible proxy if your vendor is not OpenAI.

**Vision:** Use a model that accepts image inputs (e.g. LLaVA, `llama3.2-vision` on Ollama). Plain chat models will not see pixels.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/janashia7/ravensync/main/install.sh | sh
```

**Go** (requires Go **1.25+** per `go.mod`):

```sh
go install github.com/janashia7/ravensync/cmd/ravensync@latest
```

**Releases:** [GitHub Releases](https://github.com/janashia7/ravensync/releases) — archives for macOS, Linux, Windows.

**From source:** `git clone` → `make build` (binary in `bin/`) or `make install`.

**Docker:** `docker pull ghcr.io/janashia7/ravensync:latest` — see `docker-compose.yml` and `.env.example`. For Ollama on the host, use `host.docker.internal:11434` where relevant.

## Quick start

```sh
ravensync init    # password, LLM, Telegram, allowlist
ravensync doctor  # sanity-check config
ravensync serve   # TUI + Telegram
```

Config lives at `~/.ravensync/config.yaml`. After the first `init`, prefer **`ravensync config`** / **`config set`** to change models or tokens (avoids rotating the encryption salt and breaking `memory.db`).

**Allowlist (examples):**

```sh
ravensync init --allowed-users "123456789,@yourusername" --telegram-token "YOUR_TOKEN"
ravensync config allow-users "123456789,@yourusername"
```

Restart `serve` after config changes. Numeric ID from [@userinfobot](https://t.me/userinfobot); `@username` is case-insensitive (no `@` in YAML is fine).

## Configuration

| YAML key | Environment | Purpose |
|----------|-------------|---------|
| `data_dir` | — | Data directory (default `~/.ravensync`) |
| `encryption_salt` | — | Set by `init`; do not hand-edit |
| `llm_provider` | — | `ollama` or any other label (non-`ollama` → OpenAI-compatible client) |
| `llm_api_key` | `RAVENSYNC_LLM_KEY` | API key (omit for Ollama) |
| `llm_model` | — | Chat model name |
| `embedding_model` | — | Embedding model name |
| `telegram_token` | `RAVENSYNC_TELEGRAM_TOKEN` | Bot token |
| `allowed_users` | — | Optional Telegram user IDs |
| `allowed_usernames` | — | Optional usernames without `@` |
| — | `RAVENSYNC_PASSWORD` | Encryption password for `serve` |

## CLI

| Command | Purpose |
|---------|---------|
| `ravensync init` | First-time setup |
| `ravensync serve` | Run agent + TUI |
| `ravensync doctor` | Validate config and connectivity |
| `ravensync stats` | Memory / usage stats |
| `ravensync config` | Interactive menu |
| `ravensync config show` | Redacted config |
| `ravensync config set` | Wizard or `--flags` |
| `ravensync config allow-users` | Edit allowlist |
| `ravensync version` | Version |

## Telegram commands

| Command | Description |
|---------|-------------|
| `/start` | Intro |
| `/help` | Commands |
| `/stats` | Session / memory summary |
| `/memories` | Recent memory previews |
| `/forget` | Delete all memories for your Telegram user |

## Ollama (local)

```sh
ollama pull llama3
ollama pull nomic-embed-text
# For photos:
ollama pull llava   # or another vision model
```

Select Ollama in `init` and set `llm_model` / `embedding_model` to match.

## Architecture

```
Console / Telegram → connector → agent (embed → search → history + LLM) → memory store
```

## Security

- Argon2id key derivation; AES-256-GCM per record; config `0600`, data dir `0700`.
- No telemetry. Password is not stored on disk.

## Development

```sh
make test
make build
make lint   # uses golangci-lint via go run (pinned in Makefile); first run may download modules
```

## License

MIT
