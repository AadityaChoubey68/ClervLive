package handlers

import (
	"fmt"
	"net/http"

	"github.com/AadityaChoubey68/clevr-live/internal/buffer"
	"github.com/AadityaChoubey68/clevr-live/internal/core"
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type SubscriberHandler struct {
	topicManager  *core.TopicManager
	bufferManager *buffer.AddaptiveBufferManager
}

func NewSubscribeHandler(tm *core.TopicManager, bufferMgr *buffer.AddaptiveBufferManager) *SubscriberHandler {
	return &SubscriberHandler{
		topicManager:  tm,
		bufferManager: bufferMgr,
	}
}

func (h *SubscriberHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant_id, ok := r.Context().Value("tenantId").(string)
	if !ok || tenant_id == "" {
		tenant_id = "default-tenant"
	}

	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "Topic parameter is required", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}

	subscriberID := uuid.New().String()

	bufferSize := h.bufferManager.GetBufferSize()

	ctx := r.Context()

	subscriber := core.NewSubscriber(subscriberID, tenant_id, topic, conn, ctx, bufferSize)

	if err := h.topicManager.Subscribe(tenant_id, topic, subscriberID, subscriber); err != nil {
		conn.Close(websocket.StatusInternalError, fmt.Sprintf("Failed to subscribe: %v", err))
		return
	}

	<-subscriber.Context().Done()

	h.topicManager.Unsubscribe(tenant_id, topic, subscriberID)

	fmt.Printf("Subscriber %s disconnected from %s:%s\n", subscriberID, tenant_id, topic)
}
