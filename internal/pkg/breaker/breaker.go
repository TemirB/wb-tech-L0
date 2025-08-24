package breaker

import (
	"errors"
	"sync"
	"time"

	"github.com/TemirB/wb-tech-L0/internal/config"
)

var ErrOpenState = errors.New("circuit breaker is open")

type State uint8

const (
	Closed State = iota
	Open
	HalfOpen
)

type Breaker struct {
	mu           sync.RWMutex
	cfg          config.Breaker
	state        State
	failCount    uint32
	lastOpenTime time.Time
	halfOpenReq  uint32
}

func New(cfg config.Breaker) *Breaker {
	return &Breaker{
		cfg:   cfg,
		state: Closed,
	}
}

func (b *Breaker) Allow() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	switch b.state {
	case Closed:
		return nil
	case Open:
		if time.Since(b.lastOpenTime) < b.cfg.OpenTimeout {
			return ErrOpenState
		}
	case HalfOpen:
		if b.halfOpenReq >= uint32(b.cfg.MaxHalfOpen) {
			return ErrOpenState
		}
	}

	b.mu.RUnlock()
	b.mu.Lock()
	defer b.mu.RLock()
	defer b.mu.Unlock()

	switch b.state {
	case Open:
		if time.Since(b.lastOpenTime) >= b.cfg.OpenTimeout {
			b.state = HalfOpen
			b.halfOpenReq = 0
			return nil
		}
		return ErrOpenState
	case HalfOpen:
		if b.halfOpenReq < b.cfg.MaxHalfOpen {
			b.halfOpenReq++
			return nil
		}
		return ErrOpenState
	default:
		return nil
	}
}

func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case HalfOpen:
		b.state = Closed
		b.failCount = 0
	case Closed:
		b.failCount = 0
	}
}

func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		b.failCount++
		if b.failCount >= b.cfg.Threshold {
			b.state = Open
			b.lastOpenTime = time.Now()
		}
	case HalfOpen:
		b.state = Open
		b.lastOpenTime = time.Now()
	}
}

func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}
