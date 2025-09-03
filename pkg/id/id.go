package id

import (
	"fmt"
	"sync"
	"time"
)

// Custom epoch: Aug 30 2025 00:00:00 UTC
var SnowflakeGeneratorCustomEpoch = time.Date(2025, 8, 30, 0, 0, 0, 0, time.UTC)

const (
	timestampBits = 41
	machineBits   = 10
	sequenceBits  = 12

	machineShift   = sequenceBits               // 12
	timestampShift = machineBits + sequenceBits // 22

	sequenceMask = (1 << sequenceBits) - 1 // 0xFFF
	machineMask  = (1 << machineBits) - 1  // 0x3FF
	timestampMax = (1 << timestampBits) - 1
)

type SnowflakeGenerator struct {
	lastTimestamp uint64 // ms since custom epoch
	machineId     uint16 // 10 bits
	sequenceId    uint16 // 12 bits
	lock          sync.Mutex
}

func NewSnowflakeGenerator(machineId uint16) *SnowflakeGenerator {
	now := time.Now().UnixMilli() - SnowflakeGeneratorCustomEpoch.UnixMilli()
	return &SnowflakeGenerator{
		lastTimestamp: uint64(max(0, now)),
		machineId:     machineId & machineMask,
		sequenceId:    0,
	}
}

func (g *SnowflakeGenerator) nowMs() int64 {
	return time.Now().UnixMilli() - SnowflakeGeneratorCustomEpoch.UnixMilli()
}

// waitUntilAfter waits until the generator time is strictly greater than 'after'.
// Importantly, this happens WITHOUT holding the mutex.
func (g *SnowflakeGenerator) waitUntilAfter(after int64) int64 {
	for {
		ts := g.nowMs()
		if ts > after {
			return ts
		}
		// Yield a touch to avoid hot spinning; this keeps latency tiny.
		time.Sleep(time.Microsecond)
	}
}

func (g *SnowflakeGenerator) Generate() (int64, error) {
	for {
		ts := g.nowMs()

		g.lock.Lock()
		last := int64(g.lastTimestamp)

		// If clock went backwards, release lock and wait outside the critical section.
		if ts < last {
			g.lock.Unlock()
			g.waitUntilAfter(last)
			continue
		}

		// Same millisecond: increment sequence; rollover -> wait for next ms (outside lock).
		if ts == last {
			nextSeq := (g.sequenceId + 1) & sequenceMask
			if nextSeq == 0 {
				g.lock.Unlock()
				g.waitUntilAfter(last)
				continue
			}
			g.sequenceId = nextSeq
		} else {
			// New millisecond: reset sequence and advance timestamp.
			g.sequenceId = 0
			g.lastTimestamp = uint64(ts)
		}

		// Sanity (useful in tests; remove if you want max speed)
		if ts < 0 || ts > timestampMax {
			g.lock.Unlock()
			return 0, fmt.Errorf("timestamp %d out of 41-bit range", ts)
		}

		// Assemble ID (uint64 to keep sign bit clean), then cast to int64.
		id := (uint64(ts)&timestampMax)<<timestampShift |
			(uint64(g.machineId)&machineMask)<<machineShift |
			(uint64(g.sequenceId) & sequenceMask)

		g.lock.Unlock()
		return int64(id), nil
	}
}

// small helper to avoid importing math
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
