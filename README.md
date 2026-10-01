# Rift

Privacy-first webhook tunnel. Receive webhooks on localhost without third-party services inspecting your traffic.

```
[Webhook Provider] → HTTPS → [Rift Relay] → Encrypted WebSocket → [rift CLI → localhost]
```

The relay server is **zero-knowledge** — it forwards encrypted bytes only. Even if the relay is compromised, your webhook payloads remain private (E2E encrypted with X25519 + ChaCha20-Poly1305).

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

Point your webhook provider (Stripe, GitHub, Midtrans, etc.) to the public URL. All requests are forwarded to your local server.

## Install

### From source

```bash
git clone https://github.com/radityajayantara/rift.git
cd rift
make build
# Binary at ./bin/rift
```

### Pre-built binaries

Download from [GitHub Releases](https://github.com/radityajayantara/rift/releases). Single binary, no dependencies.

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

Open `http://localhost:4040` while `rift listen` is running. The inspect UI shows:

- All incoming webhook requests in real-time
- Request/response headers and body (pretty-printed JSON)
- Status codes and timing
- Copy button for payloads
- Request metadata (ID, tunnel, timestamp)

The UI updates live via WebSocket — no need to refresh.

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

### Security model

1. CLI generates an **ephemeral X25519 keypair** per session
2. Session key derived via **HKDF-SHA256**
3. All traffic encrypted with **XChaCha20-Poly1305** (AEAD)
4. Relay is **zero-knowledge** — cannot decrypt, does not store, does not log
5. **Forward secrecy** — new keys every session

### Why not P2P?

Webhook providers (Stripe, GitHub, etc.) need a public URL to send HTTP POST requests. They won't install a P2P client. A thin relay is the minimal component needed to bridge the gap, while E2E encryption ensures it can't read your data.

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

## Project structure

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

## Tech stack

| Component | Choice | Why |
|---|---|---|
| Language | Go | Single binary, easy cross-compile |
| Tunnel protocol | WebSocket | Firewall-friendly, HTTP/443 |
| Encryption | X25519 + XChaCha20-Poly1305 | Forward secrecy, fast without HW AES |
| Storage | SQLite via `modernc.org/sqlite` | Pure Go, CGO_ENABLED=0 |
| CLI | `spf13/cobra` | Standard Go CLI framework |
| WebSocket | `nhooyr.io/websocket` | Modern, context-aware |

## License

MIT
