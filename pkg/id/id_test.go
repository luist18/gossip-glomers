package id

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- helpers -----------------------------------------------------------------

// concurrentSet is a sharded set optimized for high write concurrency.
type concurrentSet struct {
	shards []map[uint64]struct{}
	mu     []sync.Mutex
}

func newConcurrentSet(shards int) *concurrentSet {
	cs := &concurrentSet{
		shards: make([]map[uint64]struct{}, shards),
		mu:     make([]sync.Mutex, shards),
	}
	for i := range cs.shards {
		cs.shards[i] = make(map[uint64]struct{}, 4096)
	}
	return cs
}

func (cs *concurrentSet) add(v uint64) bool {
	i := int(v) & (len(cs.shards) - 1) // shards must be power-of-two for this mask
	cs.mu[i].Lock()
	_, exists := cs.shards[i][v]
	if !exists {
		cs.shards[i][v] = struct{}{}
	}
	cs.mu[i].Unlock()
	return !exists
}

func (cs *concurrentSet) len() int {
	n := 0
	for i := range cs.shards {
		cs.mu[i].Lock()
		n += len(cs.shards[i])
		cs.mu[i].Unlock()
	}
	return n
}

// waitUntilNextMillisecond blocks until the next Unix milli tick.
func waitUntilNextMillisecond() int64 {
	start := time.Now().UnixMilli()
	for {
		if now := time.Now().UnixMilli(); now != start {
			return now
		}
		runtime.Gosched()
	}
}

// generateBurst tries to issue n Generate() calls as tightly as possible.
// It returns the number of successfully recorded unique IDs and duplicate count.
func generateBurst(t *testing.T, g *SnowflakeGenerator, n int) (unique int, dups int) {
	t.Helper()
	cs := newConcurrentSet(1 << 8) // 256 shards
	wg := sync.WaitGroup{}
	wg.Add(n)

	// Release all goroutines together.
	start := make(chan struct{})

	var dupCount int64
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			id, _ := g.Generate()
			if !cs.add(uint64(id)) {
				atomic.AddInt64(&dupCount, 1)
			}
		}()
	}

	close(start)
	wg.Wait()
	return cs.len(), int(dupCount)
}

// generateMany spawns workers to produce total IDs across w goroutines.
func generateMany(t *testing.T, g *SnowflakeGenerator, w, total int) (unique int, dups int) {
	t.Helper()
	cs := newConcurrentSet(1 << 10) // 1024 shards
	wg := sync.WaitGroup{}
	wg.Add(w)

	var dupCount int64
	per := total / w
	extra := total % w

	for i := 0; i < w; i++ {
		n := per
		if i < extra {
			n++
		}
		go func(count int) {
			defer wg.Done()
			for j := 0; j < count; j++ {
				id, _ := g.Generate()
				if !cs.add(uint64(id)) {
					atomic.AddInt64(&dupCount, 1)
				}
			}
		}(n)
	}
	wg.Wait()
	return cs.len(), int(dupCount)
}

// --- tests -------------------------------------------------------------------

// High-concurrency uniqueness across a single generator.
func TestUniqueness_ConcurrentHeavy(t *testing.T) {
	g := NewSnowflakeGenerator(1)

	workers := max(2, int64(runtime.NumCPU()*4))
	total := 200_000

	unique, dups := generateMany(t, g, int(workers), total)

	if unique != total {
		t.Fatalf("expected %d unique IDs, got %d (dups=%d)", total, unique, dups)
	}
}

// Multiple generators with different machine IDs generating concurrently.
func TestUniqueness_MultiNode(t *testing.T) {
	nodes := []uint16{0, 1, 2, 3, 511, 1023} // several distinct machine IDs
	gens := make([]*SnowflakeGenerator, len(nodes))
	for i, nid := range nodes {
		gens[i] = NewSnowflakeGenerator(nid)
	}

	cs := newConcurrentSet(1 << 12) // 4096 shards
	var dupCount int64

	wg := sync.WaitGroup{}
	wg.Add(len(gens))

	per := 40_000
	for _, g := range gens {
		go func(gen *SnowflakeGenerator) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				id, _ := gen.Generate()
				if !cs.add(uint64(id)) {
					atomic.AddInt64(&dupCount, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	want := per * len(gens)
	got := cs.len()
	if got != want {
		t.Fatalf("expected %d unique IDs across nodes, got %d (dups=%d)", want, got, dupCount)
	}
}

// Try to pack exactly 4096 calls within one millisecond window, repeated R rounds.
// Each round aligns to the next millisecond boundary to tighten scheduling.
func TestBurst_Upto4096PerMillisecond(t *testing.T) {
	g := NewSnowflakeGenerator(7)

	const perMS = 4096
	const rounds = 50

	for r := 0; r < rounds; r++ {
		_ = waitUntilNextMillisecond()
		unique, dups := generateBurst(t, g, perMS)
		if unique != perMS {
			t.Fatalf("round %d: expected %d unique IDs in burst, got %d (dups=%d)",
				r, perMS, unique, dups)
		}
	}
}

// Extended soak test under time budget; cancels if it takes too long.
func TestSoak_HighThroughput(t *testing.T) {
	g := NewSnowflakeGenerator(42)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cs := newConcurrentSet(1 << 12)
	var dupCount int64

	workers := max(2, int64(runtime.NumCPU()*4))
	wg := sync.WaitGroup{}
	wg.Add(int(workers))

	var total int64

	for w := 0; w < int(workers); w++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					id, _ := g.Generate()
					atomic.AddInt64(&total, 1)
					if !cs.add(uint64(id)) {
						atomic.AddInt64(&dupCount, 1)
					}
				}
			}
		}()
	}

	wg.Wait()

	got := cs.len()
	want := int(atomic.LoadInt64(&total))
	if got != want {
		t.Fatalf("soak: expected %d unique IDs, got %d (dups=%d)", want, got, dupCount)
	}
}

// Optional: monotonicity under sequential generation (single goroutine).
// If your ID layout implies monotonic growth for sequential calls,
// this checks for strictly increasing values in a short window.
func TestSequential_StrictlyIncreasing(t *testing.T) {
	g := NewSnowflakeGenerator(99)
	const n = 50_000

	prev, _ := g.Generate()
	for i := 1; i < n; i++ {
		cur, _ := g.Generate()
		if cur <= prev {
			t.Fatalf("ID not strictly increasing at i=%d: prev=%d cur=%d", i, prev, cur)
		}
		prev = cur
	}
}

// --- benchmarks --------------------------------------------------------------

func BenchmarkGenerate_Sequential(b *testing.B) {
	g := NewSnowflakeGenerator(1)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = g.Generate()
	}
}

func BenchmarkGenerate_Parallel(b *testing.B) {
	g := NewSnowflakeGenerator(1)
	b.ReportAllocs()
	b.SetParallelism(int(max(2, int64(runtime.NumCPU()*4))))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = g.Generate()
		}
	})
}
