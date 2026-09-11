# HTTPS and certificates implementation plan

> **For agentic workers:** Use superpowers:subagent-driven-development for the independent certificate component and final review; parent integrates proxy/UI sequentially.

**Goal:** Let the desktop proxy forward HTTPS tunnels and decrypt selected targets using a locally generated CA, reusing existing device/Mock/Header/traffic behavior.

**Architecture:** A standalone certificates package owns CA persistence, public metadata and bounded leaf cache. Core owns CONNECT connections and sends decrypted HTTP/1.1 requests through existing routing with original socket IP. Desktop exposes public certificate operations only. User trust installation is manual.

**Tech Stack:** Go standard crypto/tls/x509/net/http; Windows DPAPI for private-key-at-rest protection; existing Wails/Vue/SQLite.

## Constraints and decisions

- Existing approved D07/D20/D21/D22/D23 apply. Proxy still starts manually, config persists, traffic does not.
- Manual initial CA generation, idempotent if already present. No implicit replacement, no automatic OS trust installation, no private-key export.
- D24 confirmed by user: explicit domain/IP targets only. Unselected targets are byte tunnels and cannot apply Headers/Mock/body capture.
- Upstream TLS validation is mandatory. Client handshake failure must not silently fall back to a tunnel.
- Host/SNI/request authority must agree with the CONNECT target. Source IP remains original socket identity. Decrypted keep-alive connections route each request with a fresh config snapshot.
- Existing 30s request and 3s stop grace are provisional development limits. Hijacked connections must be tracked and closed on stop.
- First interception version negotiates HTTP/1.1. No HTTP/3/QUIC or certificate pinning bypass promised.

### Task 1: Local authority

Files: `internal/certificates/authority.go`, `protect_windows.go`, `protect_other.go`, `authority_test.go`.
API: `Open(dir string) (*Authority,error)`, `Info() Info`, `Generate() (Info,error)`, `PublicDER() ([]byte,error)`, `Certificate(host string) (*tls.Certificate,error)`.
Info JSON fields: available bool, subject string, fingerprint string (SHA-256 public DER), notBefore/notAfter time.Time. No key data.

- [x] Write and observe failing tests: absent, generate/reopen stable, repeated generate stable, DNS/IP SAN leaf verifies under CA, different hosts, invalid input, corrupt store does not overwrite.
- [x] Implement ECDSA P-256 CA and leaves, validity bounds and random serials, persistent protected key, bounded cache; enforce concurrency safety.
- [x] Validate Windows DPAPI encryption; non-Windows restrictive file permissions. Never save plaintext private key on Windows.
- [x] Test/vet and independent review.

### Task 2: CONNECT and interception

Files: `internal/core/https.go`, `https_test.go`, `proxy.go`, `service.go`, `model.go`.

- [x] First tests for raw TLS tunnel, trusted MITM fixed response, upstream verification failure, correct source IP, repeated HTTPS requests, stop cleanup, HTTP regressions.
- [x] Add persisted TLS settings to Config, strict host parsing, connection registry/lifecycle and explicit error flows.
- [x] Integrate CA through service methods; distinguish tunnel traffic from decrypted requests. Accept https URL rules.
- [x] Ensure buffered bytes after CONNECT are preserved and destination cannot be changed by inner Host or absolute URL.

### Task 3: Desktop UI and certificate download

Files: `app.go`, `main.go`, `frontend/src/{App.vue,api.ts,model.ts,CertificatePanel.vue}`, tests/docs.

- [x] Device page certificate status, fingerprint/expiry, generate action, native public certificate save dialog, platform guidance.
- [x] Expose confirmed decryption settings, separate from device rule activation and project browsing.
- [x] Full typecheck/build, browser smoke, Go integration tests, real desktop bridge smoke.
- [x] User handoff for iOS/Android trust and real App HTTPS validation; update implemented versus pending boundaries, commit/push.
