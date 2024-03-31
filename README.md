# Wake - wake goroutines and broadcast
[![Go Reference](https://pkg.go.dev/badge/github.com/nursik/wake.svg)](https://pkg.go.dev/github.com/nursik/wake)
[![Go Report Card](https://goreportcard.com/badge/github.com/nursik/wake)](https://goreportcard.com/report/github.com/nursik/wake)

Go library to wake goroutines. Thread-safe, small memory footprint, fast and simple [API](https://pkg.go.dev/github.com/nursik/wake).

The library was written primarily as a dependency for safer [sync.Cond](https://github.com/nursik/go-cond).

You also might check [go-broadcast](https://github.com/nursik/go-broadcast) to send/broadcast messages of any type and unlike this library you are able to not miss these messages/signals.

## Features
- `Signaller`:
    - Wake N goroutines (Signal)
    - Wake exactly N goroutines with context (SignalWithContext)
    - Wake all goroutines (Broadcast)
    - Check how many goroutines are currently waiting (WaitCount)
	- Close signaller to unblock goroutines
- `Receiver`:
    - Wait with/without context (Wait/WaitWithContext). Receiver can check, if it was awoken due signal or context cancellation/closing.

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
	// r.Wait() blocks until it is awoken. Returns true if it was awoken by signal or false if signaller was closed
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

	// Wait may start after Signal, sleep a little bit
	time.Sleep(time.Microsecond)

	_ = s.Signal(1) // wakes id 1 OR 2

	// Wait may start after Signal
	time.Sleep(time.Microsecond)

	s.Broadcast() // wakes id 1 and 2

	// Blocks until context is cancelled, signaller is closed or wakes 1 goroutine
	_, _ = s.SignalWithContext(context.Background(), 1) // wakes id 1 or 2
}
```