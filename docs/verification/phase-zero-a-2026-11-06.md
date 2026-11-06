# AI A phase-zero checkpoint

Date: November 6, 2026. Branch: ai-a/phase0-contract. Baseline: d753444.

Completed: reviewed issue #1; installed Go 1.27.3 (darwin/arm64); drafted partial protocol 0.1-draft.1 schemas/fixtures and explicit review questions; implemented transport-independent speaking-turn arbitration. No native/mobile files changed.

Observed checks:
- services/control: `go test -race -v ./...` — nine tests passed, including 100 concurrent requests with exactly one winner.
- services/control: `go vet ./...` — passed.
- `python contracts/check.py` using jsonschema 4.26.0 — four schemas plus readiness fixtures and negative boundary cases passed.
- `git diff --check` — passed.

Local Go checks used GOCACHE=/private/tmp/talkie-go-cache. Contract validation used a temporary Python virtual environment; reproduce by installing contracts/requirements-dev.txt into a venv. Go module has no third-party dependencies and declares Go 1.24 minimum; only Go 1.27.3 was tested here.

The tests exercise internal control logic, not a server endpoint. They do not establish authentication, real APNs delivery, WebRTC interoperability, OS behavior, battery performance, or latency. No physical-device result is claimed.

Pending: AI B review of fields, wake provider policy, signaling roles, cancellation semantics, and lease delay accounting; remaining room/server schemas and generated models; authenticated HTTPS/WSS adapter; APNs credentials/provider integration; end-to-end timing harness; device/provisioning access. Phase zero is started, not passed.

Reported physical-phone/provisioning blockers have not been independently re-inventoried. The redundant repo/ checkout remains untouched. This checkpoint introduces no paid deployment or cloud audio fallback.

Coordination update before handoff: AI B reports in issue comment 6697122213 that the user selected Android foreground service with no FCM for now, has one OnePlus 12, no iPhone, and a free Apple account. This draft already rejects FCM and keeps registration provider-specific. Treat OxygenOS idle behavior as a required later test. Android-first delivery has not been approved in this chat and iOS remains in scope but physically unverified. Do not substitute emulator pairing for two-phone evidence.
