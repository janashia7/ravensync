# Ravensync

[![CI](https://github.com/janashia7/ravensync/actions/workflows/ci.yml/badge.svg)](https://github.com/janashia7/ravensync/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/janashia7/ravensync)](https://goreportcard.com/report/github.com/janashia7/ravensync)
[![License: MIT](https://img.shields.io/badge/License-MIT-crimson.svg)](LICENSE)

Privacy-first, cross-platform AI memory. A unified, always-on memory layer for your personal AI assistant that works across Telegram, local console, and more — with zero knowledge of your data.

## Features

- **Unified memory** — conversations and context stored once, shared across all connectors
- **End-to-end encryption** — AES-256-GCM with Argon2id key derivation; the server never sees plaintext
- **Telegram integration** — connect a bot and chat with persistent memory
- **Local console** — interactive TUI dashboard with real-time chat
- **Bring your own LLM** — OpenAI, Google Gemini, Anthropic, Ollama (free, local), or any OpenAI-compatible endpoint
- **RAG-powered recall** — embedding-based memory search for contextual, relevant answers
- **Conversation history** — short-term context for natural back-and-forth dialogue
- **Self-hosted** — run on your own hardware, keep full control
- **Cross-platform** — macOS, Linux, Windows, Docker

## Installation

### Quick install (macOS / Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/janashia7/ravensync/main/install.sh | sh
```

This auto-detects your OS/arch, downloads the latest release binary, and installs to `/usr/local/bin`.

### Homebrew (macOS)

```sh
brew install ravensync/tap/ravensync
```

### Go install

Requires Go 1.26+:

```sh
go install github.com/janashia7/ravensync/cmd/ravensync@latest
```

### Download binary

Grab the latest release from [GitHub Releases](https://github.com/janashia7/ravensync/releases):

| Platform        | File                                |
|-----------------|-------------------------------------|
| macOS (Apple)   | `ravensync_darwin_arm64.tar.gz`     |
| macOS (Intel)   | `ravensync_darwin_amd64.tar.gz`     |
| Linux (x86_64)  | `ravensync_linux_amd64.tar.gz`      |
| Linux (ARM64)   | `ravensync_linux_arm64.tar.gz`      |
| Windows (x86_64)| `ravensync_windows_amd64.zip`       |
| Windows (ARM64) | `ravensync_windows_arm64.zip`       |

Extract and move to your `PATH`:

```sh
# macOS / Linux
tar xzf ravensync_*.tar.gz
sudo mv ravensync /usr/local/bin/

# Windows (PowerShell)
Expand-Archive ravensync_windows_amd64.zip -DestinationPath .
Move-Item ravensync_windows_amd64.exe C:\Windows\System32\ravensync.exe
```

### Build from source

```sh
git clone https://github.com/janashia7/ravensync.git
cd ravensync
make build      # binary in bin/
make install    # installs to GOPATH/bin
```

### Docker

```sh
docker pull ghcr.io/janashia7/ravensync:latest
```

Or build locally:

```sh
docker build -t ravensync .
```

Run with Docker:

```sh
docker run -it --rm \
  -e RAVENSYNC_PASSWORD="your-encryption-password" \
  -e RAVENSYNC_TELEGRAM_TOKEN="your-bot-token" \
  -e RAVENSYNC_LLM_KEY="sk-..." \
  -v ravensync-data:/root/.ravensync \
  ghcr.io/janashia7/ravensync:latest
```

Or use Docker Compose:

```sh
# Copy .env.example to .env and fill in your values
cp .env.example .env

docker compose up -d
```

#### Using Ollama with Docker

If you run Ollama on the host machine, uncomment the `extra_hosts` line in `docker-compose.yml` and set the LLM endpoint to `http://host.docker.internal:11434`.

## Getting Started

### 1. Initialize

```sh
ravensync init
```

The interactive setup wizard walks you through:
- Setting an encryption password (derives AES-256 key via Argon2id)
- Choosing your LLM provider (OpenAI, Gemini, Ollama, Anthropic, custom)
- Entering API credentials
- Selecting chat and embedding models
- Optionally configuring Telegram

Config is saved to `~/.ravensync/config.yaml`.

### 2. Start the agent

```sh
ravensync serve
```

Opens the live TUI dashboard with:
- Real-time chat panel (local console)
- Event log showing memory operations
- Simultaneous Telegram bot (if configured)
- Keyboard shortcuts: `Tab` to switch focus, `Ctrl+C` to quit

### 3. Check your setup

```sh
ravensync doctor
```

Validates config file, LLM connectivity, Telegram token, database access, and encryption.

## LLM Providers

| Provider | Model examples | Embedding model | Cost |
|----------|---------------|-----------------|------|
| **OpenAI** | `gpt-4o-mini`, `gpt-4o` | `text-embedding-3-small` | Paid API |
| **Google Gemini** | `gemini-2.0-flash` | `text-embedding-004` | Free tier available |
| **Ollama** | `llama3`, `mistral`, `phi3` | `nomic-embed-text` | Free (local) |
| **Anthropic** | `claude-3-haiku-20240307` | Uses OpenAI embeddings | Paid API |
| **Custom** | Any OpenAI-compatible endpoint | Configurable | Varies |

### Ollama setup

1. Install Ollama: https://ollama.com
2. Pull models:
   ```sh
   ollama pull llama3
   ollama pull nomic-embed-text
   ```
3. Run `ravensync init` and select "Ollama (local, free)"

## CLI Commands

| Command | Description |
|---------|-------------|
| `ravensync` | Show help with available commands |
| `ravensync init` | Interactive setup wizard |
| `ravensync serve` | Start the agent with TUI dashboard |
| `ravensync doctor` | Validate config and dependencies |
| `ravensync stats` | Show memory and usage statistics |
| `ravensync version` | Print version info |

## Configuration

Config file: `~/.ravensync/config.yaml`

| Key | Env Variable | Description |
|-----|-------------|-------------|
| `llm_provider` | — | `openai`, `gemini`, `ollama`, `anthropic`, `custom` |
| `llm_api_key` | `RAVENSYNC_LLM_KEY` | API key (not needed for Ollama) |
| `llm_model` | — | Chat model name |
| `embedding_model` | — | Embedding model name |
| `llm_api_base` | — | Custom API endpoint URL |
| `telegram_token` | `RAVENSYNC_TELEGRAM_TOKEN` | Telegram bot token |
| — | `RAVENSYNC_PASSWORD` | Encryption password |

## How It Works

```
  You (Console / Telegram)
           |
           v
  +------------------+
  |  Connector        |  (Telegram Bot API / Local TUI)
  +--------+---------+
           |
           v
  +--------+---------+
  |  Agent            |  (RAG loop + conversation history)
  +--+--------+------+
     |        |
     v        v
  Memory    LLM Provider
  Store     (OpenAI / Gemini / Ollama / ...)
  (SQLite + AES-256-GCM)
```

1. You send a message via console or Telegram
2. The agent embeds the message and retrieves relevant past memories (RAG)
3. It builds a context-augmented prompt with conversation history and memories
4. The response is sent back and the conversation is stored as encrypted memory
5. All memory is AES-256-GCM encrypted at rest; the key is derived from your password via Argon2id and never persisted

## Security

- Encryption key derived via **Argon2id** (time=3, mem=64MB, threads=4)
- Each memory item encrypted with **AES-256-GCM** using unique nonces
- Config file stored with `0600` permissions; data directory with `0700`
- No telemetry, no analytics, no data exfiltration
- Encryption password never stored on disk

## Development

```sh
# Run tests
make test

# Build for all platforms
make release-snapshot

# Build Docker image
make docker
```

## License

MIT
