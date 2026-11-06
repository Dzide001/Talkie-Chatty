# Control service phase-zero foundation

This folder currently contains a transport-independent floor state machine, authenticated in-memory session store, bounded retry ledger, and injected-credential APNs PTT transport, not a deployable HTTP/WSS server. No live APNs request has been sent or verified. The client contract is under review in ../../contracts/v0.1.

Run with Go 1.24 or later:

    go test -race ./...
    go vet ./...

The core serializes room mutations, snapshots recipients including sleeping listeners, distinguishes wake acknowledgment from audio readiness, expires preparation/leases, caps speech duration, and rejects stale identities. IDs are correlation values, not authentication tokens.

The session store now issues and authenticates random credentials, assigns room epochs, limits room/member counts, expires invitations/sessions, and stores private wake registrations. The retry ledger detects changed-content request ID reuse and retains completed responses across reconnects when owned by the session. Neither package is wired to a network endpoint yet.

The eventual adapter must perform strict schema validation, translate internal types into explicit wire DTOs, authenticate every protected operation, rate-limit requests, coordinate deduplication, serialize room events, schedule deadlines, relay bounded signaling, send APNs, and publish state transitions. Process restart invalidates all in-memory credentials. Durable membership is outside this prototype. See internal/session/README.md for integration requirements.

No device, cloud, microphone, WebRTC, or APNs success is claimed by these tests.

Lifecycle additions: idempotent local stop, peer-bound readiness, interruption revocation, and bounded latest-end reason. The future network adapter must publish transitions in its serialized room loop; LastEnd is not an event queue. APNs tests use a simulated transport, documented in internal/apns/README.md.
