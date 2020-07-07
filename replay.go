package replay

import (
	"sort"
	"sync"
	"time"
)

type Replay struct {
	data       map[[32]byte]bool
	added      []*Entry
	timestamps []time.Time
	windows    int
	mtx        sync.RWMutex
	blocktime  time.Duration
}

type Entry struct {
	key  [32]byte
	time time.Time
}

func New(blocktime time.Duration, windows int) *Replay {
	r := new(Replay)
	r.data = make(map[[32]byte]bool)
	r.timestamps = make([]time.Time, 0, windows+1)
	//r.timestamps = append(r.timestamps, oldest)
	r.blocktime = blocktime
	return r
}

func (r *Replay) Update(hash [32]byte, ts time.Time) bool {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if _, ok := r.data[hash]; ok {
		return false
	}

	r.data[hash] = true
	r.added = append(r.added, &Entry{key: hash, time: ts})
	return true
}

func (r *Replay) Has(hash [32]byte, ts time.Time) bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if ts.Before(r.timestamps[0]) {
		return false // todo: TooOld
	}

	_, ok := r.data[hash]
	return ok
}

func (r *Replay) Recenter(ts time.Time) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.timestamps = append(r.timestamps, ts)
	if len(r.timestamps) > r.windows {
		r.timestamps = r.timestamps[1:]

		sort.Slice(r.added, func(i, j int) bool {
			return r.added[i].time.Before(r.added[j].time)
		})

		for k := range r.added {
			if r.added[k].time.Before(r.timestamps[0]) {
				delete(r.data, r.added[k].key)
			} else {
				r.added = r.added[k:]
				break
			}
		}

	}
}
