// Package event contains domain events.
package event

// Event domain event interface.
type Event interface {
	Name() string
	IsAsynchronous() bool
}

type baseEvent struct {
	name            string
	isAsynchronious bool
}

func (e baseEvent) Name() string {
	return e.name
}

func (e baseEvent) IsAsynchronous() bool {
	return e.isAsynchronious
}

// Handler interface.
type Handler interface {
	Notify(event Event)
}

// Publisher notifies subscribers by events.
type Publisher struct {
	handlers map[string][]Handler
}

// NewPublisher constructor.
func NewPublisher() *Publisher {
	return &Publisher{
		handlers: make(map[string][]Handler),
	}
}

// NotifyAll notifies all handlers.
func (e *Publisher) NotifyAll(event Event) {
	if event.IsAsynchronous() {
		go e.notify(event)
		return
	}

	e.notify(event)
}

func (e *Publisher) notify(event Event) {
	for _, handler := range e.handlers[event.Name()] {
		handler.Notify(event)
	}
}

// Subscribe subscribes handler for specified events.
func (e *Publisher) Subscribe(handler Handler, events ...Event) {
	for _, event := range events {
		handlers := e.handlers[event.Name()]
		handlers = append(handlers, handler)
		e.handlers[event.Name()] = handlers
	}
}
