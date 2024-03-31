package wake_test

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	. "github.com/nursik/wake"
)

func equal[T comparable](t *testing.T, expected T, got T) {
	if expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func isNilError(t *testing.T, got error) {
	if got != nil {
		t.Fatalf("expected nil error, got %v", got)
	}
}

func TestNew(t *testing.T) {
	s, r := New()
	equal(t, false, s.IsClosed())
	equal(t, false, r.IsClosed())
	if s == nil {
		t.Fatalf("expected not nil, got %v", s)
	}
	if r == nil {
		t.Fatalf("expected not nil, got %v", r)
	}
	equal(t, 0, s.WaitCount())
}

func TestSignallerBroadcast(t *testing.T) {
	const N = 10
	s, r := New()
	awake := make(chan bool, N)

	s.Broadcast()

	for i := 0; i < N; i++ {
		go func() {
			r.Wait()
			awake <- true
			r.Wait()
			awake <- true
			r.Wait()
			awake <- true
			r.Wait()
			awake <- true
		}()
	}
	for s.WaitCount() != N {
	}
	s.Broadcast()
	for i := 0; i < N; i++ {
		<-awake
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	for s.WaitCount() != N {
	}
	s.Broadcast()
	for i := 0; i < N; i++ {
		<-awake
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	for s.WaitCount() != N {
	}
	equal(t, 0, s.Signal(0))
	for i := 0; i < N; i++ {
		<-awake
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	for s.WaitCount() != N {
	}
	cnt, err := s.SignalWithCtx(context.Background(), 0)
	equal(t, 0, cnt)
	isNilError(t, err)
	for i := 0; i < N; i++ {
		<-awake
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}
}

func TestSignallerSignal(t *testing.T) {
	const N = 20
	const M = 20
	const S = 10

	s, r := New()
	awake := make(chan bool, max(N+M, N*S))
	for i := 0; i < N+M; i++ {
		go func() {
			r.Wait()
			awake <- true
		}()
	}
	for s.WaitCount() != N+M {
	}
	// Usually r.Wait is fast enough to pass this test, but sometimes it is not.
	time.Sleep(time.Microsecond * 100)
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	equal(t, N, s.Signal(N))

	for i := 0; i < N; i++ {
		<-awake
	}

	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	equal(t, M, s.Signal(M+1))

	for i := 0; i < M; i++ {
		<-awake
	}

	equal(t, 0, s.Signal(1))

	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	for i := 0; i < N*S; i++ {
		go func() {
			r.Wait()
			awake <- true
		}()
	}
	for s.WaitCount() != N*S {
	}
	// Usually r.Wait is fast enough to pass this test, but sometimes it is not.
	time.Sleep(time.Microsecond * 100)
	start := make(chan struct{})

	for i := 0; i < S; i++ {
		go func() {
			<-start
			equal(t, N, s.Signal(N))
		}()
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}
	close(start)

	for i := 0; i < N*S; i++ {
		<-awake
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}
}

func TestSignallerSignalWithCtx(t *testing.T) {
	t.Run("Background", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)

		go func() {
			ok := r.Wait()
			equal(t, true, ok)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		cnt, err := s.SignalWithCtx(context.Background(), 1)
		equal(t, 1, cnt)
		isNilError(t, err)
		<-awake

		cnt, err = s.SignalWithCtx(context.Background(), -1)
		equal(t, 0, cnt)
		isNilError(t, err)

		s.Close()
		cnt, err = s.SignalWithCtx(context.Background(), 1)
		equal(t, 0, cnt)
		isNilError(t, err)
	})
	t.Run("Deadline", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)

		go func() {
			ok := r.Wait()
			equal(t, true, ok)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		cnt, err := s.SignalWithCtx(context.Background(), 1)
		equal(t, 1, cnt)
		isNilError(t, err)
		<-awake

		ctx1, cancel1 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel1()
		cnt, err = s.SignalWithCtx(ctx1, 1)
		equal(t, 0, cnt)
		equal(t, context.DeadlineExceeded, err)
		s.Close()

		// Sometimes SignalWithCtx is so fast that it bypasses IsClosed and returns ctx.Err() if deadline is Nanosecond
		ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
		defer cancel2()
		cnt, err = s.SignalWithCtx(ctx2, 1)
		equal(t, 0, cnt)
		isNilError(t, err)
	})

	t.Run("Cancel", func(t *testing.T) {
		s, r := New()
		done := make(chan struct{}, 2)
		ctx1, cancel1 := context.WithCancel(context.Background())

		go func() {
			cnt, err := s.SignalWithCtx(ctx1, 1)
			equal(t, context.Canceled, err)
			equal(t, 0, cnt)
			done <- struct{}{}
		}()
		select {
		case <-done:
			t.Fatal("goroutine not asleep")
		default:
		}

		cancel1()
		<-done

		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()

		go func() {
			cnt, err := s.SignalWithCtx(ctx2, 1)
			isNilError(t, err)
			equal(t, 1, cnt)
			done <- struct{}{}
		}()

		r.Wait()
		<-done

		ctx3, cancel3 := context.WithCancel(context.Background())
		defer cancel3()

		go func() {
			done <- struct{}{}
			cnt, err := s.SignalWithCtx(ctx3, 1)
			isNilError(t, err)
			equal(t, 0, cnt)
			done <- struct{}{}
		}()
		<-done
		s.Close()
		<-done

		cnt, err := s.SignalWithCtx(ctx2, 1)
		equal(t, 0, cnt)
		isNilError(t, err)
	})
}

func TestSignallerClose(t *testing.T) {
	const N = 10

	t.Run("Basic", func(t *testing.T) {
		s, r := New()

		equal(t, true, s.Close())
		equal(t, false, s.Close())
		equal(t, true, s.IsClosed())
		equal(t, true, r.IsClosed())
		equal(t, 0, s.WaitCount())
		equal(t, 0, s.Signal(0))
		equal(t, 0, s.Signal(1))
		cnt, err := s.SignalWithCtx(context.Background(), N)
		equal(t, 0, cnt)
		isNilError(t, err)

		cnt, err = s.SignalWithCtx(context.Background(), 0)
		equal(t, 0, cnt)
		isNilError(t, err)
	})

	t.Run("WakesOthers", func(t *testing.T) {
		s, r := New()
		running := make(chan struct{}, N)
		awake := make(chan struct{}, N)
		for i := 0; i < N; i++ {
			go func() {
				running <- struct{}{}
				if i < 5 {
					ok := r.Wait()
					equal(t, false, ok)
				} else {
					ok, err := r.WaitWithCtx(context.Background())
					equal(t, false, ok)
					isNilError(t, err)
				}
				awake <- struct{}{}
			}()
		}
		for i := 0; i < N; i++ {
			<-running
		}
		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}

		for s.WaitCount() != N {
		}

		s.Close()

		for i := 0; i < N; i++ {
			<-awake
		}

		equal(t, 0, s.WaitCount())
	})

	t.Run("UnblocksSignalWithCtx", func(t *testing.T) {
		s, _ := New()
		done := make(chan bool)
		go func() {
			cnt, err := s.SignalWithCtx(context.Background(), N)
			equal(t, 0, cnt)
			isNilError(t, err)
			done <- false
		}()

		s.Close()
		<-done
	})

	t.Run("SendsToReceiversValidOk", func(t *testing.T) {
		s, r := New()
		done := make(chan bool, N)

		for i := 0; i < N; i++ {
			go func() {
				done <- r.Wait()
			}()
		}
		for s.WaitCount() != N {
		}

		equal(t, N/2, s.Signal(N/2))

		for i := 0; i < N/2; i++ {
			equal(t, true, <-done)
		}
		select {
		case <-done:
			t.Fatal("goroutine not asleep")
		default:
		}

		s.Close()
		for i := 0; i < N-N/2; i++ {
			equal(t, false, <-done)
		}
	})
}

func TestReceiverWait(t *testing.T) {
	s, r := New()
	awake := make(chan bool, 1)

	go func() {
		awake <- r.Wait()
	}()

	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}
	// Using with context, because sometimes it is too fast and signals before goroutine starts waiting
	s.SignalWithCtx(context.Background(), 1)
	equal(t, true, <-awake)

	s.Close()
	equal(t, false, r.Wait())
}

