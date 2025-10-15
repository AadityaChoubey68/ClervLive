package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type DropStrategy int

const (
	DROP_OLDEST DropStrategy = iota
	DROP_NEWEST
	CIRCUIT_BREAKER
)

type Subscriber struct {
	ID               string
	TenantID         string
	Topic            string
	messageChan      chan Message
	conn             *websocket.Conn
	ctx              context.Context
	cancel           context.CancelFunc
	dropStrategy     DropStrategy
	droppedCount     atomic.Int64
	messagesRecieved atomic.Int64
	messagesSent     atomic.Int64
	lastActive       time.Time
	done             chan struct{}
	closeOnce        sync.Once
}

func NewSubscriber(id, tenantID, topic string, conn *websocket.Conn, ctx context.Context, bufferSize int) *Subscriber {
	ctx, cancel := context.WithCancel(ctx)

	return &Subscriber{
		ID:           id,
		TenantID:     tenantID,
		Topic:        topic,
		messageChan:  make(chan Message, bufferSize),
		conn:         conn,
		ctx:          ctx,
		cancel:       cancel,
		dropStrategy: DROP_OLDEST,
		lastActive:   time.Now(),
		done:         make(chan struct{}),
	}
}

func (s *Subscriber) Start() {
	go s.sendLoop()
}

func (s *Subscriber) SendMessages(msg Message) error {
	s.messagesRecieved.Add(1)

	select {
	case s.messageChan <- msg:
		return nil
	default:
		return s.handleBackPressure(msg)
	}
}

func (s *Subscriber) handleBackPressure(msg Message) error {
	switch s.dropStrategy {
	case DROP_OLDEST:
		select {
		case <-s.messageChan:
			s.droppedCount.Add(1)
		default:
		}

		select {
		case s.messageChan <- msg:
			return nil
		default:
			s.droppedCount.Add(1)
			return fmt.Errorf("buffer Still Full After dropping data for subscriber : %s", s.ID)
		}

	case DROP_NEWEST:
		s.droppedCount.Add(1)
		return fmt.Errorf("subscriber %s: buffer full, dropped new message", s.ID)

	case CIRCUIT_BREAKER:
		dropped := s.droppedCount.Load()
		if dropped > 100 {
			s.Close()
			return fmt.Errorf("subscriber %s: circuit breaker triggered", s.ID)
		}
		return nil

	default:
		s.droppedCount.Add(1)
		return fmt.Errorf("subscriber %s: unknown drop strategy", s.ID)
	}
}

func (s *Subscriber) sendLoop() {
	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case msg, ok := <-s.messageChan:
			if !ok {
				return
			}

			if err := s.sendToClient(msg); err != nil {
				s.Close()
				return
			}
			s.messagesSent.Add(1)
			s.lastActive = time.Now()
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			err := s.conn.Ping(ctx)
			cancel()

			if err != nil {
				s.Close()
				return
			}
		case <-s.done:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Subscriber) sendToClient(msg Message) error {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	return wsjson.Write(ctx, s.conn, msg)
}

func (s *Subscriber) Close() {
	s.closeOnce.Do(func() {
		s.cancel()

		close(s.done)

		s.conn.Close(websocket.StatusNormalClosure, "Subscriber Disconnected")

		close(s.messageChan)
	})
}

func (s *Subscriber) GetMetrics() map[string]int64 {
	return map[string]int64{
		"Messages Recieved": s.messagesRecieved.Load(),
		"Messages Sent":     s.messagesSent.Load(),
		"Messages Dropped":  s.droppedCount.Load(),
	}
}

func (s *Subscriber) IsHealthy() bool {
	if time.Since(s.lastActive) > 60*time.Second {
		return false
	}

	messagesRecieved := s.messagesRecieved.Load()
	droppedCount := s.droppedCount.Load()

	if messagesRecieved > 0 {
		dropRate := float64(droppedCount) / float64(messagesRecieved)
		if dropRate > 0.1 {
			return false
		}
	}
	return true
}

func (s *Subscriber) IsSlow() bool {
	return s.droppedCount.Load() > 0
}
