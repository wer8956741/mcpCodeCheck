package pkg

import "sync/atomic"

type AtomicBool struct {
	b atomic.Value
}

func NewAtomicBool() *AtomicBool {
	b := &AtomicBool{}
	b.b.Store(false)
	return b
}

func (b *AtomicBool) Store(value bool) {
	b.b.Store(value)
}

func (b *AtomicBool) Load() bool {
	return b.b.Load().(bool)
}

type AtomicString struct {
	b atomic.Value
}

func NewAtomicString() *AtomicString {
	b := &AtomicString{}
	b.b.Store("")
	return b
}

func (b *AtomicString) Store(value string) {
	b.b.Store(value)
}

func (b *AtomicString) Load() string {
	return b.b.Load().(string)
}
