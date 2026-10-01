# Rift

[![CI](https://github.com/radityajayantara/rift/actions/workflows/ci.yml/badge.svg)](https://github.com/radityajayantara/rift/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/radityajayantara/rift)](https://github.com/radityajayantara/rift/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/radityajayantara/rift)](https://goreportcard.com/report/github.com/radityajayantara/rift)

**Privacy-first webhook tunnel.** Receive webhooks on localhost without third-party services inspecting your traffic.

```
[Webhook Provider] → HTTPS → [Rift Relay] → Encrypted WebSocket → [rift CLI → localhost]
```

## Why Rift?

Every webhook tunnel tool routes your traffic through a central server. **Rift is different** — the relay is zero-knowledge. It forwards encrypted bytes only. Even if the relay is compromised, your webhook payloads remain private.

### Comparison

| | ngrok | localtunnel | bore | frp | **Rift** |
|---|---|---|---|---|---|
| **Relay sees payload** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes | **❌ No (E2E encrypted)** |
| **Self-hostable** | ❌ Paid | ⚠️ Complex | ✅ Yes | ✅ Yes | **✅ Single command** |
| **Inspect UI** | ✅ Yes | ❌ No | ❌ No | ✅ Dashboard | **✅ Built-in** |
| **Request replay** | ❌ Paid | ❌ No | ❌ No | ❌ No | **✅ Yes** |
| **Path filtering** | ❌ Paid | ❌ No | ❌ No | ✅ Config | **✅ CLI flag** |
| **Forward secrecy** | ❌ No | ❌ No | ❌ No | ❌ No | **✅ Per-session keys** |
| **Free request limit** | 20/min | Unlimited | Unlimited | Unlimited | **Unlimited** |
| **Single binary** | ✅ Yes | ❌ Node.js | ✅ Yes | ✅ Yes | **✅ Yes (16MB)** |

## 🔒 Security Model

This is Rift's core differentiator — not an afterthought.

1. CLI generates an **ephemeral X25519 keypair** every session — new keys, every time
2. Session key derived via **HKDF-SHA256**
3. All webhook payloads encrypted with **XChaCha20-Poly1305** (AEAD)
4. Relay is **zero-knowledge** — cannot decrypt, does not store, does not log
5. **Forward secrecy** — compromise one session, past sessions stay safe

```
You (CLI)                         Relay                    Stripe/GitHub
    │                               │                           │
    │◄──── Encrypted WebSocket ────►│◄──── HTTPS POST ─────────│
    │   (relay sees ciphertext      │   /t/abc123               │
    │    only — cannot read)        │                           │
    ▼                               │                           │
 localhost:8080                     │                           │
```

> **Even if someone owns the relay, they get nothing.** That's the point.

## Quick Start

```bash
# Terminal 1: Start relay (self-hosted)
rift relay

# Terminal 2: Start tunnel
rift listen --to localhost:8080 --relay ws://localhost:8443
```

Output:

```
  Rift tunnel aktif
  Public URL : http://localhost:8443/t/a1b2c3d4
  Forwarding : → http://localhost:8080
  Inspect    : http://localhost:4040
  Encryption : E2E (X25519 + ChaCha20-Poly1305)
```

```bash
# Terminal 3: Send a test webhook
curl -X POST http://localhost:8443/t/a1b2c3d4/webhook \
  -H "Content-Type: application/json" \
  -d '{"event":"payment.success","amount":50000}'
```

Point your webhook provider (Stripe, GitHub, Midtrans, etc.) to the public URL. All requests are forwarded to your local server.

## Install

### Pre-built binaries

Download from [GitHub Releases](https://github.com/radityajayantara/rift/releases). Single binary, no dependencies.

```bash
# macOS (Apple Silicon)
curl -L https://github.com/radityajayantara/rift/releases/latest/download/rift-darwin-arm64 -o rift
chmod +x rift
sudo mv rift /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/radityajayantara/rift/releases/latest/download/rift-linux-amd64 -o rift
chmod +x rift
sudo mv rift /usr/local/bin/
```

### From source

```bash
git clone https://github.com/radityajayantara/rift.git
cd rift
make build
# Binary at ./bin/rift
```

## Commands

### `rift listen`

Create a tunnel and forward webhooks to localhost.

```bash
rift listen --to localhost:8080
rift listen --to localhost:3000 --relay wss://relay.example.com
rift listen --to localhost:8080 --no-inspect
rift listen --to localhost:8080 --filter /webhook,/stripe
```

| Flag | Default | Description |
|---|---|---|
| `--to` | (required) | Target localhost address |
| `--relay` | `wss://relay.riftunnel.dev` | Relay server URL |
| `--inspect` | `:4040` | Local inspect UI address |
| `--no-inspect` | `false` | Disable inspect UI |
| `--filter` | (none) | Only forward requests matching these path patterns |

The tunnel auto-reconnects with exponential backoff if the connection drops.

### `rift relay`

Run a self-hosted relay server.

```bash
rift relay
rift relay --addr :8443
rift relay --addr :443 --tls-cert cert.pem --tls-key key.pem
```

| Flag | Default | Description |
|---|---|---|
| `--addr` | `:8443` | Listen address |
| `--tls-cert` | | TLS certificate file |
| `--tls-key` | | TLS private key file |

Behind a reverse proxy (recommended for production):

```nginx
# Nginx
server {
    listen 443 ssl;
    server_name relay.example.com;

    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

```
# Caddy
relay.example.com {
    reverse_proxy localhost:8443
}
```

### `rift replay`

Replay a stored webhook request to localhost.

```bash
rift replay a1b2c3d4
rift replay a1b2c3d4 --to localhost:3000
```

## Inspect UI

Open `http://localhost:4040` while `rift listen` is running.

<!-- TODO: Add screenshot here -->
<!-- ![Rift Inspect UI](docs/inspect-ui.png) -->

Features:
- All incoming webhook requests in real-time (live via WebSocket)
- Request/response headers and body (pretty-printed JSON)
- Status codes and response timing
- Copy button for payloads
- Request metadata (ID, tunnel, timestamp)

No need to refresh — the UI updates automatically.

## Self-hosting

Rift is designed for self-hosting. Run `rift relay` on any VPS:

```bash
# Minimal setup on a $5/month VPS
rift relay --addr :8443

# With TLS (or use Caddy/Nginx for automatic HTTPS)
rift relay --addr :443 --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

Then point your CLI:

```bash
rift listen --to localhost:8080 --relay wss://your-vps.example.com
```

### Why not P2P?

Webhook providers (Stripe, GitHub, etc.) need a public URL to send HTTP POST requests. They won't install a P2P client. A thin relay is the minimal component needed to bridge the gap, while E2E encryption ensures it can't read your data.

## Architecture

```
┌──────────────┐     HTTPS      ┌──────────────┐   Encrypted WS   ┌──────────────┐
│   Webhook    │ ──────────────→│  Rift Relay   │←────────────────→│  rift CLI    │
│   Provider   │     /t/<id>    │  (stateless)  │  E2E encrypted   │  (localhost) │
└──────────────┘                └──────────────┘                   └──────┬───────┘
                                 Sees only                                │
                                 ciphertext                               ▼
                                                                  ┌──────────────┐
                                                                  │ Local Server │
                                                                  │ :8080        │
                                                                  └──────────────┘
```

## Tech Stack

| Component | Choice | Why |
|---|---|---|
| Language | Go | Single binary, easy cross-compile |
| Tunnel protocol | WebSocket | Firewall-friendly, HTTP/443 |
| Encryption | X25519 + XChaCha20-Poly1305 | Forward secrecy, fast without HW AES |
| Storage | SQLite via `modernc.org/sqlite` | Pure Go, CGO_ENABLED=0 |
| CLI | `spf13/cobra` | Standard Go CLI framework |
| WebSocket | `nhooyr.io/websocket` | Modern, context-aware |

## Project Structure

```
rift/
├── cmd/rift/main.go              # CLI entrypoint
├── internal/
│   ├── cli/                      # Cobra commands
│   ├── client/                   # Tunnel, forwarder, inspect server
│   ├── crypto/                   # X25519 handshake + ChaCha20 stream
│   ├── protocol/                 # Wire format (JSON envelopes)
│   ├── relay/                    # HTTP server, WebSocket, tunnel registry
│   └── storage/                  # SQLite (pure Go, no CGO)
├── web/                          # Inspect UI (embedded via go:embed)
├── test/integration/             # E2E tests
└── Makefile
```

## Development

```bash
# Run tests
make test

# Build for current platform
make build

# Cross-compile all platforms
make build-all

# Run relay locally
make run-relay

# Run client locally
make run-listen
```

## Contributing

Contributions are welcome! Here's how:

1. **Fork** the repo
2. **Create a branch** — `git checkout -b feat/my-feature`
3. **Make changes** — ensure `make test` passes
4. **Commit** — use [conventional commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`)
5. **Push** and open a **Pull Request**

### Areas where help is appreciated

- 🌍 Managed public relay infrastructure
- 🔌 Protocol adapters (gRPC webhooks, GraphQL subscriptions)
- 🖥️ TUI inspect mode (terminal-only alternative to web UI)
- 📦 Package managers (Homebrew, apt, AUR)
- 📝 Documentation and examples for specific providers (Stripe, GitHub, etc.)

Please open an issue first for large changes so we can discuss the approach.

## License

[MIT](LICENSE)
