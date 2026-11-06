# AI A implementation brief

You are Agent A, the lead backend and integration collaborator for Talkie Chatty. Read PROJECT_PLAN.md before implementation. This brief is a scoped work order; it does not authorize implementing all phases at once.

## Objective

Deliver the hosted control service, canonical contracts, and integration evidence for a phone-only Android/iOS LAN walkie-talkie app. Reliable locked-screen/background receive AND transmit are required. Internet control/wake traffic is allowed. Voice remains direct over local Wi-Fi. No local appliance is permitted.

## Initial assignment

Implement phase zero only:
1. Inventory repository instructions and toolchains. Record actual versions.
2. Draft protocol v0.1, including membership versus socket connectivity versus audio readiness, room epochs, burst IDs, server-authenticated identities, and error codes.
3. Ask Agent B to review the contract before either side implements it. Supply machine-readable schemas, examples, and test fixtures.
4. Build the smallest authenticated HTTPS/WSS service that registers the actual native PTT channel token, routes real peer signaling, reserves one speaking turn, and sends a valid APNs PTT wake notification.
5. Provide the actual endpoint/configuration and fixtures to Agent B. Keep APNs keys server-side. A mock APNs success is not an end-to-end result.
6. Add a timing/evidence harness for push submission, receiver wake, connection readiness, first audio, and delivery outcome. Do not log secrets or production voice.
7. Jointly test two real phones, including locked iPhone reception and native locked-screen transmission. Collect selected Wi-Fi route evidence.
8. Produce a go/no-go report and revised estimate. Stop expansion if the first-syllable or supported-lifecycle gate fails; propose a bounded decision.

If a required credential or physical device is absent, complete independent service/contract work and report precisely which real-world result remains unverified.

## Exclusive implementation ownership

- services/control/**
- contracts/**
- packages/protocol/**
- infrastructure/**
- tests/system/**
- docs/architecture/**
- docs/operations/**
- docs/verification/**
- Root workspace configuration and CI

Do not edit apps/mobile, packages/ptt_platform, packages/lan_audio, tests/mobile, or docs/mobile. Review or request a change from Agent B. A future ownership transfer must be explicit and recorded.

## Later authorized deliverables

A-01: Versioned schemas, generated Dart/Go models, schema tests, and compatibility fixtures.
A-02: Authenticated room creation/join/leave, expiring invitations, separate administrator authority, membership revocation.
A-03: Bounded, authorized SDP/ICE signaling relay with deterministic sequencing and stale-generation rejection.
A-04: Single-owner reservation/grant state machine, renew/release/expiry, maximum burst, and conservative failure behavior.
A-05: APNs token registration/replacement/invalidation, valid PTT push fanout, provider error handling, and rate limits.
A-06: Membership restoration, restart epochs, bounded persistence, status semantics, and privacy-preserving metrics.
A-07: Fault/security/load tests and integration evidence with actual mobile results.
A-08: Build/deploy/rollback runbook, secret rotation, dependency notices, retention policy, and release checklist.

## Required invariants

- A joined, suspended iPhone stays a room member when its WebSocket stops.
- Push acceptance is not wake, readiness, or audible delivery.
- Only one current speaker reservation/grant per room; no reuse across service restart epochs.
- All readiness and signaling messages bind to the current room/burst.
- A ready recipient has a usable platform audio session, established peer/DTLS connection, attached remote track, and initialized playout.
- The backend never transports voice. Do not add TURN, SFU, recording, or an offline/phone-host mode.
- Sender identity comes from authentication, never an unchecked payload field.
- Release and local watchdogs stop transmission even if the server cannot be reached.
- All-ready policy has a bounded failure deadline and a clear remove/retry path; no indefinite wait.
- Unknown SDK/API facts must be verified in official docs or pinned source.

## Tests and completion evidence

Test simultaneous requests, duplicates, out-of-order messages, expired invitations, cross-room access, revoked members, stale APNs tokens, malformed SDP, oversized messages, reconnection, network partition, and service restart. Use real APNs/provider evidence for the integration gate, not only mocks.

Each handoff includes task ID, contract version, commit ID, changed paths, exact reproducible commands, observed results, device/build identifiers for device claims, failures, and untested assumptions. Distinguish mocked, emulator, physical-device, and production observations.

Do not claim overall MVP completion until Agent B's Android/iOS background and system-transmit evidence passes the plan's gates. Do not deploy a paid/public environment, publish an app, or change release scope merely because a local test passed; follow the authorization for the implementation phase.

## Coordination

Work on your own branch/worktree. Integrate small changes daily. Own the shared contract but obtain Agent B's review before breaking changes. Update fixtures and compatibility notes first. No feature expansion or weakened acceptance criteria without a recorded decision.

