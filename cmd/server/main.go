package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AadityaChoubey68/clevr-live/internal/buffer"
	"github.com/AadityaChoubey68/clevr-live/internal/config"
	"github.com/AadityaChoubey68/clevr-live/internal/core"
	"github.com/AadityaChoubey68/clevr-live/internal/handlers"
	"github.com/AadityaChoubey68/clevr-live/internal/throttle"
)

func main() {
	config := config.LoadConfig()
	log.Printf("Configuration loaded: MaxMemory=%dMB", config.MaxMemory/(1024*1024))

	bufferManager := buffer.NewAdaptiveBufferManager(config.MaxMemory)
	bufferManager.Start()
	log.Println("Buffer manager started")

	throttlerConfig := throttle.DefaultConfig()
	adaptiveThrottler := throttle.NewAdaptiveThrottler(throttlerConfig)
	log.Println("Adaptive throttler initialized")

	topicManager := core.NewTopicManager(bufferManager, adaptiveThrottler)
	log.Println("Topic manager started")

	publishHandler := handlers.NewPublishHandler(topicManager, adaptiveThrottler)
	subscribeHandler := handlers.NewSubscribeHandler(topicManager, bufferManager)
	healthHandler := handlers.NewHealthHandler(topicManager)

	mux := http.NewServeMux()

	mux.HandleFunc("/publish", publishHandler.ServeHTTP)

	mux.HandleFunc("/subscribe", subscribeHandler.ServeHTTP)

	mux.HandleFunc("/health", healthHandler.ServeHTTP)

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		metrics := topicManager.GetMetrics()

		fmt.Fprintf(w, "%v", metrics)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "ClevrLive event Streaming system\n\n")
		fmt.Fprintf(w, "Endpoints:\n")
		fmt.Fprintf(w, "  POST /publish          - Publish a message\n")
		fmt.Fprintf(w, "  WS   /subscribe?topic= - Subscribe to a topic\n")
		fmt.Fprintf(w, "  GET  /health           - Health check\n")
		fmt.Fprintf(w, "  GET  /metrics          - System metrics\n")
	})

	server := &http.Server{
		Addr:         config.Address,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on %s", config.Address)
		log.Println("Ready to accept connections!")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	topicManager.ShutDown()

	bufferManager.Stop()

	log.Println("Server stopped gracefully")
}
