# Desktop foundation implementation plan

> **For agentic workers:** Use superpowers:subagent-driven-development for the independently reviewable core task; integrate the desktop shell sequentially.

**Goal:** Deliver the first runnable slice: persistent device configuration, project-scoped fixed rules and rule sets, actual HTTP forwarding/Mock and session-only traffic in the approved four-module UI.

**Architecture:** A Go application service owns validated configuration and immutable request snapshots. SQLite persists configuration only. A bounded in-memory recorder owns traffic. Wails binds the same service to Vue; UI browsing state never changes device bindings.

**Tech Stack:** Go, Wails v2, Vue 3 + TypeScript + Vite, SQLite (pure Go driver).

## Global constraints

- D20: project-wide request/response Headers; unbound IP never rewritten; interface Headers override last.
- D22/D23: devices and sets many-to-many within one project; one active set per device; controls only on device page.
- D07: restore configuration, never start listener automatically; sequential execution never automatically resumes.
- D09/D13: traffic memory only across app lifetime; export only in directory UI, no import.
- Existing approved prototype remains a reference. No fixture traffic or fake success in the application.
- This slice covers fixed priority rules and HTTP. HTTPS interception, sequential runs, random generation, samples and export remain explicit subsequent milestones, not falsely completed controls.

### Task 1: Configuration and HTTP core

Files: `go.mod`, `internal/core/{model,store,service,proxy}.go`, corresponding `*_test.go`.

Interface: `Open(path string) (*Service,error)`; `Close() error`; `Snapshot() Snapshot`; `SaveConfig(Config, expectedRevision int64) error`; `StartProxy(address string) error`; `StopProxy() error`; `Flows() []Flow`; `ClearFlows()`.

JSON lower camelCase. Config: revision, projects (id/name/requestHeaders/responseHeaders), devices (ip/label/projectId/linkedRuleSetIds/activeRuleSetId/lastSelectedRuleSetId), rules (id/projectId/name/method/url/status/body/headers/enabled), ruleSets (id/projectId/name/ruleIds). Snapshot includes config and proxyAddress. Flow includes id/ip/method/url/status/source/start/duration/requestHeaders/responseHeaders/requestBody/responseBody/error.

- [x] Write tests proving invalid/cross-project activation is rejected, shared set activation is independent, stale revision rejected, restart configuration restored with no listener or traffic.
- [x] Observe failing test before implementation.
- [x] Implement atomic SQLite persistence and validation; traffic never persisted.
- [x] Integration test forwarding via httptest, fixed first matching rule, project Headers and unbound pass-through, bounded traffic, failed listener visible.
- [x] Implement snapshot-before-I/O routing, body limits, timeouts, hop-by-hop Header removal and no environment proxy recursion; CONNECT explicitly reports unsupported in this milestone.
- [x] Run Go tests and vet; record limitations.

### Task 2: Desktop and approved UI integration

Files: `main.go`, `app.go`, `wails.json`, `frontend/{package.json,index.html,src/*}`, `README.md`.

- [x] Create Wails application bindings around core, store DB in OS user config directory, close on exit.
- [x] Port approved visual layout into real Vue views; create/edit projects, fixed rules and ordered rule sets, bind devices and enable/stop one set.
- [x] Show real flow data, all/per-IP filtering, list and host/path directory; explicit loading/error/empty states.
- [x] Preserve drafts on save failures and serialize mutations with revision feedback; poll runtime without overwriting editing drafts.
- [x] Build frontend and Windows desktop executable. Verify browser UI with an explicit test adapter only; production contains no fake fixtures.

### Task 3: Review and handoff

- [x] Independent spec/quality review of core, fix material findings.
- [x] Run core tests/vet, frontend typecheck/build, desktop compilation.
- [x] Document implemented/remaining scope and manual desktop/phone acceptance steps. Update stage status with evidence; ask only material next-stage decisions.

