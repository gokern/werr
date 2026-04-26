// Package arena provides a slab-pooled allocator for fixed-size types.
//
// Each Take returns a *T drawn from a per-P sync.Pool of slabs; new heap
// allocations only happen when an existing slab is exhausted, so the
// per-call cost is amortised across slabSize calls. Pointers handed out
// are never reused; each slot is written once.
//
// Lifetime: a slab stays alive as long as any *T drawn from it remains
// reachable, because Go's GC keeps the underlying array alive through
// the live pointer. sync.Pool may drop a slab under GC pressure; the
// next Take allocates a fresh one. There is no use-after-free hazard.
//
// There is no Release(*T): pointers stay live as long as callers retain
// them. werr.Error has unbounded lifetime (errors live as long as the
// user keeps them), so a free-list design is incompatible with the
// API contract.
//
// sync.Pool is concurrency-safe; Arena inherits that without extra locking.
package arena

import "sync"

// Arena is a slab-pooled allocator for values of type T. Construct one
// per type via [New]; share it freely across goroutines.
type Arena[T any] struct {
	slabSize int
	pool     sync.Pool
}

type slab[T any] struct {
	buf []T
	idx int
}

// New constructs an Arena that allocates slabs of slabSize elements.
// A larger slabSize amortises per-call cost over more Take()s but holds
// more memory live per slab; 1024 is a reasonable default for 24–48 byte
// payloads.
//
// Panics if slabSize <= 0 (Take would otherwise spin forever).
func New[T any](slabSize int) *Arena[T] {
	if slabSize <= 0 {
		panic("arena: slabSize must be positive")
	}

	return &Arena[T]{
		slabSize: slabSize,
		pool: sync.Pool{New: func() any {
			return &slab[T]{buf: make([]T, slabSize), idx: 0}
		}},
	}
}

// Take returns a pointer to a fresh, zeroed slot from a pooled slab.
// Allocation only happens when the current slab is exhausted (the slow
// path). Splitting the slow path out keeps Take's inline cost under the
// compiler's budget so it inlines into newError, which in turn inlines
// into werr.Wrap on the hot path.
func (a *Arena[T]) Take() *T {
	s, _ := a.pool.Get().(*slab[T])
	if s.idx < len(s.buf) {
		ptr := &s.buf[s.idx]
		s.idx++
		a.pool.Put(s)

		return ptr
	}

	return a.takeSlow()
}

// takeSlow handles the rare case where the pooled slab is exhausted.
// Allocation budget: one slab per slabSize Take calls in steady state.
func (a *Arena[T]) takeSlow() *T {
	for {
		s, _ := a.pool.Get().(*slab[T])
		if s.idx < len(s.buf) {
			ptr := &s.buf[s.idx]
			s.idx++
			a.pool.Put(s)

			return ptr
		}
		// Drop the exhausted slab; GC frees the underlying array once
		// every *T handed out from it has been collected. Loop to pick
		// up a fresh slab from the pool (or pool.New).
	}
}
