# Control service phase-zero foundation

This folder currently contains a transport-independent floor state machine, not a deployable HTTP/WSS server. No APNs request is sent. The client contract is under review in ../../contracts/v0.1.

Run with Go 1.24 or later:

    go test -race ./...
    go vet ./...

The core serializes room mutations, snapshots recipients including sleeping listeners, distinguishes wake acknowledgment from audio readiness, expires preparation/leases, caps speech duration, and rejects stale identities. IDs are correlation values, not authentication tokens.

The eventual adapter must authenticate identities, assign fresh unpredictable room epochs, issue credentials, validate/deduplicate/rate-limit messages, enforce capacity, schedule deadlines, persist membership as appropriate, relay bounded signaling, send APNs, and publish state transitions. Those pieces are not implemented or implied by the unit tests.

No device, cloud, microphone, WebRTC, or APNs success is claimed by these tests.
