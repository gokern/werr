package arena_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gokern/werr/v2/internal/arena"
)

type box struct {
	a, b, c uintptr
}

func TestTake_returnsZeroedPointer(t *testing.T) {
	t.Parallel()

	a := arena.New[box](16)

	for range 32 {
		p := a.Take()
		require.NotNil(t, p)
		require.Equal(t, box{}, *p, "Take must return a zeroed slot")
		// Mutate the slot — if the next Take ever returned the same slot
		// without resetting it, the next iteration's zero check would fail.
		p.a = 1
		p.b = 2
		p.c = 3
	}
}

func TestTake_distinctPointers(t *testing.T) {
	t.Parallel()

	const n = 256

	a := arena.New[box](32)

	seen := make(map[*box]struct{}, n)

	for range n {
		p := a.Take()
		_, dup := seen[p]
		require.False(t, dup, "arena returned a duplicate pointer")

		seen[p] = struct{}{}
	}
}

func TestTake_concurrent(t *testing.T) {
	t.Parallel()

	const goroutines = 16

	const perG = 200

	a := arena.New[box](64)

	var (
		mu   sync.Mutex
		seen = make(map[*box]struct{}, goroutines*perG)
		wg   sync.WaitGroup
	)

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			locals := make([]*box, 0, perG)
			for range perG {
				locals = append(locals, a.Take())
			}

			mu.Lock()
			defer mu.Unlock()

			for _, p := range locals {
				_, dup := seen[p]
				assert.False(t, dup)

				seen[p] = struct{}{}
			}
		}()
	}

	wg.Wait()

	require.Len(t, seen, goroutines*perG)
}

func TestTake_independentSlots(t *testing.T) {
	t.Parallel()

	a := arena.New[box](8)

	p1 := a.Take()
	p2 := a.Take()
	require.NotSame(t, p1, p2)

	p1.a = 0xAA
	p2.a = 0xBB

	require.Equal(t, uintptr(0xAA), p1.a)
	require.Equal(t, uintptr(0xBB), p2.a)
}

func TestNew_invalidSize(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { _ = arena.New[box](0) })
	require.Panics(t, func() { _ = arena.New[box](-1) })
	require.Panics(t, func() { _ = arena.New[box](-1024) })
}

// Take crossing a slab boundary still returns fresh slots.
func TestTake_pastSlabBoundary(t *testing.T) {
	t.Parallel()

	const slabSize = 4

	const totalCalls = slabSize * 5

	a := arena.New[box](slabSize)

	pointers := make([]*box, 0, totalCalls)
	for range totalCalls {
		pointers = append(pointers, a.Take())
	}

	seen := make(map[*box]struct{}, totalCalls)

	for _, p := range pointers {
		require.NotNil(t, p)
		_, dup := seen[p]
		require.False(t, dup)

		seen[p] = struct{}{}
	}
}
