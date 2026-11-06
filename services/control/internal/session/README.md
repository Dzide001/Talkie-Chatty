# Phase-zero session store

Internal Go APIs only; not new HTTP endpoints or a change to protocol 0.1-draft.2. The store implements the draft's four-member cap, 15-minute invitations, and maximum 24-hour room/member credentials. Default room cap is 128 and configurable. Expired rooms are pruned on operations; retained state is bounded even while idle.

Credentials and identifiers use 32 random bytes from crypto/rand. Authentication credentials and invitations are retained as SHA-256 digests. Admin and member authority are separate; departure/removal invalidates member authority without deleting the owner's separate admin credential. Room closure invalidates all credentials. Restart creates an empty store.

New membership starts disconnected. A current control connection is online; an offline registered APNs member is wakeable; an offline member with provider none is unreachable. APNs registration requires explicit backend enablement, matching environment, iOS platform, valid channel UUID and hex token. APNs remains disabled by default. A stale provider failure cannot invalidate a newer token registration. Private APNs tokens must be retained for submission, and are excluded from JSON along with issued credential fields; never log these structs or tokens. These internal types are deliberately not wire DTOs. The future adapter must explicitly serialize issued credentials only in authenticated bootstrap responses and never in a roster.

Each Connect replaces the member's socket generation. IsCurrent rejects superseded sockets; Disconnect ignores their delayed callbacks. The coordinator must serialize generation checks AND subsequent effects in the same room event loop. Calling IsCurrent then mutating the floor concurrently is not sufficient. Membership/wake updates and floor updates also need serialized coordination; these packages do not form a transaction by themselves.

# Retry ledger integration

internal/replay is one ledger per authenticated member session, retained across socket reconnects and dropped on revocation/expiry. Validate schema, identity and epoch before Begin. Only New authorizes executing an operation; Pending means the original is still in flight; Complete returns a copy of its prior result. Changed content under the same ID is rejected. Finish must store a bounded response even when the operation fails.

The ledger compares exact bytes. The future adapter must canonicalize validated JSON consistently if it accepts semantically identical retries with different JSON serialization. It must also define wire mappings for capacity, pending and invalid-result outcomes. Those choices are not frozen here.

There is no eviction or timeout that could accidentally repeat an effect. When full, the ledger rejects new IDs while still serving existing retries. Constructor limits bound per-ledger memory; the future adapter must select practical limits, impose a service-wide memory/admission budget, and define capacity recovery with AI B before exposing this behavior. Renew-heavy sessions make this a required integration decision, not an acceptable silent operational limit. No retry ledger is currently attached to the session store or floor coordinator.

Run all backend checks from services/control:

    go test -race -count=1 ./...
    go vet ./...

These are in-process tests, not HTTP authentication, real WebSocket, APNs delivery, or physical-device evidence.
