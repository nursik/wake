package wake

import (
	"context"
	"sync"
)

type commonCond struct {
	s *Signaller
	r *Receiver
}

func (c *commonCond) Signal(n int) int {
	if n <= 0 {
		return c.s.Signal(0)
	}

	var x int
	// we need to notify at least one receiver if we know that at least one is waiting.
	// we are doing it in for loop, because unlike golang's sync.Cond we may start waiting after sending Signal.
	// golang's sync.Cond Wait() appends to notification_list before unlocking.
	for c.s.sig.waits.Load() > 0 {
		x = c.s.Signal(n)
		n = n - x
		if x > 0 {
			break
		}
	}
	// don't accidentally broadcast
	if n == 0 {
		return x
	}
	return x + c.s.Signal(n)
}

func (c *commonCond) SignalWithCtx(ctx context.Context, n int) (int, error) {
	return c.s.SignalWithCtx(ctx, n)
}

func (c *commonCond) Broadcast() {
	c.s.Broadcast()
}

func (c *commonCond) Close() bool {
	return c.s.Close()
}

func (c *commonCond) IsClosed() bool {
	return c.s.IsClosed()
}

func (c *commonCond) WaitCount() int {
	return c.s.WaitCount()
}

type Cond struct {
	L sync.Locker
	commonCond
}

func (c *Cond) Wait() bool {
	subsig := c.r.sig.subsig.Load()
	// We have to do it before Unlock, so Signal will behave as sync.Cond.Signal
	c.r.sig.waits.Add(1)
	c.L.Unlock()
	ok := c.r.swait(subsig)
	c.r.sig.waits.Add(-1)
	c.L.Lock()
	return ok
}

func (c *Cond) WaitWithCtx(ctx context.Context) (bool, error) {
	subsig := c.r.sig.subsig.Load()
	c.r.sig.waits.Add(1)
	c.L.Unlock()
	ok, err := c.r.swaitWithCtx(ctx, subsig)
	c.r.sig.waits.Add(-1)
	c.L.Lock()
	return ok, err
}

// NewCond returns Cond with associated locker. The same as sync.Cond, but has more functionality. Only Wait like methods use lock.
// Slower than sync.Cond by ~3 times (sync.Cond's tests which only benchmarks broadcast).
func NewCond(l sync.Locker) *Cond {
	s, r := New()
	return &Cond{
		L: l,
		commonCond: commonCond{
			s: s,
			r: r,
		},
	}
}

type RWCond struct {
	L *sync.RWMutex
	commonCond
}

func (c *RWCond) Wait() bool {
	subsig := c.r.sig.subsig.Load()
	c.L.RUnlock()
	ok := c.r.swait(subsig)
	c.L.RLock()
	return ok
}

func (c *RWCond) WaitWithCtx(ctx context.Context) (bool, error) {
	subsig := c.r.sig.subsig.Load()
	c.L.RUnlock()
	ok, err := c.r.swaitWithCtx(ctx, subsig)
	c.L.RLock()
	return ok, err
}

// NewRWCond returns RWCond with associated sync.RWMutex. Uses RUnlock and Rlock for Wait like methods. Other methods do not use lock.
func NewRWCond(l *sync.RWMutex) *RWCond {
	s, r := New()
	return &RWCond{
		L: l,
		commonCond: commonCond{
			s: s,
			r: r,
		},
	}
}
