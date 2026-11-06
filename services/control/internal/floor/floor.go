// Package floor implements the internal, transport-independent speaking-turn
// state machine. It is not a network API. Callers must authenticate member IDs,
// supply server time (never client timestamps), and schedule expiry processing.
package floor

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrMember       = errors.New("unknown member")
	ErrBusy         = errors.New("floor busy")
	ErrNoRecipients = errors.New("no recipients")
	ErrStale        = errors.New("stale burst or grant")
	ErrNotRecipient = errors.New("not a recipient")
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
type Room struct {
	mu       sync.Mutex
	epoch    string
	sequence uint64
	members  map[string]bool
	current  *turn
}

// New requires a fresh, nonempty epoch on every coordinator restart. IDs are
// opaque correlation identifiers, not credentials.
func New(epoch string) *Room {
	if epoch == "" {
		panic("floor: empty epoch")
	}
	return &Room{epoch: epoch, members: make(map[string]bool)}
}
func (r *Room) Join(member string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member == "" {
		return ErrMember
	}
	r.members[member] = true
	return nil
}
func (r *Room) Leave(member string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, member)
	if r.current != nil {
		// Conservatively revoke on departure of a participant in this burst.
		_, recipient := r.current.ready[member]
		if r.current.Speaker == member || recipient {
			r.current = nil
		}
	}
}

// Disconnect is not Leave: sleeping listeners retain membership and readiness
// must be re-established for every burst. A disconnected speaker loses its turn.
func (r *Room) Disconnect(member string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil && r.current.Speaker == member {
		r.current = nil
	}
}
func (r *Room) expire(now time.Time) {
	if r.current != nil && !now.Before(r.current.Deadline) {
		r.current = nil
	}
}
func (r *Room) Request(member string, now time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if !r.members[member] {
		return Snapshot{}, ErrMember
	}
	if r.current != nil {
		return Snapshot{}, ErrBusy
	}
	recipients := []string{}
	ready := map[string]bool{}
	for id := range r.members {
		if id != member {
			recipients = append(recipients, id)
			ready[id] = false
		}
	}
	if len(recipients) == 0 {
		return Snapshot{}, ErrNoRecipients
	}
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
func (r *Room) Ready(member, epoch, burst string, now time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	if !r.match(epoch, burst) {
		return Snapshot{}, ErrStale
	}
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
	r.current = nil
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