// Must be a copy of TestReceiverWait
func TestUnsafeWait(t *testing.T) {
	s, r := New()
	awake := make(chan bool, 1)
	var locker sync.Mutex

	go func() {
		locker.Lock()
		awake <- UnsafeWait(r, &locker)
		locker.Unlock()
	}()

	go func() {
		locker.Lock()
		awake <- UnsafeWait(r, &locker)
		locker.Unlock()
	}()

	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}
	// Using with context, because sometimes it is too fast and signals before goroutine starts waiting
	for s.WaitCount() != 2 {

	}
	s.SignalWithCtx(context.Background(), 1)
	equal(t, true, <-awake)

	s.Broadcast()
	equal(t, true, <-awake)

	s.Close()
	locker.Lock()
	equal(t, false, UnsafeWait(r, &locker))
	locker.Unlock()
}

func TestReceiverWaitWithCtx(t *testing.T) {
	t.Run("Background", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)

		go func() {
			ok, err := r.WaitWithCtx(context.Background())
			isNilError(t, err)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		// Using with context, because sometimes it is too fast and signals before goroutine starts waiting
		s.SignalWithCtx(context.Background(), 1)
		equal(t, true, <-awake)

		s.Close()
		ok, err := r.WaitWithCtx(context.Background())
		equal(t, false, ok)
		isNilError(t, err)
	})
	t.Run("BackgroundBroadcast", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)

		go func() {
			ok, err := r.WaitWithCtx(context.Background())
			isNilError(t, err)
			awake <- ok
			s.Close()
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}

		for !s.IsClosed() {
			s.Broadcast()
		}
		<-awake
	})
	t.Run("Deadline", func(t *testing.T) {
		s, r := New()
		ctx1, cancel1 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel1()
		ok, err := r.WaitWithCtx(ctx1)
		equal(t, true, ok)
		equal(t, context.DeadlineExceeded, err)

		s.Close()
		ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel2()

		ok, err = r.WaitWithCtx(ctx2)
		equal(t, false, ok)
		isNilError(t, err)
	})

	t.Run("Cancel", func(t *testing.T) {
		s, r := New()
		done := make(chan struct{}, 2)
		ctx1, cancel1 := context.WithCancel(context.Background())

		go func() {
			ok, err := r.WaitWithCtx(ctx1)
			equal(t, true, ok)
			equal(t, context.Canceled, err)
			done <- struct{}{}
		}()
		select {
		case <-done:
			t.Fatal("goroutine not asleep")
		default:
		}

		cancel1()
		<-done

		s.Close()

		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()

		go func() {
			ok, err := r.WaitWithCtx(ctx2)
			equal(t, false, ok)
			isNilError(t, err)
			done <- struct{}{}
		}()

		<-done
	})
}

