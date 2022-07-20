package flowcontrol

import (
	"github.com/tsundata/flowline/pkg/util/clock"
	"math/rand"
	"sync"
	"time"
)

type backoffEntry struct {
	backoff    time.Duration
	lastUpdate time.Time
}

type Backoff struct {
	sync.RWMutex
	Clock           clock.Clock
	defaultDuration time.Duration
	maxDuration     time.Duration
	perItemBackoff  map[string]*backoffEntry
	rand            *rand.Rand

	// maxJitterFactor adds jitter to the exponentially backed off delay.
	// if maxJitterFactor is zero, no jitter is added to the delay in
	// order to maintain current behavior.
	maxJitterFactor float64
}

func NewBackOff(initial, max time.Duration) *Backoff {
	return NewBackOffWithJitter(initial, max, 0.0)
}

func NewBackOffWithJitter(initial, max time.Duration, maxJitterFactor float64) *Backoff {
	ck := clock.RealClock{}
	return newBackoff(ck, initial, max, maxJitterFactor)
}

func newBackoff(clock clock.Clock, initial, max time.Duration, maxJitterFactor float64) *Backoff {
	var random *rand.Rand
	if maxJitterFactor > 0 {
		random = rand.New(rand.NewSource(clock.Now().UnixNano()))
	}
	return &Backoff{
		perItemBackoff:  map[string]*backoffEntry{},
		Clock:           clock,
		defaultDuration: initial,
		maxDuration:     max,
		maxJitterFactor: maxJitterFactor,
		rand:            random,
	}
}

// Get the current backoff Duration
func (p *Backoff) Get(id string) time.Duration {
	p.RLock()
	defer p.RUnlock()
	var delay time.Duration
	entry, ok := p.perItemBackoff[id]
	if ok {
		delay = entry.backoff
	}
	return delay
}

// Next move backoff to the next mark, capping at maxDuration
func (p *Backoff) Next(id string, eventTime time.Time) {
	p.Lock()
	defer p.Unlock()
	entry, ok := p.perItemBackoff[id]
	if !ok || hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		entry = p.initEntryUnsafe(id)
		entry.backoff += p.jitter(entry.backoff)
	} else {
		delay := entry.backoff * 2       // exponential
		delay += p.jitter(entry.backoff) // add some jitter to the delay
		entry.backoff = time.Duration(Int64Min(int64(delay), int64(p.maxDuration)))
	}
	entry.lastUpdate = p.Clock.Now()
}

// Reset forces clearing of all backoff data for a given key.
func (p *Backoff) Reset(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.perItemBackoff, id)
}

// IsInBackOffSince Returns True if the elapsed time since eventTime is smaller than the current backoff window
func (p *Backoff) IsInBackOffSince(id string, eventTime time.Time) bool {
	p.RLock()
	defer p.RUnlock()
	entry, ok := p.perItemBackoff[id]
	if !ok {
		return false
	}
	if hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		return false
	}
	return p.Clock.Since(eventTime) < entry.backoff
}

// IsInBackOffSinceUpdate Returns True if time since lastupdate is less than the current backoff window.
func (p *Backoff) IsInBackOffSinceUpdate(id string, eventTime time.Time) bool {
	p.RLock()
	defer p.RUnlock()
	entry, ok := p.perItemBackoff[id]
	if !ok {
		return false
	}
	if hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		return false
	}
	return eventTime.Sub(entry.lastUpdate) < entry.backoff
}

// GC Garbage collect records that have aged past maxDuration. Backoff users are expected
// to invoke this periodically.
func (p *Backoff) GC() {
	p.Lock()
	defer p.Unlock()
	now := p.Clock.Now()
	for id, entry := range p.perItemBackoff {
		if now.Sub(entry.lastUpdate) > p.maxDuration*2 {
			// GC when entry has not been updated for 2*maxDuration
			delete(p.perItemBackoff, id)
		}
	}
}

func (p *Backoff) DeleteEntry(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.perItemBackoff, id)
}

// Take a lock on *Backoff, before calling initEntryUnsafe
func (p *Backoff) initEntryUnsafe(id string) *backoffEntry {
	entry := &backoffEntry{backoff: p.defaultDuration}
	p.perItemBackoff[id] = entry
	return entry
}

func (p *Backoff) jitter(delay time.Duration) time.Duration {
	if p.rand == nil {
		return 0
	}

	return time.Duration(p.rand.Float64() * p.maxJitterFactor * float64(delay))
}

// After 2*maxDuration we restart the backoff factor to the beginning
func hasExpired(eventTime time.Time, lastUpdate time.Time, maxDuration time.Duration) bool {
	return eventTime.Sub(lastUpdate) > maxDuration*2 // consider stable if it's ok for twice the maxDuration
}

// Int64Min returns the minimum of the params
func Int64Min(a, b int64) int64 {
	if b < a {
		return b
	}
	return a
}
