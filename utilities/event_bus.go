package utilities

import "sync"

type EventHandler func(interface{})

type EventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

func (eb *EventBus) Subscribe(event string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers[event] = append(eb.handlers[event], handler)
}

func (eb *EventBus) Publish(event string, data interface{}) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	if handlers, found := eb.handlers[event]; found {
		for _, handler := range handlers {
			go handler(data) // Run handlers asynchronously
		}
	}
}

// Global instance
var GlobalEventBus = NewEventBus()
