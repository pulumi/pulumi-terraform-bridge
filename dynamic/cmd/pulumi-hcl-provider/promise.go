package main

import (
	"sync"
)

type promise[T any] struct {
	wg    sync.WaitGroup
	value T
}

func newPromise[T any]() *promise[T] {
	p := promise[T]{wg: sync.WaitGroup{}}
	p.wg.Add(1)
	return &p
}

func goPromise[T any](create func() T) *promise[T] {
	p := newPromise[T]()
	go func() {
		value := create()
		p.fulfill(value)
	}()
	return p
}

// Must be called only once, will panic if called twice.
func (p *promise[T]) fulfill(value T) {
	p.value = value
	p.wg.Done() // this panics if called twice
}

func (p *promise[T]) await() T {
	p.wg.Wait()
	return p.value
}
