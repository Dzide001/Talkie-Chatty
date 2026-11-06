// Package floor implements the internal, transport-independent speaking-turn
// state machine. It is not a network API. Callers must authenticate member IDs,
// supply server time (never client timestamps), and schedule expiry processing.
package floor

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrMember          = errors.New("unknown member")
	ErrBusy            = errors.New("floor busy")
	ErrNoRecipients    = errors.New("no recipients")
	ErrStale           = errors.New("stale burst or grant")
	ErrNotRecipient    = errors.New("not a recipient")
	ErrPeerUnreachable = errors.New("peer unreachable")
)

const (
	Preparation  = 5 * time.Second
	Lease        = 3 * time.Second
	MaximumBurst = 30 * time.Second
)

type Snapshot struct {
	Epoch, Burst, Grant, Speaker, Phase string
	Recipients                          []string
	Deadline                            time.Time
}
type turn struct {
	Snapshot
	ready map[string]bool
	limit time.Time
}
type End struct {
	Epoch, Burst, Speaker, Reason string
}
type memberState struct{ online, canWake bool }
type Room struct {
	mu       sync.Mutex
	epoch    string
	sequence uint64
	members  map[string]*memberState
	current  *turn
	lastEnd  *End
}

// New requires a fresh, nonempty epoch on every coordinator restart. IDs are
// opaque correlation identifiers, not credentials.
func New(epoch string) *Room {
	if epoch == "" {
		panic("floor: empty epoch")
	}
	return &Room{epoch: epoch, members: make(map[string]*memberState)}
}
func (r *Room) Join(member string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member == "" {
		return ErrMember
	}
	if existing := r.members[member]; existing != nil {
		existing.online = true
	} else {
		r.members[member] = &memberState{online: true}
	}
	return nil
}
func (r *Room) Leave(member string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, member)
	if r.current != nil {
		_, recipient := r.current.ready[member]
		if r.current.Speaker == member || (recipient && r.current.Phase == "preparing") {
			r.end("MEMBER_LEFT")
		} else if recipient {
			delete(r.current.ready, member)
			ids := []string{}
			for _, id := range r.current.Recipients {
				if id != member {
					ids = append(ids, id)
				}
			}
			r.current.Recipients = ids
			if len(ids) == 0 {
				r.end("NO_RECIPIENTS")
			}
		}
	}
}

// Disconnect is not Leave: sleeping listeners retain membership and readiness
// must be re-established for every burst. A disconnected speaker loses its turn.
func (r *Room) Disconnect(member string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := r.members[member]; m != nil {
		m.online = false
	}
	if r.current != nil && r.current.Speaker == member {
		r.end("SPEAKER_DISCONNECTED")
	}
}
func (r *Room) expire(now time.Time) {
	if r.current != nil && !now.Before(r.current.Deadline) {
		reason := "LEASE_EXPIRED"
		if r.current.Phase == "preparing" {
			reason = "PREPARE_TIMEOUT"
		} else if !now.Before(r.current.limit) {
			reason = "BURST_LIMIT"
		}
		r.end(reason)
	}
}
func (r *Room) Request(member string, now time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if r.members[member] == nil {
		return Snapshot{}, ErrMember
	}
	if r.current != nil {
		return Snapshot{}, ErrBusy
	}
	recipients := []string{}
	ready := map[string]bool{}
	for id, state := range r.members {
		if id != member {
			if !state.online && !state.canWake {
				return Snapshot{}, ErrPeerUnreachable
			}
			recipients = append(recipients, id)
			ready[id] = false
		}
	}
	if len(recipients) == 0 {
		return Snapshot{}, ErrNoRecipients
	}
	sort.Strings(recipients)
	r.sequence++
	r.current = &turn{Snapshot: Snapshot{Epoch: r.epoch, Burst: fmt.Sprint(r.sequence), Speaker: member, Phase: "preparing", Recipients: recipients, Deadline: now.Add(Preparation)}, ready: ready}
	return r.snapshot(), nil
}
func (r *Room) match(epoch, burst string) bool {
	return r.current != nil && r.epoch == epoch && r.current.Burst == burst
}

