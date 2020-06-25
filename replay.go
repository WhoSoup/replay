package replay

import (
	"sort"
	"sync"
	"time"

	"github.com/FactomProject/factomd/common/interfaces"
)

type Replay struct {
	data       map[[32]byte]bool
	added      []*Entry
	timestamps []time.Time
	windows    int
	mtx        sync.RWMutex
}

type Entry struct {
	key  [32]byte
	time time.Time
}

func New(oldest time.Time, windows int) *Replay {
	r := new(Replay)
	r.data = make(map[[32]byte]bool)
	r.timestamps = make([]time.Time, 0, windows+1)
	r.timestamps = append(r.timestamps, oldest)
	return r
}

func (r *Replay) Update(msg interfaces.IMsg) bool {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	rh := msg.GetRepeatHash().Fixed()

	if _, ok := r.data[rh]; ok {
		return false
	}

	r.data[rh] = true
	r.added = append(r.added, &Entry{key: rh, time: msg.GetTimestamp().GetTime()})
	return true
}

func (r *Replay) Has(msg interfaces.IMsg) bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if msg.GetTimestamp().GetTime().Before(r.timestamps[0]) {
		return false // todo: TooOld
	}

	_, ok := r.data[msg.GetRepeatHash().Fixed()]
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
