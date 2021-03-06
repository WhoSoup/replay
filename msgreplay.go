package replay

import (
	"sync"
	"time"
)

const (
	MsgValid           = iota * -1 // +0 |
	TimestampExpired               // -1 |
	TimestampTooFuture             // -2 | We reject things too far in the future
	ReplayMsg                      // -3 | Msg already seen

	// If the msg is added to the filter
	MsgAdded = 1
)

type MsgReplay struct {
	bucketsMtx sync.RWMutex // protects buckets and oldest
	buckets    []*bucket
	oldest     time.Time

	// helper indices for readability
	current int
	future  int

	blocktime time.Duration // Minimum block time

}

// MsgReplay is divided into blocks. The window is the number of valid blocks.
//
// If the window is 6, then there will be 8 buckets. The first 6 are the window of valid
// blocks, anything before the 0th bucket timestamp is expired. The 6th index is the current block.
// The window corresponds to the number of blocks before the current. The 7th index is all future messages
// that fall outside the current block.
func NewMsgReplay(window int, blocktime time.Duration) *MsgReplay {
	m := new(MsgReplay)
	m.buckets = make([]*bucket, window+2, window+2)
	for i := range m.buckets {
		m.buckets[i] = newBucket()
	}
	m.current = window
	m.future = window + 1
	m.blocktime = blocktime

	return m
}

// Recenter sets the new center for the window to be valid around
func (m *MsgReplay) Recenter(stamp time.Time) {
	m.bucketsMtx.Lock()
	defer m.bucketsMtx.Unlock()

	if stamp.Before(m.oldest) {
		return // We can't go backwards in time
	}

	// move everything up by one, drop first bucket
	copy(m.buckets, m.buckets[1:])
	m.buckets[m.future] = newBucket()

	m.buckets[m.current].SetTime(stamp)
	m.oldest = m.buckets[0].Time()

	// re-arrange bucket with knowledge of exact timestamp
	m.buckets[m.current-1].Transfer(m.buckets[m.current])
}

func (m *MsgReplay) CheckReplay(hash [32]byte, ts time.Time, update bool) int {
	m.bucketsMtx.RLock()
	defer m.bucketsMtx.RUnlock()
	var index int = -1 // Index of the bucket to check against

	if ts.Before(m.buckets[0].time) {
		return TimestampExpired
	}

	// First see if the this msg is from the past
	for i := 1; i < m.future; i++ {
		if ts.Before(m.buckets[i].time) {
			// Place the msg into the correct bucket
			index = i - 1 // Bucket[i-1] is the right bucket
			break
		}
	}

	if index == -1 {
		// Msg is from the future or the current
		if ts.Before(m.buckets[m.current].time.Add(m.blocktime)) {
			// Current
			index = m.current
		} else {
			// Future
			// TODO: Handle future messages
			index = m.future
			// return TimestampTooFuture
		}
	}

	if index != -1 {
		_, ok := m.buckets[index].Get(hash)
		if ok {
			return ReplayMsg
		}
		if update {
			m.buckets[index].Set(hash, ts)
			return MsgAdded // Added
		}
	}

	return MsgValid // Found, but not updated
}
