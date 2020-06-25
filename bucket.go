package replay

import (
	"sync"
	"time"
)

type bucket struct {
	mtx  sync.RWMutex
	data map[[32]byte]time.Time
}
