package pool

import "sync"

type Pool struct {
	jobs chan func()
	wg   sync.WaitGroup
}

func New(n int) *Pool {
	if n < 1 {
		n = 1
	}
	p := &Pool{
		jobs: make(chan func(), n*2),
	}
	p.wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer p.wg.Done()
			for f := range p.jobs {
				if f != nil {
					f()
				}
			}
		}()
	}
	return p

}
func (p *Pool) Submit(f func()) {
	p.jobs <- f
}

func (p *Pool) Close() {
	close(p.jobs)
}

func (p *Pool) Wait() {
	p.wg.Wait()
}
