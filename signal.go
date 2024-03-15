package wake

import (
	"context"
	"sync/atomic"
)

type subsignal struct {
	ch chan struct{}
}

func newSubsignal() *subsignal {
	s := new(subsignal)
	s.ch = make(chan struct{})
	return s
}

type signal struct {
	subsig  atomic.Pointer[subsignal]
	closed  atomic.Bool
	closeCh chan struct{}
	direct  chan struct{}
	waits   atomic.Int64
}

type Signaller struct {
	sig *signal
}

// Signal wakes n goroutines (if there are any) and returns how many goroutines were awoken.
// If n <= 0 it wakes all goroutines and returns 0 (the same as Broadcast()).
func (s *Signaller) Signal(n int) int {
	if n <= 0 {
		s.Broadcast()
		return 0
	}
	if s.IsClosed() {
		return 0
	}
	var count int
forloop:
	for n > 0 {
		// Notice: it is possible that signaller was closed and we potentially can wake up someone.
		// However having two cases in select makes it slower.
		select {
		case s.sig.direct <- struct{}{}:
			count++
			n--
		default:
			break forloop
		}
	}
	return count
}

// SignalWithCtx wakes n goroutines and reports how many goroutines were awoken and ctx.Err() if context was cancelled.
// It is a blocking operation and will be finished when all n goroutines are awoken, context is cancelled or signaller was closed.
// If n <= 0 it wakes all goroutines (the same as Broadcast()) regardless of context cancellation.
// Error is always nil for closed signaller or if n <= 0.
func (s *Signaller) SignalWithCtx(ctx context.Context, n int) (int, error) {
	if n <= 0 {
		s.Broadcast()
		return 0, nil
	}
	var count int
	for n > 0 {
		select {
		case s.sig.direct <- struct{}{}:
			count++
			n--
		case <-ctx.Done():
			return count, ctx.Err()
		case <-s.sig.closeCh:
			return count, nil
		}
	}

	return count, nil
}

// Broadcast wakes up all goroutines.
func (s *Signaller) Broadcast() {
	if s.IsClosed() {
		return
	}
	subsig := s.sig.subsig.Swap(newSubsignal())
	close(subsig.ch)
}

// WaitCount returns current number of goroutines waiting for signal.
func (s *Signaller) WaitCount() int {
	return int(s.sig.waits.Load())
}

// Close closes signaller and wakes all waiting goroutines.
// The first Close() returns true and subsequent calls always return false.
func (s *Signaller) Close() bool {
	first := !s.sig.closed.Swap(true)
	if first {
		close(s.sig.closeCh)
	}
	return first
}

// IsClosed reports if signaller is closed
func (s *Signaller) IsClosed() bool {
	return s.sig.closed.Load()
}

type Receiver struct {
	sig *signal
}

// Wait blocks until awaken by signaller (returns true) or signaller was closed (returns false).
func (r *Receiver) Wait() bool {
	if r.IsClosed() {
		return false
	}
	r.sig.waits.Add(1)
	defer r.sig.waits.Add(-1)

	subsig := r.sig.subsig.Load()

	select {
	case <-subsig.ch:
		return true
	case <-r.sig.closeCh:
		return false
	case <-r.sig.direct:
		return true
	}
}

func (r *Receiver) swait(subsig *subsignal) bool {
	if r.IsClosed() {
		return false
	}
	select {
	case <-subsig.ch:
		return true
	case <-r.sig.closeCh:
		return false
	case <-r.sig.direct:
		return true
	}
}

// WaitWithCtx blocks until awaken by signaller, context was cancelled or signaller was closed.
// Returns false and nil error only if signaller was closed.
// Returns true and error, where error is nil or ctx.Err().
func (r *Receiver) WaitWithCtx(ctx context.Context) (bool, error) {
	if r.IsClosed() {
		return false, nil
	}
	r.sig.waits.Add(1)
	defer r.sig.waits.Add(-1)

	subsig := r.sig.subsig.Load()
	select {
	case <-r.sig.closeCh:
		return false, nil
	case <-subsig.ch:
		return true, nil
	case <-ctx.Done():
		return true, ctx.Err()
	case <-r.sig.direct:
		return true, nil
	}
}

func (r *Receiver) swaitWithCtx(ctx context.Context, subsig *subsignal) (bool, error) {
	if r.IsClosed() {
		return false, nil
	}

	select {
	case <-r.sig.closeCh:
		return false, nil
	case <-subsig.ch:
		return true, nil
	case <-ctx.Done():
		return true, ctx.Err()
	case <-r.sig.direct:
		return true, nil
	}
}

// IsClosed reports if signaller was closed.
func (r *Receiver) IsClosed() bool {
	return r.sig.closed.Load()
}

// New returns signaller and receiver.
func New() (*Signaller, *Receiver) {
	sig := new(signal)
	sig.closeCh = make(chan struct{})
	sig.direct = make(chan struct{})
	sig.subsig.Store(newSubsignal())
	s, r := new(Signaller), new(Receiver)
	s.sig = sig
	r.sig = sig
	return s, r
}
