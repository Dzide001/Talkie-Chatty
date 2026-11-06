package floor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func setup() (*Room, time.Time) {
	r := New("epoch-one")
	_ = r.Join("a")
	_ = r.Join("b")
	return r, time.Now()
}
func TestWakeAckIsNotReady(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	if err := r.WakeAck("b", s.Epoch, s.Burst, n); err != nil {
		t.Fatal(err)
	}
	if r.State(n).Phase != "preparing" {
		t.Fatal("wake acknowledgment granted audio")
	}
	s, err := r.ReadyFor("b", "a", s.Epoch, s.Burst, n.Add(4*time.Second))
	if err != nil || s.Phase != "active" || !s.Deadline.Equal(n.Add(7*time.Second)) {
		t.Fatalf("separate preparation/lease: %+v %v", s, err)
	}
}
func TestConcurrentRequests(t *testing.T) {
	r, n := setup()
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := r.Request("a", n); e == nil {
				wins.Add(1)
			} else if !errors.Is(e, ErrBusy) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("winners=%d", wins.Load())
	}
}
func TestSleepingMemberAndSnapshot(t *testing.T) {
	r, n := setup()
	_ = r.SetWakeable("b", true)
	r.Disconnect("b")
	s, e := r.Request("a", n)
	if e != nil || len(s.Recipients) != 1 || s.Recipients[0] != "b" {
		t.Fatalf("sleeping member lost: %+v %v", s, e)
	}
	_ = r.Join("c")
	if _, e = r.ReadyFor("c", "a", s.Epoch, s.Burst, n); !errors.Is(e, ErrNotRecipient) {
		t.Fatal("late join entered burst")
	}
	if _, e = r.ReadyFor("b", "a", s.Epoch, s.Burst, n); e != nil {
		t.Fatal(e)
	}
	r.Disconnect("a")
	if r.State(n).Phase != "idle" {
		t.Fatal("speaker disconnect retained grant")
	}
}
func TestExpiryAndStale(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	if _, e := r.ReadyFor("b", "a", s.Epoch, s.Burst, n.Add(Preparation)); !errors.Is(e, ErrStale) {
		t.Fatal("expired preparation accepted")
	}
	fresh, _ := r.Request("a", n.Add(Preparation))
	if _, e := r.ReadyFor("b", "a", s.Epoch, s.Burst, n.Add(Preparation)); !errors.Is(e, ErrStale) {
		t.Fatal("old burst accepted")
	}
	if _, e := r.ReadyFor("b", "a", "old-epoch", fresh.Burst, n.Add(Preparation)); !errors.Is(e, ErrStale) {
		t.Fatal("old epoch accepted")
	}
}
func TestMaximumBurstAndAuthorization(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	s, _ = r.ReadyFor("b", "a", s.Epoch, s.Burst, n)
	if _, e := r.Renew("b", s.Epoch, s.Burst, s.Grant, n); !errors.Is(e, ErrStale) {
		t.Fatal("listener renewed speaker")
	}
	if e := r.Cancel("b", s.Epoch, s.Burst, n); !errors.Is(e, ErrStale) {
		t.Fatal("listener cancelled speaker")
	}
	for i := 1; i < 30; i++ {
		var e error
		s, e = r.Renew("a", s.Epoch, s.Burst, s.Grant, n.Add(time.Duration(i)*time.Second))
		if e != nil {
			t.Fatal(e)
		}
	}
	if r.State(n.Add(MaximumBurst)).Phase != "idle" {
		t.Fatal("maximum burst exceeded")
	}
}
func TestDepartureAndEmptyRoom(t *testing.T) {
	r, n := setup()
	_, _ = r.Request("a", n)
	r.Leave("b")
	if r.State(n).Phase != "idle" {
		t.Fatal("departure retained preparation")
	}
	if _, e := r.Request("a", n); !errors.Is(e, ErrNoRecipients) {
		t.Fatal(e)
	}
	if _, e := r.Request("unknown", n); !errors.Is(e, ErrMember) {
		t.Fatal(e)
	}
}
func TestSnapshotCannotMutateRoom(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	s.Recipients[0] = "intruder"
	if r.State(n).Recipients[0] != "b" {
		t.Fatal("mutable state escaped")
	}
}

func TestDuplicateReadyDoesNotExtendLease(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	s, _ = r.ReadyFor("b", "a", s.Epoch, s.Burst, n)
	repeat, e := r.ReadyFor("b", "a", s.Epoch, s.Burst, n.Add(2*time.Second))
	if e != nil || !repeat.Deadline.Equal(s.Deadline) {
		t.Fatal("duplicate ready renewed lease")
	}
	if _, e = r.Renew("a", s.Epoch, s.Burst, s.Grant, n.Add(Lease)); !errors.Is(e, ErrStale) {
		t.Fatal("expired grant revived")
	}
}
func TestCancelAndReacquire(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	if e := r.Cancel("a", s.Epoch, s.Burst, n); e != nil {
		t.Fatal(e)
	}
	fresh, e := r.Request("b", n)
	if e != nil || fresh.Burst == s.Burst {
		t.Fatal("reused burst")
	}
	if e := r.Cancel("a", s.Epoch, s.Burst, n); !errors.Is(e, ErrStale) {
		t.Fatal("stale cancellation accepted")
	}
}

