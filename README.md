# Wake - wake goroutines and broadcast

## Features
No mutexes, no internal lists for waiters. Fast and simple API (subject to change till v1.0.0)

- `Signaller`:
    - Wake N goroutines (Signal)
    - Wake exactly N goroutines with context (SignalWithCtx)
    - Wake all goroutines (Broadcast)
    - Check how many goroutines are currently waiting (WaitCount)
- `Receiver`:
    - Wait with/without context (Wait/WaitWithCtx)
- `Cond`: same as sync.Cond, but with better API
- `RWCond`: same as Cond, but uses RLock/RUnlock instead of Lock/Unlock

## Usage
```
go get github.com/nursik/wake
```

```go
import "context"
import "fmt"
import "github.com/nursik/wake"

func Wait(r *wake.Receiver, id int){
    // r.Wait() blocks until it awoken. Returns true if it was awoken or false if it signaller was closed
    for r.Wait() {
        fmt.Printf("%v: received signal or broadcast\n", id)
    } 
    fmt.Printf("%v: signaller is closed\n", id)
}

func main(){
    s, r := New()
    defer s.Close()

    go Wait(r, 1)

    // Wait may start after Signal
    // time.Sleep(time.Microsecond)

    _ = s.Signal(1) // wakes id 1
    go Wait(r, 2)
    
    // ...

    s.Broadcast() // wakes id 1 and 2

    // Blocks until context is cancelled, signaller is closed or wakes N goroutines
    _, _ = s.SignalWithCtx(context.Background(), 1) // waked id 1 or 2 
}
```