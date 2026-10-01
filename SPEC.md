# Rift MVP Spec

## Overview
CLI tool Go yang membuat encrypted tunnel antara localhost dan relay server publik untuk menerima webhook secara privat.

## Commands

### `rift listen`
Buat tunnel ke relay dan forward webhook ke localhost.

```
rift listen --to localhost:8080 [--relay wss://relay.example.com] [--inspect :4040]
```

| Flag | Default | Deskripsi |
|---|---|---|
| `--to` | (required) | Target localhost address |
| `--relay` | `wss://relay.riftunnel.dev` | Relay server URL |
| `--inspect` | `:4040` | Port untuk local inspect web UI |
| `--no-inspect` | `false` | Disable inspect UI |

**Output saat connect:**
```
Rift tunnel aktif
  Public URL : https://relay.riftunnel.dev/t/a1b2c3d4
  Forwarding : → http://localhost:8080
  Inspect    : http://localhost:4040
  Encryption : E2E (X25519 + ChaCha20-Poly1305)
```

### `rift relay`
Jalankan relay server sendiri.

```
rift relay [--addr :8443] [--tls-cert cert.pem --tls-key key.pem]
```

| Flag | Default | Deskripsi |
|---|---|---|
| `--addr` | `:8443` | Listen address |
| `--tls-cert` | (none) | TLS certificate (opsional, bisa pakai reverse proxy) |
| `--tls-key` | (none) | TLS private key |

### `rift replay`
Kirim ulang request yang tersimpan.

```
rift replay <request-id> [--to localhost:8080]
```

## Arsitektur Internal

### Project Structure
```
rift/
├── cmd/
│   └── rift/
│       └── main.go              # CLI entrypoint (cobra)
├── internal/
│   ├── client/                  # rift listen logic
│   │   ├── tunnel.go            # WebSocket connection + reconnect
│   │   ├── forwarder.go         # Forward decrypted request ke localhost
│   │   └── inspect.go           # Local inspect server
│   ├── relay/                   # rift relay logic
│   │   ├── server.go            # HTTP server + WebSocket upgrade
│   │   ├── tunnel_registry.go   # Track active tunnels (in-memory)
│   │   └── handler.go           # Menerima webhook, route ke tunnel
│   ├── crypto/                  # E2E encryption
│   │   ├── handshake.go         # X25519 key exchange
│   │   └── stream.go            # ChaCha20-Poly1305 encrypt/decrypt
│   ├── protocol/                # Wire protocol (message framing)
│   │   └── message.go           # Request/Response message types
│   └── storage/                 # SQLite storage
│       ├── store.go             # CRUD operations
│       └── models.go            # Request/Response models
├── web/                         # Inspect UI assets (go:embed)
│   ├── index.html
│   ├── app.js
│   └── style.css
├── go.mod
├── go.sum
├── Makefile
├── CONTEXT.md
└── README.md
```

### Wire Protocol (WebSocket Messages)

Semua message di-wrap dalam envelope:

```go
type Envelope struct {
    Type    string // "handshake", "request", "response", "ping"
    Payload []byte // encrypted setelah handshake selesai
}
```

#### Handshake Flow
1. CLI → Relay: `{"type": "register"}`
2. Relay → CLI: `{"type": "registered", "tunnel_id": "abc123"}`
3. (E2E key exchange terjadi di layer terpisah — relay tidak tahu)

#### Request Flow
1. Webhook provider HTTP request → Relay `/t/abc123`
2. Relay wrap request → `Envelope{Type: "request", Payload: encrypted(HTTPRequest)}`
3. Relay kirim via WebSocket ke CLI
4. CLI decrypt → forward ke localhost → tangkap response
5. CLI → Relay: `Envelope{Type: "response", Payload: encrypted(HTTPResponse)}`
6. Relay forward HTTP response ke webhook provider

### SQLite Schema

```sql
CREATE TABLE requests (
    id          TEXT PRIMARY KEY,    -- UUID
    tunnel_id   TEXT NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    headers     TEXT NOT NULL,       -- JSON
    body        BLOB,
    status_code INTEGER,
    res_headers TEXT,                -- JSON
    res_body    BLOB,
    duration_ms INTEGER,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_requests_tunnel ON requests(tunnel_id);
CREATE INDEX idx_requests_created ON requests(created_at DESC);
```

### Crypto Flow Detail

```
CLI                              Relay                     (Relay cannot read)
 |                                 |
 |-- Generate ephemeral X25519 --→ |
 |   keypair (cli_pub, cli_priv)   |
 |                                 |
 |   (Public key dikirim via       |
 |    WebSocket saat handshake,    |
 |    tapi relay hanya forward)    |
 |                                 |
 |   Shared secret = X25519(       |
 |     cli_priv, peer_pub)         |
 |                                 |
 |   Session key = HKDF(shared)    |
 |                                 |
 |-- ChaCha20-Poly1305 encrypt --→ |  ← relay sees ciphertext only
 |   semua message setelah         |
 |   handshake                     |
```

> **Catatan untuk MVP**: Karena di MVP belum ada peer kedua (CLI-to-CLI), E2E encryption di-exercise sebagai encrypt-to-self — CLI encrypt sebelum kirim ke relay, relay forward blind, CLI di ujung lain decrypt. Ini memastikan relay tetap zero-knowledge dan arsitektur siap untuk multi-peer di masa depan.

## Dependensi Utama

| Package | Versi (pin) | Fungsi |
|---|---|---|
| `github.com/spf13/cobra` | latest | CLI framework |
| `nhooyr.io/websocket` | latest | WebSocket (modern, context-aware) |
| `modernc.org/sqlite` | latest | SQLite pure Go |
| `golang.org/x/crypto` | latest | X25519, ChaCha20-Poly1305, HKDF |
| `github.com/google/uuid` | latest | Tunnel ID generation |

## Non-Goals untuk MVP
- Managed relay publik (pengguna self-host sendiri dulu)
- Subdomain-based URL
- TUI inspect
- Authentication/authorization di relay
- Rate limiting
- Multi-region relay
- CLI auto-update
