package simulate

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// SkipClock is the real clock, except that Skip fires the functions waiting on its AfterFunc timers now.
type SkipClock struct {
	clockwork.Clock

	mu      sync.Mutex
	pending map[*skipTimer]bool
}

func NewSkipClock() *SkipClock {
	return &SkipClock{Clock: clockwork.NewRealClock(), pending: map[*skipTimer]bool{}}
}

func (c *SkipClock) AfterFunc(d time.Duration, f func()) clockwork.Timer {
	t := &skipTimer{clock: c, f: f}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[t] = true
	t.Timer = c.Clock.AfterFunc(d, t.fire)
	return t
}

// Skip fires every pending AfterFunc timer now, instead of when it is due.
func (c *SkipClock) Skip() {
	c.mu.Lock()
	timers := make([]*skipTimer, 0, len(c.pending))
	for t := range c.pending {
		t.Timer.Stop()
		timers = append(timers, t)
	}
	clear(c.pending)
	c.mu.Unlock()

	for _, t := range timers {
		go t.f()
	}
}

// take removes t from the pending timers, reporting whether it was pending.
func (c *SkipClock) take(t *skipTimer) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	pending := c.pending[t]
	delete(c.pending, t)
	return pending
}

type skipTimer struct {
	clockwork.Timer
	clock *SkipClock
	f     func()
}

// fire runs f when the timer is due, unless Skip already ran it.
func (t *skipTimer) fire() {
	if t.clock.take(t) {
		t.f()
	}
}

func (t *skipTimer) Stop() bool {
	t.clock.take(t)
	return t.Timer.Stop()
}

func (t *skipTimer) Reset(d time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	t.clock.pending[t] = true
	return t.Timer.Reset(d)
}
