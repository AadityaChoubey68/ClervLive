package core

import "time"

type Message struct {
	Id        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	TenantID  string                 `json:"tenant_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

func NewMessage(topic, tenantID string, data map[string]interface{}) Message {
	return Message{
		Id:        GenerateId(),
		Topic:     topic,
		TenantID:  tenantID,
		Data:      data,
		Timestamp: time.Now(),
	}
}

func GenerateId() string {
	return "msg-" + time.Now().Format("20060102-150405.000000")
}
