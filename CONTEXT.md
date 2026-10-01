# Rift — Project Context

## Apa Ini
Rift adalah CLI tool Go untuk menerima webhook di localhost tanpa layanan tunneling pihak ketiga yang merekam payload. Arsitekturnya: **thin relay server + E2E encrypted tunnel**. Relay hanya forward ciphertext — zero-knowledge terhadap isi traffic.

## Problem Statement
Developer yang mengintegrasikan webhook (Stripe, GitHub, Midtrans, dsb.) harus expose localhost ke internet. Solusi yang ada (ngrok, localtunnel) memiliki:
- Batasan request di tier gratis
- Traffic melewati server pusat pihak ketiga yang bisa inspect payload
- Tidak self-hostable atau setup-nya rumit

## Arsitektur

```
[Webhook Provider] → HTTPS → [Rift Relay] → WebSocket (E2E encrypted) → [Rift CLI di localhost]
```

### Komponen
1. **Rift CLI (`rift`)** — single binary, dijalankan developer di mesin lokal
   - `rift listen --to localhost:8080` — buat tunnel, dapat URL publik
   - `rift replay <id>` — kirim ulang request yang tersimpan
   - `rift relay` — jalankan relay server sendiri
2. **Rift Relay** — stateless server publik, forward encrypted bytes saja
3. **Local Inspect UI** — web UI di `localhost:4040`, embedded via `go:embed`

### Data Flow
1. CLI connect ke relay via WebSocket, dapat tunnel ID
2. Relay expose path `/t/<tunnel-id>` untuk menerima HTTP dari webhook provider
3. Request masuk → relay wrap sebagai message → kirim via WebSocket ke CLI
4. CLI decrypt → forward ke target localhost → tangkap response → encrypt → kirim balik
5. Relay forward response ke webhook provider
6. Request/response disimpan di local SQLite untuk inspect & replay

## Keputusan Teknis (ADR)

### ADR-001: WebSocket sebagai protokol tunnel
- **Konteks**: Butuh koneksi persistent antara CLI dan relay
- **Keputusan**: WebSocket over HTTPS (port 443)
- **Alasan**: Menembus firewall/proxy perusahaan, library Go mature (gorilla/websocket, nhooyr.io/websocket), built-in ping/pong keep-alive
- **Ditolak**: Raw TCP (terlalu banyak edge case), gRPC streaming (overhead untuk MVP)

### ADR-002: E2E Encryption — X25519 + ChaCha20-Poly1305
- **Konteks**: Relay tidak boleh bisa membaca payload webhook
- **Keputusan**: Ephemeral X25519 key exchange per session, ChaCha20-Poly1305 untuk symmetric encryption
- **Alasan**: Forward secrecy, ringan di CPU tanpa hardware AES, standar modern (dipakai WireGuard, TLS 1.3)
- **Implikasi**: Relay benar-benar blind — bahkan operator relay tidak bisa inspect traffic

### ADR-003: Path-based URL (/t/<tunnel-id>)
- **Konteks**: Butuh URL publik untuk webhook provider mengirim request
- **Keputusan**: `relay.example.com/t/abc123` (path-based)
- **Alasan**: Tidak butuh wildcard DNS/TLS, mudah diletakkan di belakang Caddy/Nginx, menurunkan barrier self-host
- **Upgrade path**: Subdomain-based sebagai opsi konfigurasi di v1.0+

### ADR-004: Local Web UI via go:embed
- **Konteks**: Developer perlu inspect webhook payload, headers, timing
- **Keputusan**: Static web UI (HTML/JS/CSS) di-embed ke binary, serve di localhost:4040
- **Alasan**: Single binary tetap terjaga, inspect JSON payload lebih enak di browser daripada terminal
- **Upgrade path**: TUI sebagai enhancement opsional

### ADR-005: SQLite via modernc.org/sqlite (pure Go)
- **Konteks**: Perlu storage untuk request/response (inspect + replay)
- **Keputusan**: SQLite dengan driver `modernc.org/sqlite` (pure Go, no CGO)
- **Alasan**: CGO_ENABLED=0 cross-compilation tetap bisa, queryable untuk filter/search di inspect UI
- **Ditolak**: `mattn/go-sqlite3` (butuh CGO + C toolchain), flat file JSON lines (tidak queryable)

### ADR-006: Single Binary Distribution
- **Konteks**: CLI tool harus mudah diinstall
- **Keputusan**: Single static binary, CGO_ENABLED=0, distribusi via GitHub Releases
- **Alasan**: Zero dependency untuk end user, Go cross-compilation sangat mulus

## Glosarium
| Istilah | Definisi |
|---|---|
| **Relay** | Server publik stateless yang mem-forward encrypted bytes antara webhook provider dan CLI |
| **Tunnel** | Koneksi WebSocket persistent antara CLI dan relay, dienkripsi E2E |
| **Tunnel ID** | Identifier unik untuk sebuah tunnel session, dipakai sebagai path di URL publik |
| **Inspect** | Fitur melihat request/response webhook yang masuk, via local web UI |
| **Replay** | Mengirim ulang request webhook yang tersimpan ke localhost target |

## Scope MVP
- [x] Definisi arsitektur
- [ ] CLI: `rift listen`, `rift relay`, `rift replay`
- [ ] E2E encryption (X25519 + ChaCha20-Poly1305)
- [ ] Relay server (WebSocket + HTTP handler)
- [ ] Local inspect web UI (embedded)
- [ ] SQLite storage untuk request/response
- [ ] GitHub Releases (multi-platform binary)
