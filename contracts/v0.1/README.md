# Protocol v0.1 review draft

Status: **0.1-draft.2; not frozen; no network endpoints implemented yet.** This revision addresses AI B's lifecycle review on issue #1. AI B must review the timing and restore behavior before consumers are implemented. Internal floor and APNs packages are independent components, not a running wire service.

Generate schemas with `python3 contracts/build.py`; validate with `python contracts/check.py` in a venv containing contracts/requirements-dev.txt. The generator is the schema source. This is a breaking draft revision; draft.1 consumers must not silently mix messages.

## Proposed HTTP and WSS transport

| Request | Input | Success response |
| --- | --- | --- |
| POST /v1/rooms | create.schema.json | 201 create-response.schema.json |
| POST /v1/rooms/{roomId}/join | join.schema.json | 200 join-response.schema.json |
| PUT /v1/rooms/{roomId}/wake-registration | wake-registration.schema.json | 204 |
| DELETE /v1/rooms/{roomId}/members/self | none | 204 |
| GET /v1/rooms/{roomId}/events | authenticated WebSocket upgrade | server-event.schema.json stream |

These are proposals, not deployed endpoints. Detailed admin operations, HTTP error/refresh semantics and generated Go models remain pending; this folder is not a complete SDK. A first server adapter is gated on review of this draft.

Creation issues independent random member/admin credentials and a random invitation. Join issues only a member credential. Proposed limits: four members, invitation lifetime 15 minutes, session lifetime at most 24 hours. No account or Firebase dependency. After credentials expire the user must rejoin; automatic credential refresh is outside this phase-zero draft.

Authenticate protected HTTP requests and WSS upgrades with Authorization: Bearer. Never put credentials in URLs or participant rosters. The service derives identity from the credential; payload senderId is forbidden on client messages. A roomId, epoch or capabilities list is never proof of authorization. Enforce lan-webrtc-v1 and the platform's selected background capability at join; unknown required capabilities fail explicitly.

## Events and membership

Every client event carries protocolVersion, roomId, roomEpoch, requestId and type. The adapter rejects room/epoch mismatches. Use a bounded per-session requestId deduplication cache; identical retry returns the prior result without repeating effects, changed content under the same ID fails REQUEST_ID_REUSED. Cross-reconnect handling requires this cache to outlive sockets. Losing the cache on service restart also invalidates credentials/epochs.

The coordinator assigns monotonically increasing room-wide serverSequence values. A client ignores earlier events, but gaps alone are not a failure because some messages are recipient-specific. Every new control connection begins with resume and a full state.snapshot; no replay log is promised.

Joined membership survives idle socket disconnection. Each roster includes reachability: online means an authenticated control socket, wakeable means offline with a valid APNs registration, unreachable means offline without one. Provider none cannot be wakeable. A snapshotted unreachable member rejects floor.request immediately with PEER_UNREACHABLE, without consuming a preparation timeout. Roster changes publish a roster event with the current floor snapshot; that snapshot never grants capture/playout authority. controlConnected in a roster is not audio readiness. Disconnecting the current speaker revokes its turn. New room members join the next burst. Departure/removal during preparation cancels that preparation. During active speech, disconnect that recipient and publish the updated roster/floor recipient list; remaining listeners continue. End with NO_RECIPIENTS if the last listener leaves. The all-ready policy applies at burst start. A departing speaker always ends the burst.

## Lifecycle decisions

| Client event | Server behavior |
| --- | --- |
| wakeAcked | Record receipt only; never grant speech. |
| audioReady | Validate burst/epoch and peerId equals that burst's speaker. Requires client-confirmed OS audio active, DTLS/peer connection established, track attached, and native playout initialized. |
| audioUnavailable | Revoke the current matching burst; report AUDIO_UNAVAILABLE. |
| audioInterrupted | Revoke affected current burst, report AUDIO_INTERRUPTED. Reason is FOCUS_LOST, CALL or ROUTE_LOST. |
| audioResumed | Acknowledge only. Never restore a grant or implicitly mark audioReady. |
| networkChanged | Revoke affected current burst, report NETWORK_CHANGED; client stops capture immediately. |
| permissionState | Denial revokes affected burst with PERMISSION_REVOKED; granting permission never restores speech. |
| localStop | Idempotent stop by the authenticated speaker; no grantId needed. Expired/old burst is acknowledged without affecting a newer turn. |
| resume | Same-epoch authentication returns current snapshot; never revives old media/grants. |

Interruption/network/permission events carry burstId, nullable if the client knows no active burst. For null, the adapter must inspect and conservatively revoke any current burst involving that authenticated participant. For a non-null stale burst, do not cancel a newer one. Process events in authenticated connection order and deduplicate request IDs. Malformed reasons are rejected, never logged as arbitrary free text.

Service startup/shutdown, notification taps and battery optimization diagnostics remain client-local. Android transmission requires a user action before floor.request. No push or server grant itself authorizes unsolicited microphone capture.

