package worker

import (
	"sync"
	"time"
)

type CircuitBreaker struct {
	failureCount map[int]int
	failureTime  map[int]time.Time
	mu           sync.RWMutex
	maxFailures  int
	resetTimeout time.Duration
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureCount: make(map[int]int),
		failureTime:  make(map[int]time.Time),
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) RecordFailure(deviceID int) int {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount[deviceID]++
	count := cb.failureCount[deviceID]

	if count == cb.maxFailures {
		cb.failureTime[deviceID] = time.Now()
	}

	return count
}

func (cb *CircuitBreaker) IsBroken(deviceID int) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	count := cb.failureCount[deviceID]
	if count < cb.maxFailures {
		return false
	}

	brokenAt := cb.failureTime[deviceID]
	if time.Since(brokenAt) > cb.resetTimeout {
		// Half-Open state (resetting so we can try polling again)
		delete(cb.failureCount, deviceID)
		delete(cb.failureTime, deviceID)
		return false
	}

	return true
}

func (cb *CircuitBreaker) Reset(deviceID int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.failureCount, deviceID)
	delete(cb.failureTime, deviceID)
}

func (cb *CircuitBreaker) GetFailures(deviceID int) int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount[deviceID]
}

func (cb *CircuitBreaker) GetBrokenCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	broken := 0
	for _, count := range cb.failureCount {
		if count >= cb.maxFailures {
			broken++
		}
	}
	return broken
}
