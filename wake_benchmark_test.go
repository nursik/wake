package wake_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"

	. "github.com/nursik/wake"
)

func BenchmarkBroadcast(b *testing.B) {
	s, _ := New()

	for i := 0; i < b.N; i++ {
		s.Broadcast()
	}
}

func BenchmarkBroadcast1024Waiters(b *testing.B) {
	b.StopTimer()
	s, r := New()
	for i := 0; i < 1024; i++ {
		go func() {
			for r.Wait() {

			}
		}()
	}
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for s.WaitCount() != 1024 {
		}
		b.StartTimer()
		s.Broadcast()
	}
	s.Close()
}

func BenchmarkBroadcastParallel(b *testing.B) {
	cpus := runtime.GOMAXPROCS(0)
	waiters := []int{1, cpus, cpus * 3}

	for _, w := range waiters {
		b.Run(fmt.Sprintf("waiters=%v", w), func(b *testing.B) {
			s, r := New()
			defer s.Close()
			for k := 0; k < w; k++ {
				go func() {
					for r.Wait() {
					}
				}()
			}
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(cpus)
			for k := 0; k < cpus; k++ {
				go func() {
					<-start
					for i := 0; i < b.N; i++ {
						s.Broadcast()
					}
					wg.Done()
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

func BenchmarkSignal1(b *testing.B) {
	s, _ := New()

	for i := 0; i < b.N; i++ {
		s.Signal(1)
	}
}

func BenchmarkSignal1Parallel(b *testing.B) {
	cpus := runtime.GOMAXPROCS(0)
	waiters := []int{1, cpus, cpus * 3}

	for _, w := range waiters {
		b.Run(fmt.Sprintf("waiters=%v", w), func(b *testing.B) {
			s, r := New()
			defer s.Close()
			for k := 0; k < w; k++ {
				go func() {
					for r.Wait() {
					}
				}()
			}
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(cpus)
			for k := 0; k < cpus; k++ {
				go func() {
					<-start
					for i := 0; i < b.N; i++ {
						s.Signal(1)
					}
					wg.Done()
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

func BenchmarkSignalWithCtx1(b *testing.B) {
	s, r := New()
	defer s.Close()

	go func() {
		for r.Wait() {
		}
	}()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		s.SignalWithContext(ctx, 1)
	}
}

func BenchmarkSignalWithCtx1Parallel(b *testing.B) {
	cpus := runtime.GOMAXPROCS(0)
	waiters := []int{1, cpus, cpus * 3}

	for _, w := range waiters {
		b.Run(fmt.Sprintf("waiters=%v", w), func(b *testing.B) {
			s, r := New()
			defer s.Close()
			for k := 0; k < w; k++ {
				go func() {
					for r.Wait() {
					}
				}()
			}
			ctx := context.Background()
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(cpus)
			for k := 0; k < cpus; k++ {
				go func() {
					<-start
					for i := 0; i < b.N; i++ {
						s.SignalWithContext(ctx, 1)
					}
					wg.Done()
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

func BenchmarkWait(b *testing.B) {
	s, r := New()
	go func() {
		ctx := context.Background()
		for !s.IsClosed() {
			s.SignalWithContext(ctx, 1)
		}
	}()
	for i := 0; i < b.N; i++ {
		r.Wait()
	}
	s.Close()
}
