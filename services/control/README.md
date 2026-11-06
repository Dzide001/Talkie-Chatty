# Control service phase-zero foundation

This folder currently contains a transport-independent floor state machine and an injected-credential APNs PTT transport, not a deployable HTTP/WSS server. No live APNs request has been sent or verified. The client contract is under review in ../../contracts/v0.1.

Run with Go 1.24 or later:

    go test -race ./...
    go vet ./...

The core serializes room mutations, snapshots recipients including sleeping listeners, distinguishes wake acknowledgment from audio readiness, expires preparation/leases, caps speech duration, and rejects stale identities. IDs are correlation values, not authentication tokens.

The eventual adapter must authenticate identities, assign fresh unpredictable room epochs, issue credentials, validate/deduplicate/rate-limit messages, enforce capacity, schedule deadlines, persist membership as appropriate, relay bounded signaling, send APNs, and publish state transitions. Those pieces are not implemented or implied by the unit tests.

No device, cloud, microphone, WebRTC, or APNs success is claimed by these tests.

Lifecycle additions: idempotent local stop, peer-bound readiness, interruption revocation, and bounded latest-end reason. The future network adapter must publish transitions in its serialized room loop; LastEnd is not an event queue. APNs tests use a simulated transport, documented in internal/apns/README.md.
