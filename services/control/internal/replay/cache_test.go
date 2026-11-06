package replay

import (
	"errors"
	"sync"
	"testing"
)

func TestConcurrentRetryExecutesOnce(t *testing.T) {
	c, _ := NewCache(4, 128, 128)
	var wg sync.WaitGroup
	tickets := make(chan Ticket, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, ticket, _, err := c.Begin("request", []byte("body"))
			if err != nil {
				t.Error(err)
			}
			if status == New {
				tickets <- ticket
			} else if status != Pending {
				t.Error(status)
			}
		}()
	}
	wg.Wait()
	close(tickets)
	count := 0
	for ticket := range tickets {
		count++
		if err := c.Finish(ticket, []byte("result")); err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatalf("executed %d times", count)
	}
	status, _, response, err := c.Begin("request", []byte("body"))
	if status != Complete || string(response) != "result" || err != nil {
		t.Fatal(status, string(response), err)
	}
}
func TestConflictAndCapacityNeverEvictHistory(t *testing.T) {
	c, _ := NewCache(1, 128, 128)
	_, ticket, _, _ := c.Begin("a", []byte("one"))
	if _, _, _, err := c.Begin("a", []byte("two")); err != ErrReused {
		t.Fatal(err)
	}
	if _, _, _, err := c.Begin("b", []byte("one")); err != ErrFull {
		t.Fatal(err)
	}
	if err := c.Finish(ticket, []byte("done")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := c.Begin("b", []byte("one")); err != ErrFull {
		t.Fatal(err)
	}
	status, _, _, err := c.Begin("a", []byte("one"))
	if status != Complete || err != nil {
		t.Fatal(status, err)
	}
}
func TestResultIsolationAndForeignTicket(t *testing.T) {
	a, _ := NewCache(2, 128, 128)
	b, _ := NewCache(2, 128, 128)
	_, ticket, _, _ := a.Begin("a", []byte("one"))
	b.Begin("a", []byte("one"))
	if err := b.Finish(ticket, []byte("wrong")); err != ErrTicket {
		t.Fatal(err)
	}
	result := []byte("done")
	if err := a.Finish(ticket, result); err != nil {
		t.Fatal(err)
	}
	result[0] = 'X'
	_, _, r, _ := a.Begin("a", []byte("one"))
	if string(r) != "done" {
		t.Fatal(string(r))
	}
	r[0] = 'Y'
	_, _, r, _ = a.Begin("a", []byte("one"))
	if string(r) != "done" {
		t.Fatal(string(r))
	}
	if err := a.Finish(ticket, []byte("changed")); err != ErrTicket {
		t.Fatal(err)
	}
}
func TestBoundsAndFailedCompletionRemainPending(t *testing.T) {
	if _, err := NewCache(4096, 65536, 65536); err != ErrInvalid {
		t.Fatal(err)
	}
	c, _ := NewCache(1, 4, 4)
	if _, _, _, err := c.Begin("a", []byte("large")); err != ErrInvalid {
		t.Fatal(err)
	}
	_, ticket, _, _ := c.Begin("a", []byte("okay"))
	if err := c.Finish(ticket, []byte("large")); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	status, _, _, err := c.Begin("a", []byte("okay"))
	if status != Pending || err != nil {
		t.Fatal(status, err)
	}
	if err := c.Finish(ticket, []byte("fail")); err != nil {
		t.Fatal(err)
	}
}
