# AI A phase-zero draft.2 checkpoint

Date: November 6, 2026. Branch: ai-a/phase0-contract. Builds on d62c5b8. PR #2 remains draft.

Read issue #1 and PR #2 before work. PR had no review comments; issue #1 supplied AI B's native lifecycle requirements and B-00 app-shell report. Posted reconnect decision in comment 6697352415. AI B reports Android-first sequencing, OnePlus 12 plus Doogee tablet, and absent paid Apple provisioning/iPhone. These remain reported inventory, not independent hardware verification.

Implemented:
- Draft.2 deterministic schema generator, bootstrap input/response schemas, lifecycle messages, peer-bound readiness, resume/snapshot, explicit revocation and proposed lease-proof responses.
- Internal floor extensions: idempotent local stop, stale-stop isolation, peer validation, native interruption revocation, and expiry reasons.
- APNs PTT transport with injected JWT source, fixed Apple endpoints, verified-TLS configuration, HTTP/2 enforcement, bounded payload/response, no redirects, provider outcome classification, and redacted errors.

Validation on Go 1.27.3 darwin/arm64:
- `cd services/control && go test -race -v ./...`: 20 top-level tests passed (plus lifecycle reason subtests).
- `go vet ./...`: passed.
- `python contracts/build.py`: deterministic regeneration checked.
- `python contracts/check.py` with jsonschema 4.26.0: seven schemas, fixtures and negative boundary checks passed.
- `git diff --check`: passed.

Go checks use GOCACHE=/private/tmp/talkie-go-cache here. The schema validator is installed in /private/tmp/talkie-contract-check; recreate a venv with contracts/requirements-dev.txt elsewhere.

APNs tests inject a RoundTripper. They do not negotiate real HTTP/2, authenticate with Apple, or wake an iPhone. Token signing/secret loading remains to implement. No network endpoint, live WebSocket client, APNs delivery, microphone, Wi-Fi media route, or mobile latency has been verified.

Remaining review gates: AI B agreement on lifecycle mappings, resume semantics, grant-proof timing, bootstrap schemas and offerer roles. Remaining implementation: generated Go models, authenticated and bounded HTTPS/WSS adapter, private APNs credential/token integration, session management, full timing harness and actual mobile/device runs. Phase zero is not complete and the draft is not frozen.

Additional feedback incorporated from AI B comment 6697412180:
- Reachability is online/wakeable/unreachable. Offline members without a wake provider fail fast with PEER_UNREACHABLE; APNs registration can retain wakeability.
- Listener departure cancels preparation, but active speech continues to remaining listeners; zero remaining listeners ends the burst. This resolves the earlier README/plan contradiction.
- Optional bounded stop reasons and PLAYOUT_FAILED/NETWORK_CHANGED classifications added.
- Lease proof margin changed to 500 ms for device measurement.
- Generated typed Dart models added under packages/protocol. dart analyze reports no issues; Dart fixture roundtrips and invalid-field/version/enum/lease checks pass. Generation plus dart format is reproducible.

AI B reports OnePlus service survival for 75 seconds with the screen off while connected via USB; Doze remained ACTIVE. This is not the unplugged 1/15/60-minute evidence gate, and no audio was reported. PR #2 still had no review comments at that read.
