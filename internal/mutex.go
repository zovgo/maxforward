package internal

import "sync"

type ValueWithMutex[T any] struct {
	sync.Mutex
	V T
}

func (v *ValueWithMutex[T]) Load() T {
	v.Lock()
	defer v.Unlock()
	return v.V
}

func (v *ValueWithMutex[T]) Store(x T) {
	v.Lock()
	defer v.Unlock()
	v.V = x
}

type ValueWithRWMutex[T any] struct {
	sync.RWMutex
	V T
}

func (v *ValueWithRWMutex[T]) Load() T {
	v.RLock()
	defer v.RUnlock()
	return v.V
}

func (v *ValueWithRWMutex[T]) Store(x T) {
	v.Lock()
	defer v.Unlock()
	v.V = x
}
