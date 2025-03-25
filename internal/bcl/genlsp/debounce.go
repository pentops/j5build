package genlsp

import (
	"context"
	"sync"
	"time"
)

type debounce[T any] struct {
	timer     *time.Timer
	duration  time.Duration
	process   func(context.Context, T)
	logicLock sync.Mutex
}

func newDebounce[T any](duration time.Duration, process func(context.Context, T)) *debounce[T] {
	return &debounce[T]{
		duration: duration,
		process:  process,
	}
}

func (d *debounce[T]) request(ctx context.Context, t T) {
	d.logicLock.Lock()
	defer d.logicLock.Unlock()
	if d.timer != nil {
		d.timer.Reset(d.duration)
		return
	}
	d.timer = time.AfterFunc(d.duration, func() {
		d.logicLock.Lock()
		defer d.logicLock.Unlock()
		d.timer = nil
		go func() {
			d.process(ctx, t)
		}()
	})
}
