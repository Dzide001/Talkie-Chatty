// Package replay provides a bounded request ledger for one authenticated member
// session. It must outlive sockets and be discarded when that session expires.
package replay

import (
	"crypto/sha256"
	"errors"
	"sync"
)

var (
	ErrInvalid = errors.New("invalid replay input")
	ErrReused  = errors.New("request ID reused with different content")
	ErrFull    = errors.New("session replay capacity reached")
	ErrTicket  = errors.New("invalid or completed replay ticket")
)

type Status int

const (
	New Status = iota
	Pending
	Complete
)

type Ticket struct {
	owner *Cache
	id    string
	entry *entry
}
type entry struct {
	digest   [32]byte
	response []byte
	complete bool
}
type Cache struct {
	mu                                            sync.Mutex
	entries                                       map[string]*entry
	maxEntries, maxRequestBytes, maxResponseBytes int
}

// NewCache reserves a fixed maximum response size per entry, bounding retained
// memory by maxEntries * maxResponseBytes plus fixed metadata. Completed entries
// are never evicted: at capacity, new work fails closed rather than replaying an
// old effect. The adapter must impose service-wide admission limits as well.
func NewCache(maxEntries, maxRequestBytes, maxResponseBytes int) (*Cache, error) {
	if maxEntries < 1 || maxEntries > 4096 || maxRequestBytes < 1 || maxRequestBytes > 65536 || maxResponseBytes < 1 || maxResponseBytes > 65536 || int64(maxEntries)*int64(maxResponseBytes) > 4<<20 {
		return nil, ErrInvalid
	}
	return &Cache{entries: map[string]*entry{}, maxEntries: maxEntries, maxRequestBytes: maxRequestBytes, maxResponseBytes: maxResponseBytes}, nil
}

// Begin compares exact validated request bytes. The caller must use the same
// serialization for retries, or canonicalize before calling. Identity/epoch and
// schema checks must precede Begin. Pending is not authorization to execute again.
func (c *Cache) Begin(id string, request []byte) (Status, Ticket, []byte, error) {
	if len(id) < 1 || len(id) > 128 || len(request) < 1 || len(request) > c.maxRequestBytes {
		return 0, Ticket{}, nil, ErrInvalid
	}
	digest := sha256.Sum256(request)
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[id]; e != nil {
		if e.digest != digest {
			return 0, Ticket{}, nil, ErrReused
		}
		if !e.complete {
			return Pending, Ticket{}, nil, nil
		}
		return Complete, Ticket{}, append([]byte(nil), e.response...), nil
	}
	if len(c.entries) >= c.maxEntries {
		return 0, Ticket{}, nil, ErrFull
	}
	e := &entry{digest: digest}
	c.entries[id] = e
	return New, Ticket{c, id, e}, nil, nil
}

// Finish stores the originator's bounded result. An invalid/oversized completion
// leaves work pending (never executable again); callers must use a bounded error
// result to complete it. There is deliberately no abort-and-retry operation.
func (c *Cache) Finish(ticket Ticket, response []byte) error {
	if len(response) > c.maxResponseBytes {
		return ErrInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[ticket.id]
	if ticket.owner != c || e == nil || e != ticket.entry || e.complete {
		return ErrTicket
	}
	e.response = append([]byte(nil), response...)
	e.complete = true
	return nil
}
