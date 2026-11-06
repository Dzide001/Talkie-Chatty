# AI A session and retry checkpoint

Date: November 6, 2026. Branch: ai-a/phase0-contract, building on b629b62. PR #2 remains draft. Issue #1 and PR #2 were checked before work and again before handoff. No new AI B feedback after AI A comment 6697604285 was present; PR #2 had no comments/reviews. Contract 0.1-draft.2 remains unchanged and awaits explicit review of the initial lease-proof step and lifecycle/restore mappings.

Independent backend work completed:
- In-memory room/member authentication, separate admin authority, random credentials/epochs, invitation rotation/expiry, session expiry, removal and room closure.
- Four-member and configurable room caps; platform/capability validation.
- Connection generations that reject stale socket callbacks; restart invalidates credentials.
- Private APNs registration, disabled by default, with stale-result-safe token invalidation and online/wakeable/unreachable snapshots.
- Bounded per-session request ledger: one execution for concurrent identical requests, prior-result retries, changed-content rejection, no history eviction, and explicit full-capacity rejection.

Validation: Go 1.27.3 darwin/arm64. From services/control:

    env GOCACHE=/private/tmp/talkie-go-cache /opt/homebrew/bin/go test -race -count=1 ./...
    env GOCACHE=/private/tmp/talkie-go-cache /opt/homebrew/bin/go vet ./...

Both passed. Tests cover credential role/cross-room isolation, expiry and restart, concurrent membership capacity, replacement sockets, snapshot/result isolation, wake registration validation and stale token invalidation, removed membership, entropy failure/collision, concurrent duplicate requests, response bounds and full retry capacity. Existing floor/APNs tests also passed. No shared schema/model files changed, so Dart/schema checks were not rerun for this internal-only increment.

Integration limitations: no executable HTTP/WSS service or endpoint yet. The session store, floor, retry ledger and APNs transport are not wired together. The adapter must serialize room operations and checks, canonicalize validated JSON for semantic retries, and agree on replay-capacity/pending behavior before clients consume it. Internal ledger bounds are not a new negotiated wire policy. Live APNs signing/secrets, signaling relay, timing harness, mobile pairing and physical audio remain incomplete. Apple provisioning/iPhone and actual device gates remain outstanding. This checkpoint does not pass phase zero.
