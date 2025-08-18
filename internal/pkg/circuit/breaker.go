package circuit

import (
	"errors"
	"sync"
	"time"
)

var ErrOpen = errors.New("circuit open")

type State int

const (
	Closed   State = iota // Ok, normal behavior
	Open                  // Open the breaker, do not allow requests until the timeout passes
	HalfOpen              // Half-open state, with trial requests
)

// Breaker implements the Circuit Breaker.
// After 'threshold' errors in Closed state, it opens the circuit.
// In Open state, it blocks all requests for 'halfOpenAfter' duration.
// In HalfOpen state, it allows up to 'maxHalfOpen' trial requests.
// Requires explicit Success()/Failure() calls to report outcomes.
type Breaker struct {
	mu                 sync.Mutex    // Concurrency Security
	state              State         // Current state of Breaker
	errs, threshold    int           // Current and allowed numbers of errors
	halfOpenAfter      time.Duration // Wait time before transitioning from Open to HalfOpen
	lastChange         time.Time     //	Time of last state change
	trial, maxHalfOpen int           // Current and allowed numbers of trial request

	// This fileds, only for merics
	totalSuccess uint64
	totalFailure uint64

	epochSuccess uint64
	epochFailure uint64

	lastSuccess time.Time
	lastFailure time.Time
}

func New(threshold int, wait time.Duration, maxHalfOpen int) *Breaker {
	return &Breaker{
		state:         Closed,
		threshold:     threshold,
		halfOpenAfter: wait,
		lastChange:    time.Now(),
		maxHalfOpen:   maxHalfOpen,
	}
}

// Allow checks if a request is permitted.
// Returns ErrOpen if circuit is open or half-open trial limit reached.
// Automatically transitions Open→HalfOpen after timeout.
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	switch b.state {
	case Open:
		if now.Sub(b.lastChange) >= b.halfOpenAfter {
			b.transitionTo(now, HalfOpen)
			return nil
		}
		return ErrOpen
	case HalfOpen:
		if b.trial >= b.maxHalfOpen {
			return ErrOpen
		}
		b.trial++
		return nil
	default: // Closed
		return nil
	}
}

// Success reports a successful operation.
// Resets error count and transitions HalfOpen→Closed.
// Also resets errors in Closed state (optional, but safe).
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.totalSuccess++
	b.epochSuccess++
	b.lastSuccess = now

	switch b.state {
	case HalfOpen:
		b.transitionTo(now, Closed)
	case Closed:
		b.errs = 0
	case Open:
		// Обычно невозможно, и нужно логировать такое событие и разбираться в причинах, возможно настроить алертик
	}
}

// Failure reports a failed operation.
// Triggers Closed→Open transition if error threshold reached.
// Immediate HalfOpen→Open transition on any failure.
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.totalFailure++
	b.epochFailure++
	b.lastChange = now

	switch b.state {
	case HalfOpen:
		b.transitionTo(now, Open)
	case Closed:
		b.errs++
		if b.errs >= b.threshold {
			b.transitionTo(now, Open)
		}
	case Open:
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *Breaker) transitionTo(now time.Time, next State) {
	b.state = next
	b.lastChange = now

	b.epochFailure = 0
	b.epochSuccess = 0

	b.trial = 0
	if next == Closed {
		b.errs = 0
	}
}

// TODO: Add metrics support
// ready: epoch and total variables
