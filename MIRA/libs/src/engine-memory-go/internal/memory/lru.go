package memory

import (
	"container/list"
	"sync"
	"time"
)

type LRUCache struct {
	capacity int
	items    map[uint32]*list.Element
	order    *list.List
	mu       sync.Mutex
	hits     uint64
	total    uint64
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		items:    make(map[uint32]*list.Element),
		order:    list.New(),
	}
}

func (c *LRUCache) Get(key uint32) ([]Synapse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.total++
	if elem, ok := c.items[key]; ok {
		item := elem.Value.(*CacheItem)
		item.LastUsed = time.Now()
		item.AccessCnt++
		c.order.MoveToFront(elem)
		c.hits++
		return item.Synapses, true
	}
	return nil, false
}

func (c *LRUCache) Put(key uint32, synapses []Synapse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		item := elem.Value.(*CacheItem)
		item.Synapses = synapses
		item.Size = len(synapses) * 8
		item.LastUsed = time.Now()
		item.AccessCnt++
		c.order.MoveToFront(elem)
		return
	}

	item := &CacheItem{
		Key:       key,
		Synapses:  synapses,
		Size:      len(synapses) * 8,
		LastUsed:  time.Now(),
		AccessCnt: 1,
	}
	elem := c.order.PushFront(item)
	c.items[key] = elem

	c.evict()
}

func (c *LRUCache) evict() {
	for c.usageUnsafe() > c.capacity && c.order.Len() > 0 {
		elem := c.order.Back()
		item := elem.Value.(*CacheItem)
		delete(c.items, item.Key)
		c.order.Remove(elem)
	}
}

func (c *LRUCache) usageUnsafe() int {
	var total int
	for _, elem := range c.items {
		total += elem.Value.(*CacheItem).Size
	}
	return total
}

func (c *LRUCache) Usage() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usageUnsafe()
}

func (c *LRUCache) HitRatio() float64 {
	if c.total == 0 {
		return 0.0
	}
	return float64(c.hits) / float64(c.total)
}

func (c *LRUCache) Evict(bytesNeeded int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	freed := 0
	for freed < bytesNeeded && c.order.Len() > 0 {
		elem := c.order.Back()
		item := elem.Value.(*CacheItem)

		if item.AccessCnt > 100 && time.Since(item.LastUsed) < time.Minute {
			c.order.MoveToFront(elem)
			continue
		}

		freed += item.Size
		delete(c.items, item.Key)
		c.order.Remove(elem)
	}
}
