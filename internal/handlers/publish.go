package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/AadityaChoubey68/clevr-live/internal/core"
	"github.com/AadityaChoubey68/clevr-live/internal/throttle"
)

type Publishrequest struct {
	Topic string                 `json:"topic"`
	Data  map[string]interface{} `json:"data"`
}

type PublishResponse struct {
	Success   bool   `json:"success"`
	MessageId string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

type PublishHandler struct {
	topicManager *core.TopicManager
	throttler    *throttle.AdaptiveThrottler
}

func NewPublishHandler(tm *core.TopicManager, throttler *throttle.AdaptiveThrottler) *PublishHandler {
	return &PublishHandler{
		topicManager: tm,
		throttler:    throttler,
	}
}

func (h *PublishHandler) respondError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(PublishResponse{
		Success: false,
		Error:   message,
	})
}
func (h *PublishHandler) respondSuccess(w http.ResponseWriter, messageID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(PublishResponse{
		Success:   true,
		MessageId: messageID,
	})
}

func (h *PublishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	tenant_id, ok := r.Context().Value("tenantId").(string)

	if !ok || tenant_id == "" {
		tenant_id = "default-tenant"
	}

	var req Publishrequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, "Invalid request Body", http.StatusBadRequest)
		return
	}

	if req.Topic == "" {
		h.respondError(w, "Topic Needed", http.StatusBadRequest)
		return
	}

	if req.Data == nil {
		h.respondError(w, "DAta Needed", http.StatusBadRequest)
		return
	}

	if h.throttler.ShouldThrottle() {
		h.throttler.ApplyThrottle()
	}

	msg := core.NewMessage(req.Topic, tenant_id, req.Data)

	if err := h.topicManager.Publish(tenant_id, req.Topic, msg); err != nil {
		h.respondError(w, fmt.Sprintf("Failed to publish: %v", err), http.StatusInternalServerError)
		return
	}

	h.respondSuccess(w, msg.Id)
}
