package utils

import (
	"iter"
	"sync"
	"sync/atomic"
)

type WorkerPool[J any, R any] struct {
	workers int

	i       atomic.Int64
	working sync.Once
	work    func(<-chan J, func(R))

	closed    bool
	jobs      chan J
	responses chan Response[R]
}
type Response[R any] struct {
	I        int
	WorkerID int
	Response R
}

// NewWorkerPool creates a new worker pool with the given number of workers.
// The channels are buffered to the number of workers.
// The work function should use the channel to receive jobs, and use the callback function to send responses.
// WorkerPool.Work can be called concurrently.
func NewWorkerPool[J any, R any](workers int, work func(<-chan J, func(R))) WorkerPool[J, R] {
	return WorkerPool[J, R]{workers: workers, work: work, jobs: make(chan J, workers), responses: make(chan Response[R], workers)}
}

// Cap returns the capacity of the worker pool.
func (p *WorkerPool[_, _]) Cap() int { return p.workers }

// Closed returns true if the response channel is closed.
func (p *WorkerPool[_, _]) Closed() bool { return p.closed }

func (p *WorkerPool[_, R]) Work() <-chan Response[R] {
	p.working.Do(p.do)
	return p.responses
}

func (p *WorkerPool[_, R]) do() {
	var workSet sync.WaitGroup
	workSet.Add(p.workers)
	for id := range p.workers {
		go func() {
			defer workSet.Done()
			p.work(p.jobs, func(r R) {
				p.responses <- Response[R]{
					I:        int(p.i.Add(1) - 1),
					WorkerID: id,
					Response: r,
				}
			})
		}()
	}

	go func() {
		workSet.Wait()
		close(p.responses)
		p.closed = true
	}()
}

// Add adds jobs to the worker pool. It blocks if the pool is full.
func (p *WorkerPool[J, _]) Add(j ...J) {
	for _, j := range j {
		p.jobs <- j
	}
}

// AddIter adds jobs to the worker pool from an iterator. It blocks if the pool is full.
func (p *WorkerPool[J, _]) AddIter(j iter.Seq[J]) {
	for j := range j {
		p.jobs <- j
	}
}

// AddAndClose adds jobs to the worker pool and closes it after all jobs are added.
func (p *WorkerPool[J, _]) AddAndClose(j ...J) {
	go func() {
		for _, j := range j {
			p.jobs <- j
		}
		p.Close()
	}()
}

// AddAndCloseIter adds jobs to the worker pool from an iterator and closes it after all jobs are added.
func (p *WorkerPool[J, _]) AddAndCloseIter(j iter.Seq[J]) {
	go func() {
		for j := range j {
			p.jobs <- j
		}
		p.Close()
	}()
}

// Close closes the worker pool. It should be called after all jobs are added.
func (p *WorkerPool[_, _]) Close() {
	close(p.jobs)
}

// Iter returns an iterator that yields the results from the worker pool.
// It returns the index of the result and the result itself.
func (p *WorkerPool[_, R]) Iter() iter.Seq2[int, R] {
	return func(yield func(int, R) bool) {
		for r := range p.responses {
			if !yield(r.I, r.Response) {
				return
			}
		}
	}
}