// Must be a copy of ReceiverWaitWithCtx
func TestUnsafeWaitCtx(t *testing.T) {
	t.Run("Background", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)
		var locker sync.Mutex

		go func() {
			locker.Lock()
			ok, err := UnsafeWaitCtx(r, &locker, context.Background())
			locker.Unlock()
			isNilError(t, err)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		// Using with context, because sometimes it is too fast and signals before goroutine starts waiting
		s.SignalWithCtx(context.Background(), 1)
		equal(t, true, <-awake)

		s.Close()
		locker.Lock()
		ok, err := UnsafeWaitCtx(r, &locker, context.Background())
		locker.Unlock()
		equal(t, false, ok)
		isNilError(t, err)
	})
	t.Run("BackgroundBroadcast", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)
		var locker sync.Mutex

		go func() {
			locker.Lock()
			ok, err := UnsafeWaitCtx(r, &locker, context.Background())
			locker.Unlock()
			isNilError(t, err)
			awake <- ok
			s.Close()
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}

		for !s.IsClosed() {
			s.Broadcast()
		}
		<-awake
	})
	t.Run("Deadline", func(t *testing.T) {
		s, r := New()
		var locker sync.Mutex

		ctx1, cancel1 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel1()
		locker.Lock()
		ok, err := UnsafeWaitCtx(r, &locker, ctx1)
		locker.Unlock()
		equal(t, true, ok)
		equal(t, context.DeadlineExceeded, err)

		s.Close()

		ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel2()

		locker.Lock()
		ok, err = UnsafeWaitCtx(r, &locker, ctx2)
		locker.Unlock()
		equal(t, false, ok)
		isNilError(t, err)
	})

	t.Run("Cancel", func(t *testing.T) {
		s, r := New()
		done := make(chan struct{}, 2)
		var locker sync.Mutex

		ctx1, cancel1 := context.WithCancel(context.Background())

		go func() {
			locker.Lock()
			ok, err := UnsafeWaitCtx(r, &locker, ctx1)
			locker.Unlock()
			equal(t, true, ok)
			equal(t, context.Canceled, err)
			done <- struct{}{}
		}()
		select {
		case <-done:
			t.Fatal("goroutine not asleep")
		default:
		}

		cancel1()
		<-done

		s.Close()

		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		go func() {
			locker.Lock()
			ok, err := UnsafeWaitCtx(r, &locker, ctx2)
			locker.Unlock()
			equal(t, false, ok)
			isNilError(t, err)
			done <- struct{}{}
		}()

		<-done
	})
}

