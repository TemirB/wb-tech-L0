package kafka

import (
	"sync"
	"sync/atomic"
)

type Pool struct {
	jobs    chan func()
	wg      sync.WaitGroup
	closed  atomic.Bool
	closeCh chan struct{}
}

func NewPool(n int) *Pool {
	if n < 1 {
		n = 1
	}
	p := &Pool{
		jobs:    make(chan func(), n*2),
		closeCh: make(chan struct{}),
	}
	p.wg.Add(n)
	for i := 0; i < n; i++ {
		go p.worker()
	}
	return p

}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case f, ok := <-p.jobs:
			if !ok {
				return
			}
			if f != nil {
				f()
			}
		case <-p.closeCh:
			return
		}
	}
}

func (p *Pool) Submit(f func()) {
	if p.closed.Load() {
		return
	}
	select {
	case p.jobs <- f:
	case <-p.closeCh:
	}
}

func (p *Pool) Close() {
	if p.closed.Swap(true) {
		return
	}
	close(p.closeCh)
	close(p.jobs)
}

func (p *Pool) Wait() {
	p.wg.Wait()
}
