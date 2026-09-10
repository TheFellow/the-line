package solver

import (
	"sync"
	"time"
)

// WebAssembly shares the browser event loop. Gosched alone cannot release it
// while Go has runnable goroutines; a short timer lets input and painting run.
// This only schedules work and never changes the numerical candidate order.
var browserYield struct {
	sync.Mutex
	last time.Time
}

func cooperate() {
	browserYield.Lock()
	due := time.Since(browserYield.last) >= 8*time.Millisecond
	if due {
		browserYield.last = time.Now()
	}
	browserYield.Unlock()
	if due {
		time.Sleep(time.Millisecond)
	}
}
