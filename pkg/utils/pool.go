package utils

import (
	"sync"
)

type pool[T any] struct {
	pool sync.Pool
}

func New[T any](fn func() T) pool[T] {
	return pool[T]{
		pool: sync.Pool{New: func() any { return fn() }},
	}
}

func NewPool[T any]() pool[T] {
	return New[T](func() T {
		var t T
		return t
	})
}

func NewPoolMake[P *T, T any]() pool[P] {
	return New[P](func() P {
		return new(T)
	})
}

func (p *pool[T]) Get() T {
	return p.pool.Get().(T)
}

func (p *pool[T]) Put(x T) {
	p.pool.Put(x)
}
