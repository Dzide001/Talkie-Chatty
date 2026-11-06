# Protocol v0.1 review draft

Status: **0.1-draft.1; not frozen; no network endpoints implemented yet.** AI B must review the schemas and decisions before a production client/server adapter is implemented. Internal floor code is transport-independent and is not this wire protocol.

## Proposed transport and bootstrap

Use HTTPS for room creation/join and wake registration, and WSS for events. Proposed paths: POST /v1/rooms, POST /v1/rooms/{roomId}/join, PUT /v1/rooms/{roomId}/wake-registration, GET /v1/rooms/{roomId}/events (WebSocket upgrade). These are review proposals, not a running service.

Join input is join.schema.json. Creation will reuse device metadata without an invite; its exact response schema and room lifecycle schema remain to be supplied. Proposed responses issue opaque roomId, roomEpoch, participantId, a short-lived member bearer credential, and (only to creator) distinct admin credentials and an invitation. Credentials require crypto/rand and secure storage. The service authenticates every HTTP request and WebSocket upgrade using Authorization: Bearer; never put credentials in URL query parameters. Sender identity is derived from that session, not from client event fields. Credentials are not part of peer roster events.

This initial review covers client burst/signaling messages, join input, wake registration and server floor notifications. It does not yet define all roster, creation, renewal, resume, or server signaling schemas. Those and generated Go/Dart models follow the initial review; the schema folder must not be presented as a complete SDK.

## Client events

client-event.schema.json forbids unknown fields and binds each event to protocolVersion, roomId, roomEpoch and requestId. Burst-related messages also require burstId. The server rejects any room/epoch inconsistent with authentication. requestId provides correlation and bounded retry deduplication; reuse with changed content must fail. An adapter must bound the replay cache and reject stale events, rather than merely repeating an action.

- floor.request reserves one turn and snapshots all other joined members, including suspended listeners.
- wakeAcked acknowledges receipt of the wake request only. It can never issue a grant.
- audioReady asserts OS audio activation, connected DTLS/peer transport, attached remote track, and initialized playout for this burst. It is a client claim, not independent proof of playback.
- audioUnavailable cancels preparation/transmission conservatively in the initial implementation. The sender sees failure; it must not silently omit the listener.
- floor.cancel cancels preparation without a grantId. floor.release ends an active grant; floor.renew must match its authenticated speaker and grantId.
- signal.offer/answer/ice can target only an intended member of this burst. The speaker offers to each recipient in phase zero. Relayed server events must stamp authenticated sender identity. Buffer early ICE until the matching remote description exists. Empty candidate denotes end-of-candidates. No arbitrary peer-address answer channel is implied.

Proposed total serialized message bound: 48 KiB UTF-8; SDP 32,768 characters within that byte limit. The adapter also requires limits on requests, participants, pending events, credentials, and sessions. Schema validation alone is not sufficient authorization or resource protection.

## Floor lifecycle

Preparation expires after five seconds and is not renewed. At least one recipient is required. New joins wait until the next burst. Leave/removal of a current recipient cancels the burst; a new attempt takes a fresh snapshot. This conservative initial rule also revokes active transmission on such departure.

Once all snapshot recipients are ready, issue a three-second active lease, renewed each second, bounded by 30 seconds total. Duplicate readiness does not extend a grant. Disconnect of the speaker revokes its turn. Disconnect of an idle listener does not remove membership. The adapter schedules expiry notifications and invalidates old epochs on restart.

Client timers must conservatively account for message delay; receiving a lease must not blindly reset a full lease duration. Final clock/delay-margin semantics and renewal acknowledgment schema are explicitly review items. Local release/route loss/interruption stops capture immediately even if no control message gets through. Receivers enforce authorized speaker/epoch/burst as well as the sender's gate.

## Wake registration

wake-registration.schema.json accepts APNs PTT (channel UUID, environment, variable-length hex token) or none. An APNs token comes from the joined native PTT channel. Environment must match deployment configuration; do not accept a sandbox/production override blindly. Registration is private and authenticated; token invalidation/leave removes it. Provider success is not a delivery/readiness acknowledgment.

FCM is a reserved future provider, deliberately rejected with WAKE_PROVIDER_UNSUPPORTED until selected and implemented. AI B should confirm this shape instead of adding an ambiguous nullable token on join. Android baseline is the explicitly joined foreground service, subject to physical Doze/OEM verification. No push-triggered unsolicited microphone capture.

APNs baseline payload carries correlation/channel/speaker metadata only. SDP-in-push is an optional benchmark after baseline; no raw media keys. Any payload variant requires measured serialized byte limits, authenticated binding and fallback. WebRTC transport remains selected.

## Review requested from AI B

1. Confirm platform/appVersion/capabilities fields and separate wakeAcked/audioReady events.
2. Confirm provider-specific wake registration and unsupported FCM response.
3. Confirm speaker-offerer signaling and which native interruption events map to audioUnavailable.
4. Review cancellation and all-recipient policy, including membership changes during a burst.
5. Agree conservative lease timing and native send/playout gating before freezing the adapter contract.

Example fixtures use deliberately non-production identifiers and contain no secrets.