func TestLocalStopIdempotentAndOldStopSafe(t *testing.T) {
	r, n := setup()
	old, _ := r.Request("a", n)
	for i := 0; i < 2; i++ {
		if e := r.LocalStop("a", old.Epoch, old.Burst, n); e != nil {
			t.Fatal(e)
		}
	}
	current, _ := r.Request("b", n)
	if e := r.LocalStop("a", old.Epoch, old.Burst, n); e != nil {
		t.Fatal(e)
	}
	if r.State(n).Burst != current.Burst {
		t.Fatal("old stop cancelled new burst")
	}
	if e := r.LocalStop("a", "old-epoch", old.Burst, n); !errors.Is(e, ErrStale) {
		t.Fatal(e)
	}
}
func TestPeerReadinessAndLifecycleRevocation(t *testing.T) {
	for _, reason := range []string{"AUDIO_INTERRUPTED", "NETWORK_CHANGED", "PERMISSION_REVOKED", "AUDIO_UNAVAILABLE"} {
		t.Run(reason, func(t *testing.T) {
			r, n := setup()
			s, _ := r.Request("a", n)
			if _, e := r.ReadyFor("b", "wrong-peer", s.Epoch, s.Burst, n); !errors.Is(e, ErrStale) {
				t.Fatal("wrong peer ready")
			}
			if _, e := r.ReadyFor("b", "a", s.Epoch, s.Burst, n); e != nil {
				t.Fatal(e)
			}
			if e := r.Unavailable("b", s.Epoch, s.Burst, reason, n); e != nil {
				t.Fatal(e)
			}
			ended, ok := r.LastEnd()
			if !ok || ended.Reason != reason || ended.Burst != s.Burst || r.State(n).Phase != "idle" {
				t.Fatal(ended)
			}
		})
	}
}
func TestExpiryReasons(t *testing.T) {
	r, n := setup()
	_, _ = r.Request("a", n)
	r.State(n.Add(Preparation))
	ended, _ := r.LastEnd()
	if ended.Reason != "PREPARE_TIMEOUT" {
		t.Fatal(ended)
	}
	s, _ := r.Request("a", n.Add(Preparation))
	_, _ = r.ReadyFor("b", "a", s.Epoch, s.Burst, n.Add(Preparation))
	r.State(n.Add(Preparation + Lease))
	ended, _ = r.LastEnd()
	if ended.Reason != "LEASE_EXPIRED" {
		t.Fatal(ended)
	}
}
func TestExpiredStopAndUnknownInterruption(t *testing.T) {
	r, n := setup()
	s, _ := r.Request("a", n)
	if e := r.Unavailable("b", s.Epoch, s.Burst, "arbitrary", n); e == nil {
		t.Fatal("unknown reason accepted")
	}
	if e := r.LocalStop("a", s.Epoch, s.Burst, n.Add(Preparation)); e != nil {
		t.Fatal(e)
	}
	if e := r.Unavailable("b", s.Epoch, s.Burst, "NETWORK_CHANGED", n.Add(Preparation)); !errors.Is(e, ErrStale) {
		t.Fatal(e)
	}
}

func TestOfflineWithoutWakeFailsFast(t *testing.T) {
	r, n := setup()
	r.Disconnect("b")
	if state, _ := r.Reachability("b"); state != "unreachable" {
		t.Fatal(state)
	}
	if _, e := r.Request("a", n); !errors.Is(e, ErrPeerUnreachable) {
		t.Fatal(e)
	}
	if r.State(n).Phase != "idle" {
		t.Fatal("unreachable peer reserved floor")
	}
	_ = r.SetWakeable("b", true)
	if state, _ := r.Reachability("b"); state != "wakeable" {
		t.Fatal(state)
	}
	if _, e := r.Request("a", n); e != nil {
		t.Fatal(e)
	}
}
func TestActiveListenerLeaveKeepsOtherRecipients(t *testing.T) {
	r, n := setup()
	_ = r.Join("c")
	s, _ := r.Request("a", n)
	_, _ = r.ReadyFor("b", "a", s.Epoch, s.Burst, n)
	_, _ = r.ReadyFor("c", "a", s.Epoch, s.Burst, n)
	r.Leave("b")
	current := r.State(n)
	if current.Phase != "active" || len(current.Recipients) != 1 || current.Recipients[0] != "c" {
		t.Fatal(current)
	}
	r.Leave("c")
	if r.State(n).Phase != "idle" {
		t.Fatal("speech without listeners")
	}
	ended, _ := r.LastEnd()
	if ended.Reason != "NO_RECIPIENTS" {
		t.Fatal(ended)
	}
}
