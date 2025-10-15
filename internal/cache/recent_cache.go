package cache

import (
	"sync"

	"github.com/AadityaChoubey68/clevr-live/internal/core"
)

type RecentMessageCache struct {
	messages []core.Message
	size     int
	index    int
	count    int
	mu       sync.RWMutex
}

func NewRecentMessageCache(size int) *RecentMessageCache {
	return &RecentMessageCache{
		messages: make([]core.Message, size),
		size:     size,
		index:    0,
		count:    0,
	}
}

func (c *RecentMessageCache) Add(msg core.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages[c.index] = msg

	c.index = (c.index + 1) % c.size

	if c.count < c.size {
		c.count++
	}
}

func (c *RecentMessageCache) GetLast(n int) []core.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if n > c.count {
		n = c.count
	}

	if n == 0 {
		return []core.Message{}
	}

	result := make([]core.Message, n)

	startPost := (c.index - n + c.size) % c.size

	for i := 0; i < n; i++ {
		pos := (startPost + i) % c.size
		result[i] = c.messages[pos]
	}
	return result
}

func (c *RecentMessageCache) GetAll() []core.Message {
	return c.GetLast(c.count)
}

func (c *RecentMessageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.index = 0
	c.count = 0
}

func (c *RecentMessageCache) GetCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}