// WakeAck deliberately cannot change readiness or issue a grant.
func (r *Room) WakeAck(member, epoch, burst string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if !r.match(epoch, burst) {
		return ErrStale
	}
	if _, ok := r.current.ready[member]; !ok {
		return ErrNotRecipient
	}
	return nil
}
func (r *Room) ready(member string, now time.Time) (Snapshot, error) {
	if _, ok := r.current.ready[member]; !ok {
		return Snapshot{}, ErrNotRecipient
	}
	r.current.ready[member] = true
	if r.current.Phase == "preparing" {
		all := true
		for _, v := range r.current.ready {
			all = all && v
		}
		if all {
			r.current.Phase = "active"
			r.current.Grant = fmt.Sprintf("grant-%d", r.sequence)
			r.current.limit = now.Add(MaximumBurst)
			r.current.Deadline = now.Add(Lease)
		}
	}
	return r.snapshot(), nil
}
func (r *Room) Renew(member, epoch, burst, grant string, now time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if !r.match(epoch, burst) || r.current.Phase != "active" || r.current.Speaker != member || r.current.Grant != grant {
		return Snapshot{}, ErrStale
	}
	deadline := now.Add(Lease)
	if deadline.After(r.current.limit) {
		deadline = r.current.limit
	}
	r.current.Deadline = deadline
	return r.snapshot(), nil
}

// Cancel supports both releasing an active grant and cancelling preparation.
// A network adapter must bind this operation to the authenticated speaker.
func (r *Room) Cancel(member, epoch, burst string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if !r.match(epoch, burst) || r.current.Speaker != member {
		return ErrStale
	}
	r.end("CANCELLED")
	return nil
}
func (r *Room) snapshot() Snapshot {
	if r.current == nil {
		return Snapshot{Epoch: r.epoch, Phase: "idle"}
	}
	s := r.current.Snapshot
	s.Recipients = append([]string(nil), s.Recipients...)
	return s
}
func (r *Room) State(now time.Time) Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	return r.snapshot()
}

// end retains only the latest transition, not an unbounded event log. The network
// adapter must serialize operations and publish transitions before advancing.
func (r *Room) end(reason string) {
	if r.current == nil {
		return
	}
	r.lastEnd = &End{Epoch: r.epoch, Burst: r.current.Burst, Speaker: r.current.Speaker, Reason: reason}
	r.current = nil
}
func (r *Room) LastEnd() (End, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastEnd == nil {
		return End{}, false
	}
	return *r.lastEnd, true
}

// LocalStop is an idempotent sender action. A delayed stop of an old burst
// cannot cancel a newer burst. Unknown identities and old epochs still fail.
func (r *Room) LocalStop(member, epoch, burst string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if r.members[member] == nil {
		return ErrMember
	}
	if epoch != r.epoch {
		return ErrStale
	}
	if r.current == nil || r.current.Burst != burst {
		return nil
	}
	if r.current.Speaker != member {
		return ErrNotRecipient
	}
	r.end("LOCAL_STOP")
	return nil
}

// Unavailable conservatively ends the current burst if an affected participant
// loses audio, network, or permission. Stale events cannot affect a new burst.
func (r *Room) Unavailable(member, epoch, burst, reason string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if r.members[member] == nil {
		return ErrMember
	}
	if !r.match(epoch, burst) {
		return ErrStale
	}
	if _, ok := r.current.ready[member]; !ok && r.current.Speaker != member {
		return ErrNotRecipient
	}
	switch reason {
	case "AUDIO_INTERRUPTED", "NETWORK_CHANGED", "PERMISSION_REVOKED", "AUDIO_UNAVAILABLE", "PLAYOUT_FAILED", "MIC_DENIED", "LAN_DENIED":
	default:
		return errors.New("invalid interruption reason")
	}
	r.end(reason)
	return nil
}

// ReadyFor prevents a stale or misdirected peer readiness claim from authorizing
// the wrong sender. Peer and burst checks run under the same lock as the update.
func (r *Room) ReadyFor(member, peer, epoch, burst string, now time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if !r.match(epoch, burst) || peer != r.current.Speaker {
		return Snapshot{}, ErrStale
	}
	return r.ready(member, now)
}

// SetWakeable is called only after validating a private APNs registration. A
// member without a wake provider cannot be labelled wakeable when disconnected.
func (r *Room) SetWakeable(member string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.members[member]
	if m == nil {
		return ErrMember
	}
	m.canWake = enabled
	return nil
}
func (r *Room) Reachability(member string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.members[member]
	if m == nil {
		return "", ErrMember
	}
	if m.online {
		return "online", nil
	}
	if m.canWake {
		return "wakeable", nil
	}
	return "unreachable", nil
}
