package service

import (
	"fmt"
	"testing"
	"time"
)

func TestBackgroundWorkCoalescesAndBoundsJobs(t *testing.T) {
	var queue BoundedBackground
	started := make(chan []string, 20)
	release := make(chan struct{})
	work := func(values []string) { started <- values; <-release }
	if !queue.Submit("user", "first", work) {
		t.Fatal("first submit rejected")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("did not start")
	}
	for i := 0; i < 10; i++ {
		queue.Submit("user", fmt.Sprint(i), work)
	}
	for i := 0; i < 7; i++ {
		if !queue.Submit(fmt.Sprint(i), "item", work) {
			t.Fatal("under capacity")
		}
	}
	if queue.Submit("overflow", "item", work) {
		t.Fatal("unbounded active users")
	}
	queue.mu.Lock()
	if len(queue.pending["user"]) != 10 {
		t.Fatal("pending updates not coalesced")
	}
	queue.mu.Unlock()
	close(release)
}
