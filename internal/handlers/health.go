package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/AadityaChoubey68/clevr-live/internal/core"
)

type HealthResponse struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Uptime      string                 `json:"uptime"`
	Topics      int                    `json:"topics"`
	Subscribers int                    `json:"subscribers"`
	Goroutines  int                    `json:"goroutines"`
	Memory      map[string]interface{} `json:"memory"`
}

type HealthHandler struct {
	topicManager *core.TopicManager
	startTime    time.Time
}

func NewHealthHandler(tm *core.TopicManager) *HealthHandler {
	return &HealthHandler{
		topicManager: tm,
		startTime:    time.Now(),
	}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	response := HealthResponse{
		Status:      "healthy",
		Timestamp:   time.Now(),
		Uptime:      time.Since(h.startTime).String(),
		Topics:      h.topicManager.GetTopicCount(),
		Subscribers: h.topicManager.GetTotalSubscriberCount(),
		Goroutines:  runtime.NumGoroutine(),
		Memory: map[string]interface{}{
			"alloc_mb":       float64(m.Alloc) / 1024 / 1024,
			"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
			"sys_mb":         float64(m.Sys) / 1024 / 1024,
			"num_gc":         m.NumGC,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
