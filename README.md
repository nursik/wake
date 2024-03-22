# Wake - wake goroutines and broadcast
[![Go Reference](https://pkg.go.dev/badge/github.com/nursik/wake.svg)](https://pkg.go.dev/github.com/nursik/wake)
[![Go Report Card](https://goreportcard.com/badge/github.com/nursik/wake)](https://goreportcard.com/report/github.com/nursik/wake)
## Features
Thread-safe, small memory footprint (no internal lists for waiters), no mutexes. Fast and simple [API](https://pkg.go.dev/github.com/nursik/wake) (subject to change till v1.0.0)

- `Signaller`:
    - Wake N goroutines (Signal)
    - Wake exactly N goroutines with context (SignalWithCtx)
    - Wake all goroutines (Broadcast)
    - Check how many goroutines are currently waiting (WaitCount)
- `Receiver`:
    - Wait with/without context (Wait/WaitWithCtx)
- `cond` submodule 
	- `cond/Cond`: same as sync.Cond, but with better [API](https://pkg.go.dev/github.com/nursik/wake/cond)
	- `cond/RWCond`: same as Cond, but uses RLock/RUnlock instead of Lock/Unlock

## Usage
```
go get github.com/nursik/wake
```

```go

import (
	"context"
	"fmt"
	"time"

	"github.com/nursik/wake"
)

func Wait(r *wake.Receiver, id int) {
	// r.Wait() blocks until it awoken. Returns true if it was awoken or false if signaller was closed
	for r.Wait() {
		fmt.Printf("%v: received signal or broadcast\n", id)
	}
	fmt.Printf("%v: signaller is closed\n", id)
}

func main() {
	s, r := wake.New()
	defer s.Close()

	go Wait(r, 1)
	go Wait(r, 2)

	// Wait may start after Signal
	time.Sleep(time.Microsecond)

	_ = s.Signal(1) // wakes id 1 OR 2

	// Wait may start after Signal
	time.Sleep(time.Microsecond)

	s.Broadcast() // wakes id 1 and 2

	// Blocks until context is cancelled, signaller is closed or wakes N goroutines
	_, _ = s.SignalWithCtx(context.Background(), 1) // wakes id 1 or 2
}
```