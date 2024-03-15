# Wake/cond - like sync.Cond with better functionality 
[![Go Reference](https://pkg.go.dev/badge/github.com/nursik/cond/wake.svg)](https://pkg.go.dev/github.com/nursik/wake/cond)

## Quickstart
Wake/cond package provides Cond and RWCond with better API such as signalling and waiting with contexts and close method. Slower than sync.Cond ~3 times (used sync/cond_test.go, which benchmarks only broadcast).

## Features
- Signal and get how many goroutines were awoken
- Signal exactly N goroutines with context and in case of context cancellation or closed Cond/RWCond get how many goroutines were awoken
- Broadcast
- Wait till Cond/RWCond is closed or Broadcast/Signal is called
- Wait with context - wake after Broadcast/Signal is called, Cond/RWCond is closed or context is cancelled
- Check hpw many goroutines are waiting
- Use RLock/RUnlock with RWCond
- Close Cond/RWCond so no waiting goroutines will hang up
## Usage
```
go get github.com/nursik/wake
```

```go
import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nursik/wake/cond"
)

func Wait(c *cond.Cond, mp map[int]int, key int) {
	c.L.Lock()
	for {
		value, ok := mp[key]
		if ok {
			fmt.Printf("Found %v at key %v\n", key, value)
			break
		}
		// Cond.Wait returns false only if Cond was closed.
		if !c.Wait() {
			fmt.Printf("Not foundd key %v\n", key)
			break
		}
	}
	c.L.Unlock()

	fmt.Printf("Key %v checker exit\n", key)
}

func main() {
	var locker sync.Mutex
	mp := make(map[int]int)

	c := cond.New(&locker)
	defer c.Close()

	go Wait(c, mp, 1)
	go Wait(c, mp, 2)
	go Wait(c, mp, 3)

	locker.Lock()
	mp[1] = 2
	locker.Unlock()

	c.Broadcast()

	locker.Lock()
	mp[2] = 4
	locker.Unlock()

	c.Broadcast()

	locker.Lock()
	mp[3] = 6
	locker.Unlock()

	c.Signal(1)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()

	locker.Lock()
	// Nobody can wake. Unblocks with (true, context.DeadlineExceeded).
	notClosed, err := c.WaitWithCtx(ctx)
	fmt.Println(notClosed, err)
	locker.Unlock()
}
```