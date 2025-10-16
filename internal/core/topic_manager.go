package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/AadityaChoubey68/clevr-live/internal/buffer"
	"github.com/AadityaChoubey68/clevr-live/internal/throttle"
)

type TopicManager struct {
	bufferManager *buffer.AddaptiveBufferManager
	throttler     *throttle.AdaptiveThrottler

	topics map[string]*Topic
	mu     sync.RWMutex

	shutDownChan chan struct{}
	shutDownOnce sync.Once
}

func NewTopicManager(buffer *buffer.AddaptiveBufferManager, throttle *throttle.AdaptiveThrottler) *TopicManager {
	tm := &TopicManager{
		bufferManager: buffer,
		throttler:     throttle,

		topics:       make(map[string]*Topic),
		shutDownChan: make(chan struct{}),
	}

	go tm.monitoLoop()

	return tm
}

func (tm *TopicManager) makeTopicKey(tenantID, topicName string) string {
	return fmt.Sprintf("%s:%s", tenantID, topicName)
}

func (tm *TopicManager) getOrCreateTopic(tenant_id, topic_name string) (*Topic, error) {

	topicKey := tm.makeTopicKey(tenant_id, topic_name)

	tm.mu.RLock()
	topic, exists := tm.topics[topicKey]
	tm.mu.RUnlock()

	if exists {
		return topic, nil
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	topic, exists = tm.topics[topicKey]
	if exists {
		return topic, nil
	}

	topic = NewTopic(topic_name, tenant_id, 100)

	tm.topics[topicKey] = topic

	fmt.Printf("Created new topic: %s\n", topicKey)

	return topic, nil
}

func (tm *TopicManager) Publish(tenant_id, topic_name string, msg Message) error {
	topic, err := tm.getOrCreateTopic(tenant_id, topic_name)
	if err != nil {
		return err
	}

	return topic.Publish(msg)
}

func (tm *TopicManager) Subscribe(tenant_id, topic_name, subscriberID string, sub *Subscriber) error {
	if sub.TenantID != tenant_id {
		return fmt.Errorf("tenant mismatch")
	}

	topic, err := tm.getOrCreateTopic(tenant_id, topic_name)
	if err != nil {
		return err
	}

	tm.bufferManager.AddNewSubscriber()

	return topic.Subscribe(sub)
}

func (tm *TopicManager) Unsubscribe(tenant_id, topic_name, subscriberID string) error {
	topicKey := tm.makeTopicKey(tenant_id, topic_name)

	tm.mu.RLock()
	topic, exists := tm.topics[topicKey]
	tm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("topic not found: %s", topicKey)
	}

	err := topic.Unsubscribe(subscriberID)
	if err != nil {
		return err
	}

	tm.bufferManager.OnSubscriberRemoval()

	return nil
}

func (tm *TopicManager) GetTopic(tenant_id, topic_name string) (*Topic, error) {
	topicKey := tm.makeTopicKey(tenant_id, topic_name)

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	topic, exists := tm.topics[topicKey]
	if !exists {
		return nil, fmt.Errorf("topic not found: %s", topicKey)
	}

	return topic, nil
}

func (tm *TopicManager) GetAllTopics() ([]*Topic, error) {
	topics := make([]*Topic, 0, len(tm.topics))
	for _, top := range tm.topics {
		topics = append(topics, top)
	}

	return topics, nil
}

func (tm *TopicManager) GetTopicCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.topics)
}

func (tm *TopicManager) GetTotalSubscriberCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	total := 0
	for _, topic := range tm.topics {
		total += topic.GetSubscriberCount()
	}

	return total
}

func (tm *TopicManager) GetSlowSubscriberCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	total := 0
	for _, topic := range tm.topics {
		total += topic.GetSlowSubscriberCount()
	}

	return total
}

func (tm *TopicManager) monitoLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			TotalSubCount := tm.GetTotalSubscriberCount()
			SlowSubCount := tm.GetSlowSubscriberCount()

			tm.throttler.UpdateSubscriber(SlowSubCount, TotalSubCount)

		case <-tm.shutDownChan:
			return
		}

	}
}

func (tm *TopicManager) GetMetrics() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	topicMetrics := make([]map[string]interface{}, 0, len(tm.topics))
	for _, topic := range tm.topics {
		topicMetrics = append(topicMetrics, topic.GetMetrics())
	}

	return map[string]interface{}{
		"total_topics":      len(tm.topics),
		"total_subscribers": tm.GetTotalSubscriberCount(),
		"slow_subscribers":  tm.GetSlowSubscriberCount(),
		"topics":            topicMetrics,
		"throttler_metrics": tm.throttler.GetMetrics(),
	}
}

func (tm *TopicManager) ShutDown() {
	tm.shutDownOnce.Do(func() {
		close(tm.shutDownChan)

		tm.mu.Lock()
		for _, topic := range tm.topics {
			subscribers := topic.getSubscribersSnapshot()
			for _, subs := range subscribers {
				subs.Close()
			}
		}
		tm.mu.Unlock()

		fmt.Println("TopicManager shut down gracefully")
	})
}
