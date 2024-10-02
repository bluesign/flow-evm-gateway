package models

import (
	"fmt"
	"sync"
)

type Publisher[T any] struct {
	mux         sync.RWMutex
	subscribers map[Subscriber[T]]struct{}
}

func NewPublisher[T any]() *Publisher[T] {
	return &Publisher[T]{
		mux:         sync.RWMutex{},
		subscribers: make(map[Subscriber[T]]struct{}),
	}
}

func (p *Publisher[T]) Publish(data T) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	for s := range p.subscribers {
		s.Notify(data)
	}
}

func (p *Publisher[T]) Subscribe(s Subscriber[T]) {
	p.mux.Lock()
	defer p.mux.Unlock()

	p.subscribers[s] = struct{}{}
}

func (p *Publisher[T]) Unsubscribe(s Subscriber[T]) {
	p.mux.Lock()
	defer p.mux.Unlock()

	delete(p.subscribers, s)
}

type Subscriber[T any] interface {
	Notify(data T)
	Error() <-chan error
}

type Subscription[T any] struct {
	err      chan error
	callback func(data T) error
}

func NewSubscription[T any](callback func(T) error) *Subscription[T] {
	return &Subscription[T]{
		callback: callback,
		err:      make(chan error),
	}
}

func (b *Subscription[T]) Notify(data T) {
	err := b.callback(data)
	if err != nil {
		select {
		case b.err <- err:
		default:
			// TODO: handle this better!
			panic(fmt.Sprintf("failed to send error to subscription %v", err))
		}
	}
}

func (b *Subscription[T]) Error() <-chan error {
	return b.err
}
