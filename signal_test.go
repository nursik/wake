package wake_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	. "github.com/nursik/wake"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNew(t *testing.T) {
	s, r := New()
	require.False(t, s.IsClosed())
	require.False(t, r.IsClosed())
	require.NotNil(t, s)
	require.NotNil(t, r)
	require.Equal(t, 0, s.WaitCount())
}

func TestSignallerBroadcast(t *testing.T) {
	const N = 1000
	s, r := New()
	awake := make(chan bool, N)

	s.Broadcast()

	for i := 0; i < N; i++ {
		go func() {
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
	s.Broadcast()
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

	s, r := New()
	awake := make(chan bool, N+M)
	for i := 0; i < N+M; i++ {
		go func() {
			r.Wait()
			awake <- true
		}()
	}
	for s.WaitCount() != N+M {
	}
	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	require.Equal(t, N, s.Signal(N))

	for i := 0; i < N; i++ {
		<-awake
	}

	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	require.Equal(t, M, s.Signal(M+1))

	for i := 0; i < M; i++ {
		<-awake
	}

	require.Equal(t, 0, s.Signal(1))

	select {
	case <-awake:
		t.Fatal("goroutine not asleep")
	default:
	}

	const S = 10
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
			require.Equal(t, N, s.Signal(N))
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
			require.True(t, ok)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		cnt, err := s.SignalWithCtx(context.Background(), 1)
		require.Equal(t, 1, cnt)
		require.Equal(t, nil, err)
		<-awake

		cnt, err = s.SignalWithCtx(context.Background(), -1)
		require.Equal(t, 0, cnt)
		require.Equal(t, nil, err)

		s.Close()
		cnt, err = s.SignalWithCtx(context.Background(), 1)
		require.Equal(t, 0, cnt)
		require.Equal(t, nil, err)
	})
	t.Run("Deadline", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)

		go func() {
			ok := r.Wait()
			require.True(t, ok)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		cnt, err := s.SignalWithCtx(context.Background(), 1)
		require.Equal(t, 1, cnt)
		require.Equal(t, nil, err)
		<-awake

		ctx1, cancel1 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel1()
		cnt, err = s.SignalWithCtx(ctx1, 1)
		require.Equal(t, 0, cnt)
		require.Equal(t, context.DeadlineExceeded, err)
		s.Close()

		ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel2()
		cnt, err = s.SignalWithCtx(ctx2, 1)
		require.Equal(t, 0, cnt)
		require.Equal(t, nil, err)
	})

	t.Run("Cancel", func(t *testing.T) {
		s, r := New()
		done := make(chan struct{}, 2)
		ctx1, cancel1 := context.WithCancel(context.Background())

		go func() {
			cnt, err := s.SignalWithCtx(ctx1, 1)
			require.Equal(t, context.Canceled, err)
			require.Equal(t, 0, cnt)
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
			require.Equal(t, nil, err)
			require.Equal(t, 1, cnt)
			done <- struct{}{}
		}()

		r.Wait()
		<-done

		ctx3, cancel3 := context.WithCancel(context.Background())
		defer cancel3()

		go func() {
			done <- struct{}{}
			cnt, err := s.SignalWithCtx(ctx3, 1)
			require.Equal(t, nil, err)
			require.Equal(t, 0, cnt)
			done <- struct{}{}
		}()
		<-done
		s.Close()
		<-done

		cnt, err := s.SignalWithCtx(ctx2, 1)
		require.Equal(t, 0, cnt)
		require.Equal(t, nil, err)
	})
}

func TestSignallerClose(t *testing.T) {
	const N = 10

	t.Run("Basic", func(t *testing.T) {
		s, r := New()

		require.True(t, s.Close())
		require.False(t, s.Close())
		require.True(t, s.IsClosed())
		require.True(t, r.IsClosed())
		require.Equal(t, 0, s.WaitCount())
		require.Equal(t, 0, s.Signal(1))
		cnt, err := s.SignalWithCtx(context.Background(), N)
		require.Equal(t, 0, cnt)
		require.Nil(t, err)
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
					require.False(t, ok)
				} else {
					ok, err := r.WaitWithCtx(context.Background())
					require.False(t, ok)
					require.Nil(t, err)
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

		require.Equal(t, 0, s.WaitCount())
	})

	t.Run("UnblocksSignalWithCtx", func(t *testing.T) {
		s, _ := New()
		done := make(chan bool)
		go func() {
			cnt, err := s.SignalWithCtx(context.Background(), N)
			require.Equal(t, 0, cnt)
			require.Nil(t, err)
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

		require.Equal(t, N/2, s.Signal(N/2))

		for i := 0; i < N/2; i++ {
			require.True(t, <-done)
		}
		select {
		case <-done:
			t.Fatal("goroutine not asleep")
		default:
		}

		s.Close()
		for i := 0; i < N-N/2; i++ {
			require.False(t, <-done)
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
	require.True(t, <-awake)

	s.Close()
	require.False(t, r.Wait())
}

func TestReceiverWaitWithCtx(t *testing.T) {
	t.Run("Background", func(t *testing.T) {
		s, r := New()
		awake := make(chan bool, 1)

		go func() {
			ok, err := r.WaitWithCtx(context.Background())
			require.Nil(t, err)
			awake <- ok
		}()

		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		// Using with context, because sometimes it is too fast and signals before goroutine starts waiting
		s.SignalWithCtx(context.Background(), 1)
		require.True(t, <-awake)

		s.Close()
		ok, err := r.WaitWithCtx(context.Background())
		require.False(t, ok)
		require.Nil(t, err)
	})
	t.Run("Deadline", func(t *testing.T) {
		s, r := New()
		ctx1, cancel1 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel1()
		ok, err := r.WaitWithCtx(ctx1)
		require.True(t, ok)
		require.Equal(t, context.DeadlineExceeded, err)

		s.Close()
		ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
		defer cancel2()

		ok, err = r.WaitWithCtx(ctx2)
		require.False(t, ok)
		require.Nil(t, err)
	})

	t.Run("Cancel", func(t *testing.T) {
		s, r := New()
		done := make(chan struct{}, 2)
		ctx1, cancel1 := context.WithCancel(context.Background())

		go func() {
			ok, err := r.WaitWithCtx(ctx1)
			require.True(t, ok)
			require.Equal(t, context.Canceled, err)
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

		go func() {
			ok, err := r.WaitWithCtx(ctx2)
			require.False(t, ok)
			require.Nil(t, err)
			done <- struct{}{}
		}()

		cancel2()
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
	require.Equal(t, 0, total)
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
		require.Equal(t, 0, total)
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
		require.Equal(t, 0, total)
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
		require.Equal(t, 0, total)
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
			require.Equal(t, 0, n)
			require.Equal(t, context.Canceled, err)
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
