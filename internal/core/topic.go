package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Topic struct {
	name     string
	tenantID string

	subscribers map[string]*Subscriber
	subMutex    sync.RWMutex

	recentCache *RecentMessageCache

	messagesPublished atomic.Int64
	totalSubscribers  atomic.Int64
	createdAt         time.Time
}

func NewTopic(name, tenantID string, cahcheSize int) *Topic {
	return &Topic{
		name:        name,
		tenantID:    tenantID,
		subscribers: make(map[string]*Subscriber),
		recentCache: NewRecentMessageCache(cahcheSize),
		createdAt:   time.Now(),
	}
}

func (t *Topic) getSubscribersSnapshot() []*Subscriber {
	t.subMutex.RLock()
	defer t.subMutex.RUnlock()

	snapshot := make([]*Subscriber, 0, len(t.subscribers))

	for _, sub := range t.subscribers {
		snapshot = append(snapshot, sub)
	}
	return snapshot
}

func (t *Topic) Publish(msg Message) error {
	t.messagesPublished.Add(1)

	t.recentCache.Add(msg)

	subscribers := t.getSubscribersSnapshot()

	if len(subscribers) == 0 {
		return nil
	}

	var wg sync.WaitGroup

	for _, sub := range subscribers {
		wg.Add(1)
		go func(s *Subscriber) {
			defer wg.Done()
			if err := s.SendMessages(msg); err != nil {
				fmt.Printf("Failed to send to subscriber %s: %v\n", s.ID, err)
			}
		}(sub)
	}

	wg.Wait()
	return nil
}

func (t *Topic) sendRecentMessages(sub *Subscriber) {
	recent := t.recentCache.GetLast(50)

	for _, msg := range recent {
		if err := sub.SendMessages(msg); err != nil {
			fmt.Printf("Failed to send recent message to %s: %v\n", sub.ID, err)
			return
		}
	}
}

func (t *Topic) Subscribe(sub *Subscriber) error {

	if t.tenantID != sub.TenantID {
		return fmt.Errorf("tenant mismatch: subscriber %s belongs to %s, topic belongs to %s",
			sub.ID, sub.TenantID, t.tenantID)
	}

	t.subMutex.Lock()
	t.subscribers[sub.ID] = sub
	t.subMutex.Unlock()

	t.totalSubscribers.Add(1)

	go t.sendRecentMessages(sub)

	sub.Start()

	fmt.Printf("Subscriber %s joined topic %s:%s (total: %d)\n",
		sub.ID, t.tenantID, t.name, len(t.subscribers))

	return nil
}

func (t *Topic) Unsubscribe(subscriberID string) error {
	t.subMutex.Lock()
	defer t.subMutex.Unlock()

	sub, exists := t.subscribers[subscriberID]
	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}

	delete(t.subscribers, subscriberID)

	sub.Close()

	fmt.Printf("Subscriber %s left topic %s:%s (remaining: %d)\n",
		subscriberID, t.tenantID, t.name, len(t.subscribers))

	return nil
}

func (t *Topic) GetSubscriberCount() int {
	t.subMutex.RLock()
	defer t.subMutex.RUnlock()
	return len(t.subscribers)
}

func (t *Topic) GetSlowSubscriberCount() int {
	t.subMutex.RLock()
	defer t.subMutex.RUnlock()

	count := 0
	for _, sub := range t.subscribers {
		if sub.IsSlow() {
			count++
		}
	}

	return count
}

func (t *Topic) GetTenantID() string {
	return t.tenantID
}

func (t *Topic) GetName() string {
	return t.name
}

func (t *Topic) GetMetrics() map[string]interface{} {
	t.subMutex.RLock()
	subCount := len(t.subscribers)
	t.subMutex.RUnlock()

	return map[string]interface{}{
		"name":               t.name,
		"tenant_id":          t.tenantID,
		"messages_published": t.messagesPublished.Load(),
		"active_subscribers": subCount,
		"total_subscribers":  t.totalSubscribers.Load(),
		"created_at":         t.createdAt,
	}
}