func TestSignallerSignalConcurrent(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	const R = 10
	const S = 4

	s, r := New()
	done := make(chan int, R+S)

	for i := 0; i < R; i++ {
		go func() {
			var cur int
			for r.Wait() {
				cur++
			}
			done <- cur
		}()
	}

	for i := 0; i < S; i++ {
		go func() {
			var cur int
			for !s.IsClosed() {
				n := rand.Intn(R) + 1
				cur -= s.Signal(n)
			}
			done <- cur
		}()
	}
	time.Sleep(time.Second * 5)
	s.Close()
	var total int

	for i := 0; i < S+R; i++ {
		total += <-done
	}
	equal(t, 0, total)
}

func TestSignallerSignalWithCtxConcurrent(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	const R = 10
	const S = 4

	t.Run("Background", func(t *testing.T) {
		s, r := New()
		done := make(chan int, R+S)
		ctx := context.Background()

		for i := 0; i < R; i++ {
			go func() {
				var cur int
				for r.Wait() {
					cur++
				}
				done <- cur
			}()
		}

		for i := 0; i < S; i++ {
			go func() {
				var cur int
				for !s.IsClosed() {
					n := rand.Intn(R) + 1
					cnt, _ := s.SignalWithCtx(ctx, n)
					cur -= cnt
				}
				done <- cur
			}()
		}
		time.Sleep(time.Second * 5)
		s.Close()
		var total int

		for i := 0; i < S+R; i++ {
			total += <-done
		}
		equal(t, 0, total)
	})

	t.Run("Cancel", func(t *testing.T) {
		s, r := New()
		defer s.Close()
		done := make(chan int, R+S)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for i := 0; i < R; i++ {
			go func() {
				var cur int
				for r.Wait() {
					cur++
				}
				done <- cur
			}()
		}

		for i := 0; i < S; i++ {
			go func() {
				var cur int
				for {
					n := rand.Intn(R) + 1
					cnt, err := s.SignalWithCtx(ctx, n)
					cur -= cnt
					if err != nil || s.IsClosed() {
						break
					}
				}
				done <- cur
			}()
		}
		time.Sleep(time.Second * 5)
		cancel()
		var total int

		for i := 0; i < S; i++ {
			total += <-done
		}
		s.Close()
		for i := 0; i < R; i++ {
			total += <-done
		}
		equal(t, 0, total)
	})

	t.Run("Deadline", func(t *testing.T) {
		s, r := New()
		defer s.Close()
		done := make(chan int, R+S)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		for i := 0; i < R; i++ {
			go func() {
				var cur int
				for r.Wait() {
					cur++
				}
				done <- cur
			}()
		}

		for i := 0; i < S; i++ {
			go func() {
				var cur int
				for {
					n := rand.Intn(R) + 1
					cnt, err := s.SignalWithCtx(ctx, n)
					cur -= cnt
					if err != nil || s.IsClosed() {
						break
					}
				}
				done <- cur
			}()
		}
		var total int

		for i := 0; i < S; i++ {
			total += <-done
		}
		s.Close()
		for i := 0; i < R; i++ {
			total += <-done
		}
		equal(t, 0, total)
	})
}

func TestSignallerBroadcastAndSignalWithCtx(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	const N = 10
	const runs = 1e7

	s, _ := New()
	done := make(chan bool, N)

	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < N; i++ {
		go func() {
			n, err := s.SignalWithCtx(ctx, 1)
			equal(t, 0, n)
			equal(t, context.Canceled, err)
			done <- true
		}()
	}

	for i := 0; i < runs; i++ {
		if i%1e4 == 0 {
			select {
			case <-done:
				t.Fatal("goroutine not asleep")
			default:
			}
		}
		s.Broadcast()
	}

	select {
	case <-done:
		t.Fatal("goroutine not asleep")
	default:
	}
	cancel()
	for i := 0; i < N; i++ {
		<-done
	}

	s.Close()
}

func TestSignallerBroadcastHeavy(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	const N = 8
	const runs = 5e6
	s, r := New()
	done := make(chan bool, N)

	for i := 0; i < N; i++ {
		go func() {
			for r.Wait() {
				select {
				case done <- true:
				default:
				}
			}
		}()
	}

	for i := 1; i <= runs; i++ {
		if i%1e4 == 0 {
			for j := 0; j < N; j++ {
				<-done
			}
		}
		s.Broadcast()
	}

	s.Close()
}
