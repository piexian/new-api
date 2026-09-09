package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store == nil {
		l.store = make(map[string]*[]int64)
		l.expirationDuration = expirationDuration
		if expirationDuration > 0 {
			go l.clearExpiredItems()
		}
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key := range l.store {
			queue := l.store[key]
			size := len(*queue)
			if size == 0 || now-(*queue)[size-1] > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	allowed, _ := l.RequestWithRetry(key, maxRequestNum, duration)
	return allowed
}

func (l *InMemoryRateLimiter) RequestWithRetry(key string, maxRequestNum int, duration int64) (bool, int64) {
	return l.requestWithRetry(key, maxRequestNum, duration, true)
}

// CheckWithRetry checks completed-request limits without counting failed attempts.
func (l *InMemoryRateLimiter) CheckWithRetry(key string, maxRequestNum int, duration int64) (bool, int64) {
	return l.requestWithRetry(key, maxRequestNum, duration, false)
}

// RecordCompletion keeps the newest completions even when requests admitted
// earlier finish after the success limit has already been reached.
func (l *InMemoryRateLimiter) RecordCompletion(key string, maxRequestNum int) {
	if maxRequestNum <= 0 {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	queue := l.store[key]
	if queue == nil {
		queue = &[]int64{}
		l.store[key] = queue
	}
	*queue = append(*queue, time.Now().Unix())
	if len(*queue) > maxRequestNum {
		*queue = (*queue)[len(*queue)-maxRequestNum:]
	}
}

func (l *InMemoryRateLimiter) requestWithRetry(key string, maxRequestNum int, duration int64, record bool) (bool, int64) {
	if maxRequestNum <= 0 {
		return true, 0
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue := l.store[key]
	now := time.Now().Unix()
	if queue == nil {
		queue = &[]int64{}
		l.store[key] = queue
	}
	for len(*queue) > 0 && now-(*queue)[0] >= duration {
		*queue = (*queue)[1:]
	}
	if len(*queue) >= maxRequestNum {
		// Also handles a configured limit being lowered while requests are live.
		return false, (*queue)[len(*queue)-maxRequestNum] + duration - now
	}
	if record {
		*queue = append(*queue, now)
	}
	return true, 0
}
