package tools

import "sync"

/*
OrderSemaphore is used to prevent multiple action when the conditions to send an order are met
by the strategy.
*/
type OrderSemaphore struct {
	mutex *sync.RWMutex
	count int
}

// NewOrderSemaphore is the OrderSemaphore constructor.
func NewOrderSemaphore() *OrderSemaphore {
	return &OrderSemaphore{mutex: &sync.RWMutex{}}
}

// OrderSent will make Try return false until a response is Notified.
func (s *OrderSemaphore) OrderSent() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.count++
}

// Notify that a order fill/error response has been received.
func (s *OrderSemaphore) Notify() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.count > 0 {
		s.count--
	}
}

// Try returns true if and order can be sent, false otherwise.
func (s *OrderSemaphore) Try() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.count != 0
}