floor.revoked includes a precise reason: LOCAL_STOP, LEASE_EXPIRED, BURST_LIMIT, PREPARE_TIMEOUT, SPEAKER_DISCONNECTED, MEMBER_LEFT, MEMBER_REMOVED, AUDIO_INTERRUPTED, NETWORK_CHANGED, PERMISSION_REVOKED, AUDIO_UNAVAILABLE, or EPOCH_CHANGED, plus ordinary cancellation/release. All recipients immediately stop accepting that burst. Epoch replacement can also make delivery of the old revocation impossible; clients must stop on control loss/timeout without waiting for it.

## Reconnect and epoch mismatch

A valid member credential in the current epoch may resume using lastServerSequence. The server always returns a full snapshot, not missed events; a sequence ahead of the server is rejected. A snapshot is informational and never authorization to restart old capture. Any new transmission needs a new user action and grant.

An authenticated request carrying an obsolete epoch receives epochChanged, whose roomEpoch is the current epoch and previousEpoch is the rejected value. Clear old grants, peers and pending burst operations; re-authenticate/rejoin. No automatic reuse of an expired invite.

Phase-zero sessions are proposed as in-memory. On coordinator restart both credentials and epochs are invalidated. Such a credential receives HTTP 401 at upgrade, not an unauthenticated epoch disclosure. The UI returns to a safe disconnected/rejoin state. Durable restoration is later work; the old plan's persistence requirement is not implemented by this prototype.

## Signaling

The authenticated speaker is offerer to each snapshotted recipient. The recipient answers only that speaker; ICE can flow both ways along those edges. The adapter validates membership/burst/recipient and stamps senderId in relayed messages. A recipient cannot signal an unrelated member or forge the sender. Buffer early ICE until its matching description exists. Empty candidate denotes end-of-candidates; nullable sdpMid/sdpMLineIndex reflect wrapper output.

Bound total serialized messages to 48 KiB UTF-8, SDP to 32,768 characters within that byte limit, and candidates to 2,048 characters. Add request, connection, queue and session limits in the adapter. Schemas alone are not authorization or denial-of-service protection.

## Preparation and safe lease timing

A floor.request reserves at most five seconds, without renewal, and requires at least one recipient. All snapshot recipients must become ready. The resulting active lease is three seconds, renewed every second by the speaker, bounded by 30 seconds total. Duplicate readiness cannot extend it.

**Proposed timing rule for AI B review:** floor.granted announces a grant but is not sufficient to start capture/playout. The speaker immediately sends floor.renew and each receiver sends floor.sync. The response floor.lease echoes that requestId and contains remainingLeaseMs, computed when sending the response and capped at 3,000. Receivers use sync without extending the lease. Both sides refresh their proofs while participating, approximately once per second.

Record the local monotonic request-send time. Local expiry = request-send time + remainingLeaseMs - 500 ms safety margin. If that deadline has passed on receipt, fail closed and request a new proof; do not start audio. This conservatively includes the request and response travel delay without synchronized wall clocks. Do not anchor expiry at response receipt. Reject duplicate/out-of-order proofs using requestId and serverSequence. The initial preparation request is not the timing anchor, avoiding loss of the lease budget during a long wake.

The 500 ms margin is provisional for phase-zero measurements, not a proven scheduling bound. Monotonic clock behavior across suspension and native stop enforcement require device tests. Uncertain timing, interruption, route change, release, or control loss stops capture locally. Every listener also enforces the sender/epoch/burst and its own current lease proof.

## Wake provider

Provider-specific registration accepts apns-ptt (native channel UUID, environment, variable-length hex device token) or none. The deployment selects the APNs environment and rejects conflicting registration. Remove registrations on leave or provider invalidation; compare registration generation before deleting so an old response cannot erase a newer token.

No FCM field/dependency is included, matching the selected Android foreground-service baseline. New providers require negotiated capability/version support. Unsupported providers fail WAKE_PROVIDER_UNSUPPORTED. APNs submission success is not wakeAcked, audioReady, or delivery.

APNs baseline contains typed correlation metadata only. SDP-in-push remains a measured optional variant. No raw media keys or custom UDP transport. Source: https://developer.apple.com/documentation/pushtotalk/creating-a-push-to-talk-app

## Review requested from AI B

1. Confirm lifecycle-to-native mapping and peer-bound readiness.
2. Confirm same-epoch snapshot versus stale-epoch re-authentication behavior.
3. Confirm the separate floor.lease proof, monotonic request-send anchor and native enforcement feasibility; suggest exact changes if needed.
4. Confirm speaker-offerer signaling and recipient departure during preparation versus active speech.
5. Review bootstrap schemas and generated Dart models before the network adapter is implemented. Continue local shell/native work without assuming these endpoints are live.

Optional stop reason: floor.release, floor.cancel and localStop accept a bounded reason enum including USER_RELEASED. audioUnavailable also accepts NETWORK_CHANGED and PLAYOUT_FAILED. Focus loss/transient loss/call/route loss maps to AUDIO_INTERRUPTED; CAN_DUCK remains client-local per AI B. Native playout failure maps to PLAYOUT_FAILED; permission failures retain MIC_DENIED or LAN_DENIED. Stop reasons are diagnostic and never widen sender authority.

Typed Dart codecs are now supplied in packages/protocol, with fixture checks. They remain draft-version-specific and do not constitute a running signaling client.
